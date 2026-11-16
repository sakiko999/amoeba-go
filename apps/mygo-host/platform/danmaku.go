// 弹幕统一契约(对齐 platform-kit/runtime/danmaku/types):
// 各平台弹幕归一成 DanmakuItem,player 的 DanmakuLayer 消费。
// color 统一 #RRGGBB(douyu 走专属 6 色映射表)。

package platform

// DanmakuItem 单条弹幕。
type DanmakuItem struct {
	Text string `json:"text"`
	// User 发送者昵称(直播聊天必填;视频弹幕通常无)。
	User *string `json:"user,omitempty"`
	// Color #RRGGBB。
	Color *string `json:"color,omitempty"`
	// TimeMs VOD 绝对位置毫秒(live 实时追加可省;0 是合法值)。
	TimeMs *int64 `json:"timeMs,omitempty"`
	// Mode B站视频弹幕 mode:1/2/3 滚动,4 底,5 顶,6 逆向,7 高级。
	Mode *int `json:"mode,omitempty"`
}

// DanmakuStream 弹幕流:订阅即开始(onItems 收批次),返回值是取消函数。
// 对齐 TS DanmakuStream = (onItems) => unsubscribe。
type DanmakuStream func(onItems func(items []DanmakuItem)) (cancel func())

// DanmakuInfo 弹幕请求选项。
type DanmakuInfo struct {
	// Cookie 登录 cookie(部分平台握手需带)。
	Cookie string
	// Kind vod(带 timeMs 时间轴)或 live(实时聊天);空 = 平台缺省。
	Kind string
}
