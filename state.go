// 主题状态：持久化到 userData/host-state.json（对应 Electron 版 HostStateStore 的
// theme 部分）。窗口/标题栏底色与按钮明暗取 @demo/ui 的 --sidebar；页面内容区
// （loading 视图与页面 webview 预绘制底色）取 --background，两者不可混用。
package main

import (
	"encoding/json"
	"os"
)

type themeStore struct {
	path  string
	theme string
}

var backgrounds = map[string]string{
	"dark":  "#171717",
	"light": "#fafafa",
}

// contentBackgrounds 与 @demo/ui 的 --background 对齐：页面 body 就是它，
// 内容区（loading/空态视图、页面 webview 预绘制）必须同色，否则每次开页整块
// 内容区会从 --sidebar 色调跳到 --background（暗色变深、亮色变白）形成闪烁。
var contentBackgrounds = map[string]string{
	"dark":  "#0a0a0a",
	"light": "#ffffff",
}

// foregrounds 用于 native UI 的空态/加载页文字（与 @demo/ui 的 --foreground 对齐）。
var foregrounds = map[string]string{
	"dark":  "#c0caf5",
	"light": "#3b3f5c",
}

func newThemeStore(path string) *themeStore {
	s := &themeStore{path: path, theme: "dark"}
	if data, err := os.ReadFile(path); err == nil {
		var saved struct {
			Theme string `json:"theme"`
		}
		if json.Unmarshal(data, &saved) == nil && saved.Theme != "" {
			s.theme = saved.Theme
		}
	}
	return s
}

func (s *themeStore) Theme() string { return s.theme }

func (s *themeStore) Background() string {
	if bg, ok := backgrounds[s.theme]; ok {
		return bg
	}
	return backgrounds["dark"]
}

func (s *themeStore) ContentBackground() string {
	if bg, ok := contentBackgrounds[s.theme]; ok {
		return bg
	}
	return contentBackgrounds["dark"]
}

func (s *themeStore) Foreground() string {
	if fg, ok := foregrounds[s.theme]; ok {
		return fg
	}
	return foregrounds["dark"]
}

// Toggle 翻转并落盘；返回新主题。
func (s *themeStore) Toggle() string {
	if s.theme == "dark" {
		s.theme = "light"
	} else {
		s.theme = "dark"
	}
	data, _ := json.Marshal(map[string]string{"theme": s.theme})
	_ = os.WriteFile(s.path, data, 0o644)
	return s.theme
}
