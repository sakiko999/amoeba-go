// 通用登录环境测试：二维码抓取失败时不得回退远程 URL。

package credentials

import (
	"context"
	"net/http"
	"testing"

	"mygo-host/platform/login/logintest"
)

func TestFetchImageRejectsInvalidBody(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/blank.png", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("<html>blocked</html>"))
	})
	env, srv := logintest.Env(mux, &logintest.Browser{})
	t.Cleanup(srv.Close)

	if got := env.FetchImage(context.Background(), "https://img.example/blank.png", ""); got != "" {
		t.Fatalf("错误响应被当成二维码: %q", got)
	}
	if got := env.FetchImage(context.Background(), "https://img.example/missing.png", ""); got != "" {
		t.Fatalf("下载失败回退成远程 URL: %q", got)
	}
}
