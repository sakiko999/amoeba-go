//go:build release

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedAssetsUnpack(t *testing.T) {
	cache := t.TempDir()
	pages, vendor, ok, err := embeddedAssets(cache)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("release 构建应包含内嵌资源")
	}
	if len(pages) == 0 {
		t.Fatal("没有页面组件")
	}
	for _, root := range pages {
		if !strings.Contains(root, filepath.Join("embedded")) {
			t.Errorf("页面根目录不在缓存: %s", root)
		}
		if _, err := os.Stat(filepath.Join(root, "page", "component.json")); err != nil {
			t.Errorf("%s: %v", root, err)
		}
	}
	if _, err := os.Stat(filepath.Join(vendor, "manifest.json")); err != nil {
		t.Error(err)
	}
}

func TestEmbeddedAssetsReusesCache(t *testing.T) {
	cache := t.TempDir()
	pages, vendor, _, err := embeddedAssets(cache)
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(filepath.Dir(pages[0]), "..", "marker")
	if err := os.WriteFile(marker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	againPages, againVendor, _, err := embeddedAssets(cache)
	if err != nil {
		t.Fatal(err)
	}
	if againPages[0] != pages[0] || againVendor != vendor {
		t.Fatalf("第二次调用应复用同一目录: %s vs %s", pages[0], againPages[0])
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("复用时应保留已有文件: %v", err)
	}
}
