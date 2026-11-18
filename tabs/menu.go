// 宿主动态菜单：“+”新建页面与 tab 右键菜单。
// chrome webview 只有 44px 高，DOM 浮层会被视口裁掉，所以菜单统一由宿主用
// mygo 原生 Menu 弹出；Popup 即当前光标位置，无需页面上报坐标。

package tabs

import "github.com/egoist/mygo"

// PopupNewTabMenu 在鼠标位置弹出“新建页面”原生菜单。
func (m *Manager) PopupNewTabMenu() {
	kinds := m.Kinds()
	items := make([]*mygo.MenuItem, 0, len(kinds))
	for _, k := range kinds {
		kind := k.ID
		items = append(items, &mygo.MenuItem{Label: k.Title, Click: func(*mygo.MenuItem, *mygo.Window) {
			if err := m.Create(kind); err != nil {
				m.logf("新建 %s 失败: %v", kind, err)
			}
		}})
	}
	if len(items) == 0 {
		items = append(items, &mygo.MenuItem{Label: "没有可用的页面组件", Disabled: true})
	}
	mygo.NewMenu(items).Popup(m.win)
}

// PopupTabMenu 弹出 tab 右键菜单；关闭其他/右侧按当前位置决定可用性。
func (m *Manager) PopupTabMenu(instanceID string) {
	idx := indexOf(m.order, instanceID)
	if idx < 0 {
		return
	}
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: "关闭", Click: func(*mygo.MenuItem, *mygo.Window) { m.Close(instanceID) }},
		{Label: "关闭其他", Disabled: len(m.order) < 2, Click: func(*mygo.MenuItem, *mygo.Window) { m.CloseOthers(instanceID) }},
		{Label: "关闭右侧", Disabled: idx >= len(m.order)-1, Click: func(*mygo.MenuItem, *mygo.Window) { m.CloseRight(instanceID) }},
	})
	menu.Popup(m.win)
}
