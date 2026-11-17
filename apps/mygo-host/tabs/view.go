// 视图树与内容区状态页：active tab 的 webview / 加载页 / 空态 / 错误页。
// 内容区由 native UI 绘制而不是 DOM，所以加载与失败态在页面 webview 首次
// did-finish-load 之前就能显示，页面组件不必自带骨架屏或错误页。
// 状态页底色/文字跟随主题，主题切换时由 Manager.applyTheme 触发 Invalidate 重绘。

package tabs

import (
	"github.com/egoist/mygo/plugins/loading"
	"github.com/egoist/mygo/ui"
)

// View 是 native UI 视图树：顶部 44px chrome + 内容区（active tab / 加载页 / 空态）。
// 在 chrome 创建前也要能安全渲染（首帧可能早于 Start）。
func (m *Manager) View(c *ui.Context) {
	m.bar = c.TitleBar()
	ui.Column(c).Fill().Background(ui.Hex(m.theme.Background())).Children(func() {
		if m.chrome != nil {
			ui.WebView(c, m.chrome).Height(ChromeHeight)
		}
		e := m.activeEntry()
		switch {
		case e == nil:
			m.emptyView(c)
		case e.loading:
			m.loadingView(c, e)
		case e.err != "":
			m.errorView(c, e)
		default:
			ui.WebView(c, e.view).Grow(1)
		}
	})
}

// 页面首次 did-finish-load 前不显示其 webview：避免白屏闪一下，先给加载页
func (m *Manager) loadingView(c *ui.Context, e *entry) {
	ui.Column(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Gap(12).Children(func() {
		// loading.dev 同款指示器（Go 绘制）；ReduceMotion 自动停帧、辅助技术读作进度条。
		// 颜色必须显式跟随宿主主题：Ring 缺省用 mygo 默认主题（跟随系统亮暗），
		// 系统暗 + 宿主亮时画白色弧，在浅色背景上不可见。
		loading.Ring(c, loading.Options{Color: ui.Hex(m.theme.Foreground())}).
			Label("正在加载 " + e.title)
		ui.Text(c, "正在加载 "+e.title+"…").TextColor(ui.Hex(m.theme.Foreground()))
	})
}

func (m *Manager) emptyView(c *ui.Context) {
	ui.Column(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Gap(12).Children(func() {
		ui.Text(c, "没有打开的页面").FontSize(15).TextColor(ui.Hex(m.theme.Foreground()))
		ui.Button(c, "新建页面").OnClick(m.PopupNewTabMenu)
	})
}

func (m *Manager) errorView(c *ui.Context, e *entry) {
	ui.Column(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Gap(8).Children(func() {
		ui.Text(c, "页面加载失败").FontSize(15).Bold().TextColor(ui.Hex(m.theme.Foreground()))
		ui.Text(c, e.err).TextColor(ui.Hex(m.theme.Foreground()))
		ui.Button(c, "重新加载").OnClick(func() {
			e.loading = true
			e.err = ""
			m.win.Invalidate()
			e.view.Page().Reload()
		})
	})
}

func (m *Manager) activeEntry() *entry {
	if m.active == "" {
		return nil
	}
	return m.entries[m.active]
}
