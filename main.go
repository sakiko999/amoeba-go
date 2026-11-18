// mygo-host —— Electron 宿主的 mygo 重写原型（阶段 1：外壳 + 组件管理器）。
// 结构：native UI 窗口（Content）+ 顶部 44px chrome webview（tab 条，React）
// + 每个 tab 一个 WebView；页面包由 registry 管理，经 page:// 协议服务。
// service 组件暂不实现（按决策为宿主内置 Go 包，后续阶段落地）。
//
// 运行：bun run dev（scripts/build.ts 统一编排 chrome/页面组件构建后 go run .）
package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/fetch"
	"github.com/egoist/mygo/ui"

	"mygo-host/components/jsrunner"
	"mygo-host/components/player"
	"mygo-host/components/profile"
	"mygo-host/components/search"
	"mygo-host/platform"
	"mygo-host/registry"
	"mygo-host/services"
	"mygo-host/tabs"
)

//go:embed all:web/chrome/dist
var chromeDist embed.FS

func main() {
	pagesDirs := flag.String("pages", strings.Join(discoverPages(), ";"), "内置页面组件根目录(每子目录一个组件包;多个根用 ; 分隔;默认自动发现 components/*/out)")
	vendorDir := flag.String("vendor", filepath.Join("web", "page-vendor", "out", "dist"), "vendor:// 共享层目录")
	flag.Parse()

	userData, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		log.Fatal(err)
	}
	theme := newThemeStore(filepath.Join(userData, "host-state.json"))
	// 亮暗源对齐宿主主题（须在 App.Run 前）：页面 prefers-color-scheme/mygo 原生菜单
	// 跟宿主而不是系统，滚动条等 UA 控件不会停在系统一侧
	tabs.ApplyThemeSource(theme.Theme())

	// 页面可调用的 Go 侧 fetch(无 CORS、任意请求头):player-h5 的 hls.js/dash.js
	// 请求出口(hostFetch.ts)与之后页面抓取都靠它。所有 trusted 页面(含外部安装的
	// 页面组件)都可调用 —— 页面本就可载任意 JS,不构成额外权限升级。
	mygo.Use(fetch.Plugin)

	comps := registry.NewManager(strings.Split(*pagesDirs, ";"), filepath.Join(userData, "pages"))
	comps.Logf = log.Printf
	comps.Scan()

	// 内置 Go 服务:平台运行时 + api-gateway(自检日志与 TS 宿主 smoke 同面)
	cache, err := platform.NewCache(filepath.Join(userData, "provider-cache"))
	if err != nil {
		log.Fatal(err)
	}
	js := jsrunner.New()
	jsSvc, err := jsrunner.NewService(js)
	if err != nil {
		log.Fatal(err)
	}
	sc := &platform.ServiceContext{
		Logf:  log.Printf,
		HTTP:  platform.NewHostClient(),
		Cache: cache,
		Creds: platform.NewCredentials(filepath.Join(userData, "credentials.json")),
		JS:    js,
	}
	mgr := tabs.NewManager(comps, theme)
	mgr.Logf = log.Printf
	// player/profile 是需要宿主依赖(打开 tab)的服务组件:构造时注入 Opener,
	// 拓扑激活时经网关取调用面;search 同为网关消费者(拓扑激活注入)
	playerSvc, playerComp, err := player.NewService(mgr)
	if err != nil {
		log.Fatal(err)
	}
	searchSvc, searchComp, err := search.NewService()
	if err != nil {
		log.Fatal(err)
	}
	profileSvc, profileComp, err := profile.NewService(mgr)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := services.Activate(sc, comps, jsSvc, playerComp, searchComp, profileComp); err != nil {
		log.Fatal(err)
	}

	// 自定义 scheme 必须在创建窗口之前注册（mygo 约定：Run 之前完成）
	if err := mygo.Protocol.Handle("page", comps.PageHandler()); err != nil {
		log.Fatal(err)
	}
	if err := mygo.Protocol.Handle("vendor", registry.VendorHandler(*vendorDir)); err != nil {
		log.Fatal(err)
	}

	mygo.BindAs("Tabs", tabs.NewService(mgr))
	mygo.BindAs("Search", searchSvc)
	mygo.BindAs("Player", playerSvc)
	mygo.BindAs("Profile", profileSvc)

	web, err := fs.Sub(chromeDist, "web/chrome/dist")
	if err != nil {
		log.Fatal(err)
	}
	mygo.SetFrontend(web)

	mygo.App.WhenReady(func() {
		mgr.MeasureChrome()
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:           "Tabs",
			Width:           1100,
			Height:          720,
			MinWidth:        640,
			MinHeight:       400,
			TitleBarStyle:   mygo.TitleBarHidden,
			TitleBarHeight:  int(mgr.ChromeHeight),
			BackgroundColor: theme.Background(),
			Content:         ui.View(mgr.View),
			// 隐藏创建，chrome 首帧就绪后 Show：避免标题栏从空白到 tab 条的闪烁
			Hidden: true,
		})
		mgr.Attach(win)
		js.Init()
		if err := mgr.Start(); err != nil {
			log.Fatal(err)
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// discoverPages 自动发现已构建的内置页面组件:components/*/out 下存在 page/component.json
// 即视为一个根。与 scripts/build.ts 的发现规则同源 —— 新增页面组件零登记。
func discoverPages() []string {
	matches, err := filepath.Glob(filepath.Join("components", "*", "out"))
	if err != nil {
		return nil
	}
	var roots []string
	for _, m := range matches {
		if _, err := os.Stat(filepath.Join(m, "page", "component.json")); err == nil {
			roots = append(roots, m)
		}
	}
	return roots
}
