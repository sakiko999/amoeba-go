package registry

import (
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// PageHandler 服务 page:// scheme：host 即组件 id（Windows 下 mygo 把自定义 scheme
// 映射成 http://<id>.page.localhost/，macOS 为 page://<id>/，两种形态都解析）。
// 路径必须是组件包内真实文件，越界一律 404（与 Electron 版同样的双校验思路）。
func (m *Manager) PageHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := schemeHost(r.Host, "page")
		root, ok := m.RootOf(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		serveComponentFile(w, r, root)
	})
}

// VendorHandler 服务 vendor:// 共享层：host 固定 "app"，只读，跨 origin 的
// ESM import 需要 ACAO（与 Electron 版一致）。
func VendorHandler(root string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if host := schemeHost(r.Host, "vendor"); host != "app" && host != "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("access-control-allow-origin", "*")
		serveComponentFile(w, r, root)
	})
}

// schemeHost 从 Host 里取出自定义 scheme 的逻辑主机名。
// 兼容 "clock.page.localhost[:port]"（Windows）与 "clock"（macOS/Linux）。
func schemeHost(host, scheme string) string {
	name := host
	if idx := strings.LastIndex(name, ":"); idx >= 0 && !strings.Contains(name[idx:], "]") {
		name = name[:idx] // 去端口（IPv6 字面量不会出现在这里的 scheme 映射中）
	}
	name = strings.TrimSuffix(name, "."+scheme+".localhost")
	if idx := strings.Index(name, "."); idx >= 0 {
		name = name[:idx]
	}
	return strings.ToLower(name)
}

func serveComponentFile(w http.ResponseWriter, r *http.Request, root string) {
	rel := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if rel == "" {
		http.NotFound(w, r)
		return
	}
	file := filepath.Join(root, filepath.FromSlash(rel))
	if !isInside(root, rel) {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(file)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	if ct := contentType(file); ct != "" {
		w.Header().Set("content-type", ct)
	}
	w.Header().Set("x-content-type-options", "nosniff")
	http.ServeContent(w, r, file, info.ModTime(), f)
}

func contentType(file string) string {
	if ct := mime.TypeByExtension(filepath.Ext(file)); ct != "" {
		return ct
	}
	switch filepath.Ext(file) {
	case ".js":
		return "text/javascript; charset=utf-8"
	case ".json", ".map":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	}
	return ""
}
