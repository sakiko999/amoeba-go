// 内置页面资源：release 构建把 components/*/out 与 vendor 共享层嵌入二进制，
// 首次启动按内容 hash 解包到缓存目录，之后直接复用。开发构建（无 release
// build tag）仍从工作区目录读取，保持热更新与调试习惯不变。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// embeddedAssets 返回解包后的页面组件根目录与 vendor 目录。第二个返回值为
// false 表示当前是开发构建，调用方应回退到工作区目录。
func embeddedAssets(cacheDir string) (pageRoots []string, vendorDir string, ok bool, err error) {
	embedded, ok := releaseEmbeddedFS()
	if !ok {
		return nil, "", false, nil
	}
	data, err := fs.ReadFile(embedded, "digest")
	if err != nil {
		return nil, "", false, err
	}
	digest := strings.TrimSpace(string(data))
	if len(digest) != 64 {
		return nil, "", false, fmt.Errorf("内嵌资源摘要无效: %q", digest)
	}
	dir, err := unpackEmbedded(embedded, digest, cacheDir)
	if err != nil {
		return nil, "", false, err
	}
	pages, err := pageRootsIn(dir)
	if err != nil {
		return nil, "", false, err
	}
	return pages, filepath.Join(dir, "vendor"), true, nil
}

func unpackEmbedded(embedded fs.FS, digest, cacheDir string) (string, error) {
	dir := filepath.Join(cacheDir, "embedded", digest)
	if _, err := os.Stat(filepath.Join(dir, ".ready")); err == nil {
		return dir, nil
	}
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", err
	}
	work, err := os.MkdirTemp(parent, "."+filepath.Base(dir)+"-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(work)

	h := sha256.New()
	if err := fs.WalkDir(embedded, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "digest" || entry.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(embedded, path)
		if err != nil {
			return err
		}
		h.Write([]byte(path))
		h.Write(data)
		target := filepath.Join(work, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	}); err != nil {
		return "", err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != digest {
		return "", fmt.Errorf("内嵌资源校验失败: got %s, want %s", got, digest)
	}
	if err := os.WriteFile(filepath.Join(work, ".ready"), nil, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(work, dir); err != nil {
		if _, statErr := os.Stat(filepath.Join(dir, ".ready")); statErr == nil {
			return dir, nil
		}
		_ = os.RemoveAll(dir)
		return dir, os.Rename(work, dir)
	}
	return dir, nil
}

func pageRootsIn(dir string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "pages", "*"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	var roots []string
	for _, root := range matches {
		if _, err := os.Stat(filepath.Join(root, "page", "component.json")); err == nil {
			roots = append(roots, root)
		}
	}
	if len(roots) == 0 {
		return nil, fmt.Errorf("内嵌资源没有页面组件: %s", dir)
	}
	return roots, nil
}
