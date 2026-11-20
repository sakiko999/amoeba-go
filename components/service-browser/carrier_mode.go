// 载具子进程入口:同一二进制的 --browser-carrier 分支。只创建一个隐藏的
// about:blank 窗口把 WebView2 环境带起来,CDP 端口由父进程设置的环境变量
// 开启;stdin EOF(父进程 Stop 或退出)即 App.Quit。基础页 Untrusted,
// 没有 window.mygo 桥;CDP 新建的 target 也不经过 mygo 注入。

package browser

import (
	"bufio"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/egoist/mygo"
)

// RunCarrierIfRequested 处理 --browser-carrier;返回 true 时宿主 main 应直接结束。
func RunCarrierIfRequested() bool {
	if slices.Contains(os.Args[1:], carrierFlag) {
		RunCarrier()
		return true
	}
	return false
}

// RunCarrier 阻塞运行载具,直到 stdin EOF 触发 App.Quit。
func RunCarrier() {
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:       "browser-carrier",
			URL:         "about:blank",
			Width:       320,
			Height:      240,
			Hidden:      true,
			SkipTaskbar: true,
			Page: mygo.PageOptions{
				BackgroundThrottling: mygo.ThrottlingOff,
				Untrusted:            true,
			},
		})
		startTidy(func(format string, args ...any) {
			fmt.Fprintf(os.Stderr, format+"\n", args...)
		})
		go func() {
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				if strings.TrimSpace(scanner.Text()) == "tidy" {
					kickTidy()
				}
			}
			mygo.App.Quit()
		}()
	})
	if err := mygo.App.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "browser-carrier:", err)
		os.Exit(1)
	}
}
