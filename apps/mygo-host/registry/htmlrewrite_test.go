// htmlrewrite 测试:夹具按 injectIntoHtmlText 的产出形态构造(import map 内联
// 脚本 + CSP sha256 hash + vendor: anchor),断言 URL/hash/source 三处同步重写。

package registry

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureHTML = `<!doctype html>
<html lang="zh-CN" class="dark">
  <head>
    <meta charset="utf-8" />
    <meta
      http-equiv="Content-Security-Policy"
      content="default-src 'self'; script-src 'self' vendor: 'sha256-HASH'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; connect-src 'self' ws:"
    />
    <title>t</title>
    <script type="importmap">MAPJSON</script>
  </head>
  <body><div id="root"></div></body>
</html>`

const mapJSON = `{"imports":{"react":"vendor://app/react-abc123.js","@demo/ui":"vendor://app/demo-ui-def456.js"}}`

func TestRewriteVendorHTML(t *testing.T) {
	old := rewriteVendorEnabled
	rewriteVendorEnabled = true
	t.Cleanup(func() { rewriteVendorEnabled = old })
	sum := sha256.Sum256([]byte(mapJSON))
	html := strings.ReplaceAll(fixtureHTML, "MAPJSON", mapJSON)
	html = strings.ReplaceAll(html, "HASH", base64.StdEncoding.EncodeToString(sum[:]))

	out := rewriteVendorHTML(html)
	if !strings.Contains(out, `"react":"http://app.vendor.localhost/react-abc123.js"`) {
		t.Errorf("import map URL 未重写:\n%s", out)
	}
	if strings.Contains(out, "vendor://app/") {
		t.Errorf("仍残留 vendor://app/")
	}
	newSum := sha256.Sum256([]byte(`{"imports":{"react":"http://app.vendor.localhost/react-abc123.js","@demo/ui":"http://app.vendor.localhost/demo-ui-def456.js"}}`))
	wantHash := "'sha256-" + base64.StdEncoding.EncodeToString(newSum[:]) + "'"
	if !strings.Contains(out, wantHash) {
		t.Errorf("CSP hash 未按重写后内容重算(缺 %s):\n%s", wantHash, out)
	}
	if !strings.Contains(out, "script-src 'self' http://app.vendor.localhost ") {
		t.Errorf("CSP vendor: source 未替换:\n%s", out)
	}
}

func TestRewriteVendorHTMLDisabled(t *testing.T) {
	old := rewriteVendorEnabled
	rewriteVendorEnabled = false
	t.Cleanup(func() { rewriteVendorEnabled = old })
	html := strings.ReplaceAll(fixtureHTML, "MAPJSON", mapJSON)
	if out := rewriteVendorHTML(html); out != html {
		t.Errorf("禁用时不应改写")
	}
}

func TestRewriteVendorHTMLNoMap(t *testing.T) {
	old := rewriteVendorEnabled
	rewriteVendorEnabled = true
	t.Cleanup(func() { rewriteVendorEnabled = old })
	html := `<html><body>no import map</body></html>`
	if out := rewriteVendorHTML(html); out != html {
		t.Errorf("无 import map 时不应改写")
	}
}

// 真机构建产物回归(先 bun run --cwd components/page-search pack):验证重写规则
// 与 injectIntoHtmlText 的实际产出形态匹配,特别是 CSP hash 语义(脚本元素文本内容)。
func TestRewriteRealArtifact(t *testing.T) {
	old := rewriteVendorEnabled
	rewriteVendorEnabled = true
	t.Cleanup(func() { rewriteVendorEnabled = old })

	const artifact = `../components/page-search/out/page/renderer/index.html`
	data, err := os.ReadFile(artifact)
	if err != nil {
		t.Skip("构建产物不存在(先 bun run --cwd apps/mygo-host/components/page-search pack)")
	}
	html := string(data)
	out := rewriteVendorHTML(html)
	if strings.Contains(out, "vendor://app/") {
		t.Errorf("重写后仍残留 vendor://app/")
	}
	if strings.Contains(out, cspVendorAnchor) {
		t.Errorf("CSP vendor: source 未替换")
	}
	// 重写后 CSP 中的 hash 必须等于重写后 import map JSON 的 hash,
	// 否则整段 import map 会被 CSP 拦下(静默失败,比 CORS 报错更难查)。
	loc := importMapRe.FindStringSubmatchIndex(out)
	if loc == nil {
		t.Fatal("重写后找不到 import map")
	}
	want := "'sha256-" + scriptHash(out[loc[2]:loc[3]]) + "'"
	if !strings.Contains(out, want) {
		t.Errorf("CSP hash 与重写后 import map 不匹配(缺 %s)", want)
	}
}

// serveComponentFile 对 .html 响应应用重写;JS 等其他文件原样服务。
func TestServeComponentFileRewritesHTML(t *testing.T) {
	old := rewriteVendorEnabled
	rewriteVendorEnabled = true
	t.Cleanup(func() { rewriteVendorEnabled = old })

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "renderer"), 0o755); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(mapJSON))
	html := strings.ReplaceAll(strings.ReplaceAll(fixtureHTML, "MAPJSON", mapJSON),
		"HASH", base64.StdEncoding.EncodeToString(sum[:]))
	htmlPath := filepath.Join(dir, "renderer", "index.html")
	if err := os.WriteFile(htmlPath, []byte(html), 0o644); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/renderer/index.html", nil)
	serveComponentFile(rec, req, dir)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "http://app.vendor.localhost/react-abc123.js") {
		t.Errorf("HTML 响应未重写 vendor URL:\n%s", body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("content-type = %q", ct)
	}
}
