// WebView2 上 mygo 把自定义 scheme 的页面映射为 http://[host.]scheme.localhost,
// 页面内部字面的 vendor:// URL 不经过导航重写,会被 Chromium 以「scheme 不支持
// CORS」在网络层直接拒绝(Electron 侧 vendor:// 是 privileged scheme,无此问题;
// macOS/Linux 的 mygo scheme 即原生 scheme,vendor:// 直接可用)。这里在服务页面
// HTML 时把 vendor://app/ 重写成映射后的 http://app.vendor.localhost/,该请求由
// mygo 的 WebResourceRequested 映射回 vendor://app/ 交给 VendorHandler。
// import map 是内联脚本,CSP 里有它的 sha256 hash:内容变了必须重算,否则整段
// map 被 CSP 拦下;CSP 的 scheme source `vendor:` 也要换成对应 host source。

package registry

import (
	"crypto/sha256"
	"encoding/base64"
	"regexp"
	"runtime"
	"strings"
)

const (
	vendorSchemeBase = "vendor://app/"
	vendorHTTPBase   = "http://app.vendor.localhost"
	// 与 page-vendor/scripts/import-map.ts 的 CSP_ANCHOR 保持一致
	cspVendorAnchor = "script-src 'self' vendor:"
)

// rewriteVendorEnabled 抽成 var 供测试关闭(仅 Windows 的 WebView2 需要重写)。
var rewriteVendorEnabled = runtime.GOOS == "windows"

var importMapRe = regexp.MustCompile(`(?s)<script type="importmap">(\{.*\})</script>`)

// scriptHash 计算 CSP 'sha256-…' source 对应的内联脚本内容 hash。
func scriptHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return base64.StdEncoding.EncodeToString(sum[:])
}

// rewriteVendorHTML 重写页面 HTML 的 vendor URL(无 import map 或非 Windows 时原样返回)。
func rewriteVendorHTML(html string) string {
	if !rewriteVendorEnabled || !strings.Contains(html, vendorSchemeBase) {
		return html
	}
	loc := importMapRe.FindStringSubmatchIndex(html)
	if loc == nil {
		return html
	}
	mapJSON := html[loc[2]:loc[3]]
	rewritten := strings.ReplaceAll(mapJSON, vendorSchemeBase, vendorHTTPBase+"/")
	out := html[:loc[2]] + rewritten + html[loc[3]:]
	// 旧 hash 精确匹配才替换:页面可能有其他内联脚本的 hash,不能动
	out = strings.Replace(out,
		"'sha256-"+scriptHash(mapJSON)+"'", "'sha256-"+scriptHash(rewritten)+"'", 1)
	out = strings.Replace(out, cspVendorAnchor,
		"script-src 'self' "+vendorHTTPBase, 1)
	return out
}
