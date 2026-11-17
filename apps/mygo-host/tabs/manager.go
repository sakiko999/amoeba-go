// Package tabs 管理 tab 生命周期（每个 tab = 一个 mygo.WebView）。
// 与 Electron 版（apps/desktop/src/main/tabs.ts）的映射：
//   - 视图树只构建 active tab 的 webview；其余 tab 的 webview 不构建即隐藏，
//     页面保留（mygo 语义），切回瞬时——对应 Electron 的 setVisible 保活
//   - 状态唯一权威源在这里；每次变更 Invalidate 原生视图并广播 tabs.changed（chrome 消费）
//   - 会话按组件隔离：persist:page-<id> ⇔ NewSession("page-"+id)
package tabs

import (
	"fmt"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mygo-host/registry"
)

// ChromeHeight 与 chrome 页面（tab 条）和 WindowOptions.TitleBarHeight 保持一致。
const ChromeHeight float32 = 44

// Summary 是广播给 chrome 的 tab 投影（字段与 Electron 版 TabSummary 对齐）。
type Summary struct {
	InstanceID string `json:"instanceId"`
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	Active     bool   `json:"active"`
	Loading    bool   `json:"loading"`
}

// Kind 是“新建 tab”菜单的数据（对应 Electron 版 TabManifest）。
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

type entry struct {
	instanceID string
	kind       string
	title      string
	view       *mygo.WebView
	loading    bool
	err        string
}

// ThemeSource 是 Manager 对主题存储的最小依赖（main 包实现落盘）。
type ThemeSource interface {
	Theme() string
	Background() string
	Foreground() string
	Toggle() string
}

type Manager struct {
	comps   *registry.Manager
	theme   ThemeSource
	win     *mygo.Window
	chrome  *mygo.WebView
	bar     ui.TitleBar
	entries map[string]*entry
	order   []string
	active  string
	seq     int
	// zoom 抵消 WebView2 把 Windows 文字缩放（TextScale）当作页面 zoom 的默认行为，
	// 让 webview 里 CSS px 与窗口 DIP 一一对应（与 Electron 版观感一致）
	zoom float64
	// Logf 由宿主注入（缺省丢弃）
	Logf func(format string, args ...any)
}

func NewManager(comps *registry.Manager, theme ThemeSource) *Manager {
	return &Manager{
		comps:   comps,
		theme:   theme,
		entries: make(map[string]*entry),
		zoom:    1,
		Logf:    func(string, ...any) {},
	}
}

// Attach 绑定宿主窗口并装好视图；随后调用 Start 创建 chrome。
func (m *Manager) Attach(win *mygo.Window) { m.win = win }

// Start 创建 chrome webview（mygo 前端），应用窗口主题并触发首帧。
func (m *Manager) Start() error {
	if ts := mygo.Theme.Preferences().TextScale; ts > 0 {
		m.zoom = 1 / ts
	}
	chrome, err := m.win.NewWebView(mygo.WebViewOptions{
		URL:             "/",
		BackgroundColor: m.theme.Background(),
		Page:            mygo.PageOptions{ZoomFactor: m.zoom},
	})
	if err != nil {
		return err
	}
	m.chrome = chrome
	chrome.Page().OnDidFinishLoad(func() { m.logf("chrome 已加载") })
	chrome.Page().OnDidFailLoad(func(err *mygo.LoadError) {
		m.logf("chrome 加载失败: %s (%d)", err.Description, err.Code)
	})
	m.applyTheme()
	m.win.Invalidate()
	m.emitChanged()
	return nil
}

// Create 打开一个页面组件的新 tab：独立 session、独立 webview，创建即激活。
func (m *Manager) Create(kind string) error {
	comp := m.comps.Get(kind)
	if comp == nil {
		return fmt.Errorf("页面组件 %q 未安装或未就绪", kind)
	}
	m.seq++
	instanceID := fmt.Sprintf("%s-%d", kind, m.seq)
	view, err := m.win.NewWebView(mygo.WebViewOptions{
		URL:             comp.Manifest.EntryURL(),
		BackgroundColor: m.theme.Background(),
		Page: mygo.PageOptions{
			Session:       mygo.NewSession("page-" + kind),
			PreloadScript: pageBridgeScript(instanceID, kind, m.theme.Theme()),
			ZoomFactor:    m.zoom,
		},
	})
	if err != nil {
		return err
	}
	e := &entry{instanceID: instanceID, kind: kind, title: comp.Manifest.Name, view: view, loading: true}
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
	return nil
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
func (m *Manager) Move(instanceID, overInstanceID string) {
	from := indexOf(m.order, instanceID)
	to := indexOf(m.order, overInstanceID)
	if from < 0 || to < 0 || from == to {
		return
	}
	m.order = removeString(m.order, instanceID)
	if from < to {
		to--
	}
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
	m.theme.Toggle()
	m.applyTheme()
	HostStateChanged.Emit(m.win, m.ThemeInfo())
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
	for _, e := range m.entries {
		_ = e.view.SetBackgroundColor(bg)
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
