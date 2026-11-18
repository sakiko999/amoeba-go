// ParseServiceManifest 的文件形态校验:合法清单解析、kind 防混、坏 JSON 报错。

package platform

import (
	"strings"
	"testing"
)

func TestParseServiceManifest(t *testing.T) {
	m, err := ParseServiceManifest([]byte(`{
		"id": "platform-x", "kind": "service", "name": "X", "version": "0.1.0",
		"main": "main.js",
		"provides": [{"name": "svc.x", "expose": ["a"]}],
		"inject": ["api.gateway"]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "platform-x" || len(m.Inject) != 1 || m.Provides[0].Name != "svc.x" {
		t.Errorf("解析结果不符: %+v", m)
	}
}

func TestParseServiceManifestRejectsPageKind(t *testing.T) {
	_, err := ParseServiceManifest([]byte(`{"id":"home","kind":"page","name":"Home","version":"0.1.0"}`))
	if err == nil || !strings.Contains(err.Error(), "service") {
		t.Errorf("page 清单应被拒,got %v", err)
	}
}

func TestParseServiceManifestBadJSON(t *testing.T) {
	if _, err := ParseServiceManifest([]byte(`{`)); err == nil {
		t.Error("坏 JSON 应报错")
	}
}
