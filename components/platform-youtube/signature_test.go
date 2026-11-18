// n 参数解密链离线测试:base.js URL 提取、函数名/函数体提取与 fixup、JS 执行面
// 的 code/fn/args 形状(等价 TS new Function(code; return fn(...args))(...args)),
// 以及 deobfuscateNParam 全链路(本地 HTTP server + fake JSRunner)。

package youtube

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mygo-host/platform"
)

func TestExtractPlayerJsURL(t *testing.T) {
	html := `<script>var ytcfg={"jsUrl":"/s/player/abc123/player_ias.vflset/en_US/base.js"};</script>`
	got := extractPlayerJsURL(html)
	want := "https://www.youtube.com/s/player/abc123/player_ias.vflset/en_US/base.js"
	if got != want {
		t.Errorf("extractPlayerJsURL = %q, want %q", got, want)
	}
	if got := extractPlayerJsURL("<html></html>"); got != "" {
		t.Errorf("无 jsUrl 应返回空,得 %q", got)
	}
}

func TestExtractFunctionNameAndFixup(t *testing.T) {
	code := `var m85=function(a){a=a.split("");if(typeof b==="undefined")return a;return Y[45]};`
	name, arrayIndex, ok := extractFunctionName(code)
	if !ok || name != "m85" || arrayIndex != nil {
		t.Fatalf("extractFunctionName = %q idx=%v ok=%v", name, arrayIndex, ok)
	}
	fn, err := extractFunction(code, name)
	if err != nil {
		t.Fatal(err)
	}
	want := `function m85(a){a=a.split("");if(typeof b==="undefined")return a;return Y[45]}`
	if fn != want {
		t.Errorf("extractFunction = %q", fn)
	}
	fixed := fixupFunction(fn)
	if strings.Contains(fixed, "typeof") || !strings.Contains(fixed, "return Y[45]") {
		t.Errorf("fixupFunction = %q", fixed)
	}

	arrCode := `var cvb=["BqJ"];x=1;`
	if got := resolveArrayFunctionName(arrCode, "cvb", iptr(0)); got != "BqJ" {
		t.Errorf("resolveArrayFunctionName = %q", got)
	}
	if got := resolveArrayFunctionName(arrCode, "cvb", nil); got != "cvb" {
		t.Errorf("无数组下标应保留原名,得 %q", got)
	}
}

func TestDeobfuscateNParamUsesJSRunner(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/watch":
			io.WriteString(w, `<script>"jsUrl":"/s/player/abc/base.js"</script>`)
		case "/s/player/abc/base.js":
			io.WriteString(w, `var m85=function(a){a=a.split("");if(typeof b==="undefined")return a;return Y[45]};`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	oldOrigin := youtubeOrigin
	youtubeOrigin = server.URL
	resetPackageState()
	defer func() {
		youtubeOrigin = oldOrigin
		resetPackageState()
	}()

	sc := testContext(t)
	var gotCode, gotFn string
	var gotArg any
	sc.JS = platform.JSRunnerFunc(func(ctx context.Context, code, fn string, args []any) (any, error) {
		gotCode, gotFn = code, fn
		if len(args) > 0 {
			gotArg = args[0]
		}
		return "DECODED", nil
	})

	rawURL := "https://rr3---sn-test.googlevideo.com/videoplayback?expire=1&n=ABC123&x=1"
	if !hasThrottlingParam(rawURL) {
		t.Fatal("hasThrottlingParam 应命中")
	}
	result, err := deobfuscateNParam(context.Background(), sc, rawURL, "vid")
	if err != nil {
		t.Fatal(err)
	}
	if gotFn != "m85" || gotArg != "ABC123" {
		t.Errorf("JS 调用 = fn %q arg %v", gotFn, gotArg)
	}
	if !strings.Contains(gotCode, "function m85") || strings.Contains(gotCode, "typeof") {
		t.Errorf("JS code 形状错误: %q", gotCode)
	}
	if !strings.Contains(result, "n=DECODED") || strings.Contains(result, "n=ABC123") {
		t.Errorf("n 替换结果 = %q", result)
	}
}
