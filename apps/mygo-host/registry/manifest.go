// Package registry 实现宿主的组件注册表（mygo 版 v1）：
// 只管理 kind=page 的静态页面包（component.json + renderer/）；service 组件按
// 2026-11-13 决策全部为宿主内置 Go 实现，不参与目录扫描与安装，激活后由
// 装配层登记进本注册表（与 page 平级）。源码组织上，components/ 目录收纳
// 全部内置 service 组件（子目录名 = 组件 id，component.json 经 go:embed 进
// 二进制）——那是源码集合，不是运行时扫描对象；运行时扫描与安装仍只针对 page。
// 校验规则移植自 apps/desktop/src/main/pages/manifest.ts（Electron 版）。
package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var idPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$`)

// ProvidesName 服务名规则保留（未来内置服务声明用）：两段小写 DNS label。
var serviceNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*\.[a-z][a-z0-9-]*$`)

// Manifest 是 component.json 的运行时形态（v2：页面不再有 main/preload）。
// Main/Preload 字段仍会被解析但被忽略——旧产物可装载，不因此报错。
type Manifest struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Version string   `json:"version"`
	Kind    string   `json:"kind"`
	Entry   string   `json:"entry"`
	Preload string   `json:"preload"`
	Color   string   `json:"color"`
	Main    string   `json:"main"`
	Inject  []string `json:"inject"`
	// root 是组件包根目录（manifest 所在目录），校验与 page:// 服务都以它为基准
	root string
}

func (m *Manifest) Root() string { return m.root }

// EntryURL 返回该页面在 page:// scheme 下的装载 URL。
func (m *Manifest) EntryURL() string { return "page://" + m.ID + "/" + m.Entry }

// readManifest 读取并校验一个组件目录；dir 必须含 component.json（page.json 回退）。
// 返回的错误带组件目录前缀，供扫描方记日志。
func readManifest(dir string) (*Manifest, error) {
	data, err := readManifestFile(dir)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("component.json 解析失败: %w", err)
	}
	m.root = dir
	if err := m.validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

func readManifestFile(dir string) ([]byte, error) {
	for _, name := range []string{"component.json", "page.json"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("缺少 component.json/page.json")
}

func (m *Manifest) validate() error {
	if !idPattern.MatchString(m.ID) {
		return fmt.Errorf("id %q 不合法（小写 DNS label，1-64 字符）", m.ID)
	}
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("name 不能为空")
	}
	if m.Kind == "" {
		m.Kind = "page"
	}
	if m.Kind != "page" && m.Kind != "service" {
		return fmt.Errorf("kind 只允许 page|service，得到 %q", m.Kind)
	}
	if m.Kind == "service" {
		return fmt.Errorf("kind=service 组件须为宿主内置 Go 实现，不支持目录装载")
	}
	for _, name := range m.Inject {
		if !serviceNamePattern.MatchString(name) || strings.HasPrefix(name, "host.") {
			return fmt.Errorf("inject 服务名 %q 不合法", name)
		}
	}
	if err := m.validateFile("entry", m.Entry, ".html", true); err != nil {
		return err
	}
	if err := m.validateFile("preload", m.Preload, ".js", false); err != nil {
		return err
	}
	m.Color = normalizeColor(m.Color)
	return nil
}

// validateFile 校验相对路径字段：必须在包根内（防 ../ 穿越）且存在、扩展名符合。
// required=false 时空值直接通过。
func (m *Manifest) validateFile(field, rel, ext string, required bool) error {
	if rel == "" {
		if required {
			return fmt.Errorf("%s 不能为空", field)
		}
		return nil
	}
	if filepath.Ext(rel) != ext {
		return fmt.Errorf("%s %q 扩展名必须是 %s", field, rel, ext)
	}
	if !isInside(m.root, rel) {
		return fmt.Errorf("%s %q 越出组件包根", field, rel)
	}
	if _, err := os.Stat(filepath.Join(m.root, rel)); err != nil {
		return fmt.Errorf("%s %q 不存在", field, rel)
	}
	return nil
}

// isInside 判断 rel 解析后仍落在 root 内（Electron 版的 resolve+isInside 简化：
// 不做 realpath 二次校验，Windows 符号链接场景按已知上限接受）。
func isInside(root, rel string) bool {
	clean := filepath.Clean(filepath.Join(root, rel))
	abs, err := filepath.Abs(clean)
	if err != nil {
		return false
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	return abs == rootAbs || strings.HasPrefix(abs, rootAbs+string(filepath.Separator))
}

// normalizeColor 非法颜色静默丢弃（与 Electron 版一致），返回空串表示无色。
func normalizeColor(color string) string {
	if len(color) != 7 || color[0] != '#' {
		return ""
	}
	for _, c := range color[1:] {
		isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
		if !isHex {
			return ""
		}
	}
	return color
}
