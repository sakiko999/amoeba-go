// Package tabs 管理 tab 生命周期（每个 tab = 一个 mygo.WebView）。
//   - 视图树只构建 active tab 的 webview；其余 tab 的 webview 不构建即隐藏，
//     页面保留（mygo 语义），切回瞬时可见
//   - 状态唯一权威源在这里；每次变更 Invalidate 原生视图并广播 tabs.changed（chrome 消费）
//   - 会话按组件隔离：persist:page-<id> ⇔ NewSession("page-"+id)
package tabs

import (
	"fmt"
	"math"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mygo-host/components/service-player"
	"mygo-host/components/service-profile"
	"mygo-host/registry"
)

// chromeBaseHeight 是 100% 文本缩放下 tab 条（chrome webview）的设计高度；
// 实际容器高度 = 基准 × 系统文本缩放：WebView2 会把页面内容按 TextScale
// 放大，native 容器必须同步放大，否则 tab 条内容溢出被裁（MeasureChrome）。
const chromeBaseHeight float32 = 44

// Summary 是广播给 chrome 的 tab 投影。
type Summary struct {
	InstanceID string `json:"instanceId"`
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	Active     bool   `json:"active"`
	Loading    bool   `json:"loading"`
}

// Kind 是“新建 tab”菜单的数据。
type Kind struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Color string `json:"color"`
}

// ThemeInfo 是 host.state 事件与 Tabs.State 的负载。
type ThemeInfo struct {
	Theme string `json:"theme"`
}

// ChromeInfo 是 chrome 页面初始化所需的标题栏度量（native UI 的 ui.TitleBar）。
// webview 页面拿不到 --mygo-titlebar-* CSS 变量，由 chrome 主动来取。
type ChromeInfo struct {
	InsetLeft  float32 `json:"insetLeft"`
	InsetRight float32 `json:"insetRight"`
	Height     float32 `json:"height"`
	Bottom     float32 `json:"bottom"`
}

// TabsChanged 在 tab 集合/顺序/激活态/加载态变化时广播到本窗口全部 webview 页面。
var TabsChanged = mygo.NewEvent[[]Summary]("tabs.changed")

// HostStateChanged 在主题变化时广播。
var HostStateChanged = mygo.NewEvent[ThemeInfo]("host.state")

// PlayerH5Play 在复用 player-h5 tab 播放新内容时广播（页面按 instanceId 过滤）。
var PlayerH5Play = mygo.NewEvent[player.H5Target]("player.h5.play")

// ProfileOpen 在复用个人主页 tab 换目标时广播（页面按 instanceId 过滤）。
var ProfileOpen = mygo.NewEvent[profile.ProfileTarget]("profile.open")

type entry struct {
	instanceID string
	kind       string
	title      string
	view       *mygo.WebView
	h5Target   *player.H5Target       // 非 nil = player-h5 页面 tab 的播放目标
	profile    *profile.ProfileTarget // 非 nil = 个人主页 tab 的主页目标
	loading    bool
	err        string
}

// ThemeSource 是 Manager 对主题存储的最小依赖（main 包实现落盘）。
type ThemeSource interface {
	Theme() string
	Background() string
	ContentBackground() string
	Foreground() string
	Toggle() string
}

type Manager struct {
	comps  *registry.Manager
	theme  ThemeSource
	win    *mygo.Window
	chrome *mygo.WebView
	bar    ui.TitleBar
	// ChromeHeight 是 chrome 容器高度（DIP），与窗口 TitleBarHeight 一致；
	// 由 MeasureChrome 在创建窗口前按系统文本缩放计算。
	ChromeHeight float32
	entries      map[string]*entry
	order        []string
	active       string
	seq          int
	// Logf 由宿主注入（缺省丢弃）
	Logf func(format string, args ...any)
}

func NewManager(comps *registry.Manager, theme ThemeSource) *Manager {
	return &Manager{
		comps:        comps,
		theme:        theme,
		ChromeHeight: chromeBaseHeight,
		entries:      make(map[string]*entry),
		Logf:         func(string, ...any) {},
	}
}

// MeasureChrome 按系统文本缩放计算 chrome 容器高度；App 就绪后、创建窗口前调用。
// 运行中改系统文本缩放不跟随（重启生效），与窗口 TitleBarHeight 的静态语义一致。
func (m *Manager) MeasureChrome() {
	if ts := mygo.Theme.Preferences().TextScale; ts > 1 {
		m.ChromeHeight = float32(math.Round(float64(chromeBaseHeight) * ts))
	}
}

// Attach 绑定宿主窗口并装好视图；随后调用 Start 创建 chrome。
func (m *Manager) Attach(win *mygo.Window) { m.win = win }

// Start 创建 chrome webview（mygo 前端），应用窗口主题并触发首帧。
func (m *Manager) Start() error {
	chrome, err := m.win.NewWebView(mygo.WebViewOptions{
		URL:             "/",
		BackgroundColor: m.theme.Background(),
	})
	if err != nil {
		return err
	}
	m.chrome = chrome
	chrome.Page().OnDidFinishLoad(func() {
		m.logf("chrome 已加载")
		m.win.Show() // 隐藏创建的窗口在 chrome 首帧就绪后再显示
	})
	chrome.Page().OnDidFailLoad(func(err *mygo.LoadError) {
		m.logf("chrome 加载失败: %s (%d)", err.Description, err.Code)
		m.win.Show() // 失败也必须显示窗口（错误态可诊断），否则窗口永远不出现
	})
	m.applyTheme()
	m.win.Invalidate()
	m.emitChanged()
	return nil
}

// Create 打开一个页面组件的新 tab：独立 session、独立 webview，创建即激活。
func (m *Manager) Create(kind string) error {
	_, err := m.create(kind, "", nil)
	return err
}

// create 是 Create 与 OpenPlayerH5 的公共创建路径；title 为空时取组件名。
func (m *Manager) create(kind, title string, h5 *player.H5Target) (*entry, error) {
	comp := m.comps.Get(kind)
	if comp == nil {
		return nil, fmt.Errorf("页面组件 %q 未安装或未就绪", kind)
	}
	if title == "" {
		title = comp.Manifest.Name
	}
	m.seq++
	instanceID := fmt.Sprintf("%s-%d", kind, m.seq)
	view, err := m.win.NewWebView(mygo.WebViewOptions{
		URL:             comp.Manifest.EntryURL(),
		BackgroundColor: m.theme.ContentBackground(),
		Page: mygo.PageOptions{
			Session:       mygo.NewSession("page-" + kind),
			PreloadScript: pageBridgeScript(instanceID, kind, m.theme.Theme()),
		},
	})
	if err != nil {
		return nil, err
	}
	e := &entry{instanceID: instanceID, kind: kind, title: title, view: view, h5Target: h5, loading: true}
	view.Page().OnDidFinishLoad(func() {
		e.loading = false
		m.logf("tab %s（%s）已加载", instanceID, kind)
		m.win.Invalidate()
		m.emitChanged()
	})
	view.Page().OnDidFailLoad(func(loadErr *mygo.LoadError) {
		m.logf("tab %s 加载失败: %s (%d)", instanceID, loadErr.Description, loadErr.Code)
		e.loading = false
		e.err = loadErr.Description
		m.win.Invalidate()
		m.emitChanged()
	})
	m.entries[instanceID] = e
	m.order = append(m.order, instanceID)
	m.Activate(instanceID)
	return e, nil
}

// OpenPlayerH5 打开(或复用)player-h5 页面 tab。首次目标随创建写入，页面挂载后
// 经 Player.Target 主动来取；复用同一个 tab 时改目标并广播事件(页面按 instanceId
// 过滤)——连续点播不堆一堆播放 tab，与原生播放器的复用语义一致。
func (m *Manager) OpenPlayerH5(title string, target player.H5Target) error {
	for _, id := range m.order {
		e := m.entries[id]
		if e.kind != "player-h5" {
			continue
		}
		target.InstanceID = id
		e.h5Target = &target
		if title != "" {
			e.title = title
		}
		m.Activate(id)
		PlayerH5Play.Emit(m.win, target)
		return nil
	}
	e, err := m.create("player-h5", title, &target)
	if err != nil {
		return err
	}
	target.InstanceID = e.instanceID
	return nil
}

// Target 取 player-h5 tab 的播放目标（页面挂载时调用；从新建菜单直接打开的没有目标）。
func (m *Manager) Target(instanceID string) (player.H5Target, bool) {
	e, ok := m.entries[instanceID]
	if !ok || e.h5Target == nil {
		return player.H5Target{}, false
	}
	return *e.h5Target, true
}

// OpenProfile 打开(或复用)个人主页页面 tab。复用语义与 player-h5 一致:
// 首次目标随创建写入,页面挂载后经 Profile.Target 主动来取;换目标时广播
// 事件(页面按 instanceId 过滤),不堆一堆主页 tab。
func (m *Manager) OpenProfile(title string, target profile.ProfileTarget) error {
	for _, id := range m.order {
		e := m.entries[id]
		if e.kind != "profile" {
			continue
		}
		target.InstanceID = id
		e.profile = &target
		if title != "" {
			e.title = title
		}
		m.Activate(id)
		ProfileOpen.Emit(m.win, target)
		return nil
	}
	e, err := m.create("profile", title, nil)
	if err != nil {
		return err
	}
	// webview 首次加载完成前页面脚本不会执行,这里补写目标无竞态。
	target.InstanceID = e.instanceID
	e.profile = &target
	return nil
}

// ProfileTarget 取个人主页 tab 的目标（页面挂载时调用；新建菜单直接打开的没有目标）。
func (m *Manager) ProfileTarget(instanceID string) (profile.ProfileTarget, bool) {
	e, ok := m.entries[instanceID]
	if !ok || e.profile == nil {
		return profile.ProfileTarget{}, false
	}
	return *e.profile, true
}

func (m *Manager) Activate(instanceID string) {
	if _, ok := m.entries[instanceID]; !ok {
		return
	}
	m.active = instanceID
	m.win.Invalidate()
	m.emitChanged()
}

func (m *Manager) Close(instanceID string) {
	e, ok := m.entries[instanceID]
	if !ok {
		return
	}
	delete(m.entries, instanceID)
	m.order = removeString(m.order, instanceID)
	if !e.view.IsDestroyed() {
		e.view.Destroy()
	}
	if m.active == instanceID {
		m.active = ""
		if rest := len(m.order); rest > 0 {
			m.active = m.order[rest-1]
		}
	}
	m.win.Invalidate()
	m.emitChanged()
}

func (m *Manager) CloseOthers(instanceID string) {
	if _, ok := m.entries[instanceID]; !ok {
		return
	}
	for _, id := range append([]string(nil), m.order...) {
		if id != instanceID {
			m.Close(id)
		}
	}
}

func (m *Manager) CloseRight(instanceID string) {
	idx := indexOf(m.order, instanceID)
	if idx < 0 {
		return
	}
	for _, id := range append([]string(nil), m.order[idx+1:]...) {
		m.Close(id)
	}
}

// Move 把 instanceID 移到 overInstanceID 当前的位置（对应拖拽排序）。
// 语义与 chrome 侧 dnd-kit 的 arrayMove 一致：移除后按原 to 下标插回——右移落在
// over 之后、左移落在 over 之前；多做 to-- 会导致右移一格被宿主视为不变，
// tabs.changed 回推后与乐观更新不一致（拖到最右端弹回一格）。
func (m *Manager) Move(instanceID, overInstanceID string) {
	from := indexOf(m.order, instanceID)
	to := indexOf(m.order, overInstanceID)
	if from < 0 || to < 0 || from == to {
		return
	}
	m.order = removeString(m.order, instanceID)
	m.order = insertString(m.order, to, instanceID)
	m.emitChanged()
}

func (m *Manager) Kinds() []Kind {
	pages := m.comps.Pages()
	kinds := make([]Kind, 0, len(pages))
	for _, c := range pages {
		color := c.Manifest.Color
		if color == "" {
			color = "#565f89"
		}
		kinds = append(kinds, Kind{ID: c.Manifest.ID, Title: c.Manifest.Name, Color: color})
	}
	return kinds
}

func (m *Manager) ThemeInfo() ThemeInfo { return ThemeInfo{Theme: m.theme.Theme()} }

func (m *Manager) ChromeInfo() ChromeInfo {
	return ChromeInfo{
		InsetLeft:  m.bar.Left,
		InsetRight: m.bar.Right,
		Height:     m.bar.Height,
		Bottom:     m.bar.Bottom,
	}
}

// SetTitle 更新 tab 条标题（页面经 window.tab.setTitle 调用）。
func (m *Manager) SetTitle(instanceID, title string) {
	e, ok := m.entries[instanceID]
	if !ok || e.title == title {
		return
	}
	e.title = title
	m.emitChanged()
}

// Ping 是页面 ↔ 宿主的 IPC 往返演示（sysinfo 页面用）。
func (m *Manager) Ping(instanceID, value string) (string, error) {
	e, ok := m.entries[instanceID]
	if !ok {
		return "", fmt.Errorf("tab %q 不存在", instanceID)
	}
	return fmt.Sprintf("pong:%s @ %s", value, e.kind), nil
}

// ToggleTheme 切换主题：持久化、窗口按钮明暗、全部 webview 底色、广播 host.state。
func (m *Manager) ToggleTheme() {
	next := m.theme.Toggle()
	ApplyThemeSource(next)
	m.applyTheme()
	HostStateChanged.Emit(m.win, m.ThemeInfo())
}

// ApplyThemeSource 把 mygo 的全局亮暗源对齐宿主主题：页面 prefers-color-scheme、
// mygo 原生菜单跟随宿主而不是系统。注意 mygo 限制——运行期 SetSource 只影响之后
// 创建的 webview，已建页面的 UA 控件靠 theme.css 的 color-scheme（跟 .dark class）保证。
func ApplyThemeSource(theme string) {
	if theme == "light" {
		mygo.Theme.SetSource(mygo.ThemeLight)
	} else {
		mygo.Theme.SetSource(mygo.ThemeDark)
	}
}

func (m *Manager) applyTheme() {
	bg := m.theme.Background()
	_ = m.win.SetBackgroundColor(bg)
	if m.theme.Theme() == "dark" {
		m.win.SetWindowButtonsTheme(mygo.ThemeDark)
	} else {
		m.win.SetWindowButtonsTheme(mygo.ThemeLight)
	}
	if m.chrome != nil {
		_ = m.chrome.SetBackgroundColor(bg)
	}
	content := m.theme.ContentBackground()
	for _, e := range m.entries {
		_ = e.view.SetBackgroundColor(content)
	}
	m.win.Invalidate()
}

func (m *Manager) emitChanged() {
	TabsChanged.Emit(m.win, m.Summaries())
}

func (m *Manager) Summaries() []Summary {
	out := make([]Summary, 0, len(m.order))
	for _, id := range m.order {
		e := m.entries[id]
		out = append(out, Summary{
			InstanceID: e.instanceID,
			Kind:       e.kind,
			Title:      e.title,
			Active:     id == m.active,
			Loading:    e.loading,
		})
	}
	return out
}

// Logf 由宿主注入（缺省丢弃）。
func (m *Manager) logf(format string, args ...any) { m.Logf(format, args...) }

func indexOf(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}

func removeString(list []string, s string) []string {
	out := list[:0]
	for _, v := range list {
		if v != s {
			out = append(out, v)
		}
	}
	return out
}

func insertString(list []string, i int, s string) []string {
	list = append(list, "")
	copy(list[i+1:], list[i:])
	list[i] = s
	return list
}
