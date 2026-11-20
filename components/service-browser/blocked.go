// 抓取页的资源拦截:图片/字体/媒体/统计域名对 SSR/DOM 抓取零价值,却实打实
// 占带宽与内存。每个 target attach 后默认启用;捕获任务只复用这层 Network
// 会话,不再随任务关闭。不拦 CSS(布局与懒加载判定依赖它)。扫码登录不依赖
// 图片解码:抖音/小红书读 data URL 或 canvas,微博/豆瓣只读 img.src 后由宿主
// HTTP 取码。媒体与图片尾缀带 * 以覆盖 query;无扩展名 CDN 只拦明确的图片
// 路径,不能整域拦同时承载 JS/CSS 的静态域名。

package browser

import (
	"context"
)

var blockedURLPatterns = []string{
	"*.avif*",
	"*.gif*",
	"*.jpeg*",
	"*.jpg*",
	"*.png*",
	"*.webp*",
	"*.woff*",
	"*.ttf*",
	"*.otf*",
	"*/fonts/*",
	"*.mp4*",
	"*.m3u8*",
	"*.m4s*",
	"*.webm*",
	"*.flv*",
	"*.mov*",
	"*.mp3*",
	"*.m4a*",
	"*.aac*",
	"*.ts",
	"*.ts?*",
	"*douyinvod.com*",
	"*douyinpic.com*",
	"*douyinstatic.com/*image*",
	"*sns-video*",
	"*sns-img*",
	"*video.weibocdn.com*",
	"*hdslb.com/*image*",
	"*doubleclick.net*",
	"*google-analytics.com*",
	"*googletagmanager.com*",
	"*hm.baidu.com*",
	"*cnzz.com*",
	"*umeng.com*",
	"*sentry.io*",
}

// armDefaultNetwork 是 target 的默认 Network 配置。失败时创建 target 失败:
// 默认拦截是载具的资源策略,不降级为可选优化。用 setBlockedURLs 而不是
// Fetch 域拦截:阻断发生在网络栈,renderer 仍会得到失败事件,但没有逐请求
// 往返 CDP 的额外开销。
func armDefaultNetwork(ctx context.Context, conn *cdpConn, sessionID string) error {
	if err := conn.call(ctx, "Network.enable", nil, sessionID, nil); err != nil {
		return err
	}
	return conn.call(ctx, "Network.setBlockedURLs",
		map[string]any{"urls": blockedURLPatterns}, sessionID, nil)
}
