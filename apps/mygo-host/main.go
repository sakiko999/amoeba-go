// mygo-host —— Electron 宿主的 mygo 重写原型（阶段 1：外壳 + 组件管理器）。
// 结构：native UI 窗口（Content）+ 顶部 44px chrome webview（tab 条，React）
// + 每个 tab 一个 WebView；页面包由 registry 管理，经 page:// 协议服务。
// service 组件暂不实现（按决策为宿主内置 Go 包，后续阶段落地）。
//
// 运行：先构建 chrome（bun run --cwd apps/mygo-host/chrome build），再 go run .
package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mygo-host/components/jsrunner"
	"mygo-host/platform"
	"mygo-host/registry"
	"mygo-host/search"
	"mygo-host/services"
	"mygo-host/tabs"
)

//go:embed all:chrome/dist
var chromeDist embed.FS

func main() {
	pagesDir := flag.String("pages", filepath.Join("..", "desktop", "out", "pages"), "内置页面组件目录")
	vendorDir := flag.String("vendor", filepath.Join("..", "desktop", "out", "vendor"), "vendor:// 共享层目录")
	flag.Parse()

	userData, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		log.Fatal(err)
	}
	theme := newThemeStore(filepath.Join(userData, "host-state.json"))

	comps := registry.NewManager(*pagesDir, filepath.Join(userData, "pages"))
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
	gw, err := services.Activate(sc, comps, jsSvc)
	if err != nil {
		log.Fatal(err)
	}

	// 自定义 scheme 必须在创建窗口之前注册（mygo 约定：Run 之前完成）
	if err := mygo.Protocol.Handle("page", comps.PageHandler()); err != nil {
		log.Fatal(err)
	}
	if err := mygo.Protocol.Handle("vendor", registry.VendorHandler(*vendorDir)); err != nil {
		log.Fatal(err)
	}

	mgr := tabs.NewManager(comps, theme)
	mgr.Logf = log.Printf
	mygo.BindAs("Tabs", tabs.NewService(mgr))
	mygo.BindAs("Search", search.NewService(gw))

	web, err := fs.Sub(chromeDist, "chrome/dist")
	if err != nil {
		log.Fatal(err)
	}
	mygo.SetFrontend(web)

	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:           "Tabs",
			Width:           1100,
			Height:          720,
			MinWidth:        640,
			MinHeight:       400,
			TitleBarStyle:   mygo.TitleBarHidden,
			TitleBarHeight:  int(tabs.ChromeHeight),
			BackgroundColor: theme.Background(),
			Content:         ui.View(mgr.View),
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
