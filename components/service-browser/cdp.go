// 载具的 CDP 发现与子进程辅助:随机端口来自 WebView2 写在
// profile/EBWebView/DevToolsActivePort 的第一行;HTTP 探测只走 127.0.0.1。
// WEBVIEW2_USER_DATA_FOLDER 指向的目录即持久化根(cookie/localStorage/
// IndexedDB/Cache/Service Worker),WebView2 实际数据在其 EBWebView 子目录。

package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var cdpHTTP = &http.Client{Timeout: 5 * time.Second}

func httpBase(port int) string { return "http://127.0.0.1:" + strconv.Itoa(port) }

func getJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := cdpHTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("GET %s: HTTP %d", url, res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// versionOK 是存活探测:与调用方 ctx 解耦,调用方超时/取消不能导致
// ensureLocked 误杀其他 origin 正在使用的载具进程。
func versionOK(ctx context.Context, port int) bool {
	probeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 1500*time.Millisecond)
	defer cancel()
	var version struct {
		Browser string `json:"Browser"`
	}
	return getJSON(probeCtx, httpBase(port)+"/json/version", &version) == nil
}

// waitBrowserWS 等 /json/version 给出 browser 级 WebSocket 端点。
func waitBrowserWS(ctx context.Context, port int) (string, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		var version struct {
			WS string `json:"webSocketDebuggerUrl"`
		}
		if err := getJSON(ctx, httpBase(port)+"/json/version", &version); err == nil && version.WS != "" {
			return version.WS, nil
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("等待 browser CDP 端点超时(127.0.0.1:%d)", port)
		case <-ticker.C:
		}
	}
}

func readActivePort(path string) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	line, _, _ := strings.Cut(strings.TrimSpace(string(raw)), "\n")
	if strings.TrimSpace(line) == "" {
		return 0, fmt.Errorf("DevToolsActivePort 为空")
	}
	return strconv.Atoi(strings.TrimSpace(line))
}

// carrierBrowserArgs 是 WebView2 环境的额外启动参数(移植自 tauri-rss 的
// amoeba-browser):关掉首启向导/后台网络/组件更新等噪声,限制 JS 堆与磁盘
// 缓存;CalculateNativeWinOcclusion 防止屏外窗口被当作遮挡而停摆。
// mygo 的 ThrottlingOff 已设置三个 --disable-background-* 开关,这里不重复。
const carrierBrowserArgs = "--remote-debugging-port=0" +
	" --no-first-run --no-default-browser-check --hide-crash-restore-bubble" +
	" --disable-sync --disable-extensions --disable-default-apps" +
	" --disable-background-networking --disable-component-update --no-pings" +
	" --disable-breakpad --metrics-recording-only --disable-gpu-compositing" +
	" --js-flags=--max-old-space-size=512 --disk-cache-size=67108864" +
	" --disable-features=msWebOOUI,msPdfOOUI,msSmartScreenProtection,CalculateNativeWinOcclusion,Translate,BackForwardCache,OptimizationHints,MediaRouter"

// carrierEnv 覆盖 WebView2 的两个环境变量,其余环境原样继承(测试注入的
// 环境变量也保留)。
func carrierEnv(base []string, profile string) []string {
	if base == nil {
		base = os.Environ()
	}
	env := make([]string, 0, len(base)+2)
	for _, kv := range base {
		if strings.HasPrefix(kv, "WEBVIEW2_USER_DATA_FOLDER=") ||
			strings.HasPrefix(kv, "WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS=") {
			continue
		}
		env = append(env, kv)
	}
	return append(env,
		"WEBVIEW2_USER_DATA_FOLDER="+profile,
		"WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS="+carrierBrowserArgs,
	)
}

type logWriter struct {
	logf func(string, ...any)
}

func (w logWriter) Write(p []byte) (int, error) {
	if w.logf != nil {
		for line := range strings.SplitSeq(strings.TrimRight(string(p), "\r\n"), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				w.logf("browser-carrier: %s", line)
			}
		}
	}
	return len(p), nil
}
