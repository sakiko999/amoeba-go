// 载具环境变量:必须剔除继承的 WEBVIEW2_* 再注入自己的值(外部全局调试端口
// 会抢走本实例的 CDP),启动参数须包含随机调试端口与内存上限。

package browser

import (
	"strings"
	"testing"
)

func TestCarrierEnv(t *testing.T) {
	env := carrierEnv([]string{
		"PATH=C:\\Windows",
		"WEBVIEW2_USER_DATA_FOLDER=C:\\elsewhere",
		"WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS=--remote-debugging-port=9222",
	}, `C:\profile`)

	var got string
	seen := map[string]int{}
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		seen[name]++
		if name == "WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS" {
			got = kv
		}
	}
	if seen["WEBVIEW2_USER_DATA_FOLDER"] != 1 || seen["WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS"] != 1 {
		t.Fatalf("WEBVIEW2_* 应各一条, got %v", seen)
	}
	if !strings.HasSuffix(env[len(env)-2], `WEBVIEW2_USER_DATA_FOLDER=C:\profile`) {
		t.Fatalf("profile 变量 = %q", env[len(env)-2])
	}
	for _, want := range []string{"--remote-debugging-port=0", "--js-flags=--max-old-space-size=512", "--disk-cache-size=67108864"} {
		if !strings.Contains(got, want) {
			t.Fatalf("启动参数缺少 %q: %s", want, got)
		}
	}
	if strings.Contains(got, "9222") {
		t.Fatalf("继承的调试端口未剔除: %s", got)
	}
}
