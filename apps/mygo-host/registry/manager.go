package registry

import (
	"os"
	"path/filepath"
	"sort"

	"mygo-host/platform"
)

// Source 区分内置（随宿主分发）与外部安装的组件。
type Source = string

const (
	SourceBuiltin  Source = "builtin"
	SourceExternal Source = "external"
)

// Component 是扫描后的一份页面组件记录；v1 只有 active 一种终态（页面包无 main，
// 装载失败只发生在清单校验阶段，校验失败直接不入表）。
type Component struct {
	Manifest *Manifest
	Source   Source
}

// ServiceRecord 已激活的内置服务组件（编译期清单，与 page 平级展示；
// 供组件清单/依赖图消费）。
type ServiceRecord struct {
	Manifest platform.ServiceManifest
}

// Manager 负责内置/外置页面包的扫描与目录服务,以及内置服务的登记。
// 状态权威源仍在宿主主进程,tab 生命周期由 tabs.Manager 消费 Kinds();
// 服务登记发生在启动期(装配层单线程调用),不需要锁。
type Manager struct {
	builtinDir  string
	externalDir string
	byID        map[string]*Component
	services    []ServiceRecord
	// Logf 由宿主注入（缺省丢弃）：记录被跳过的目录与原因
	Logf func(format string, args ...any)
}

func NewManager(builtinDir, externalDir string) *Manager {
	return &Manager{
		builtinDir:  builtinDir,
		externalDir: externalDir,
		byID:        make(map[string]*Component),
		Logf:        func(string, ...any) {},
	}
}

// RegisterService 登记一个已激活的内置服务(装配层按拓扑序调用)。
func (m *Manager) RegisterService(manifest platform.ServiceManifest) {
	m.services = append(m.services, ServiceRecord{Manifest: manifest})
}

// Services 返回全部内置服务(激活序,拷贝防调用方改写)。
func (m *Manager) Services() []ServiceRecord {
	out := make([]ServiceRecord, len(m.services))
	copy(out, m.services)
	return out
}

// Scan 全量重建组件表：内置目录优先，外部同 id 跳过；非法目录记日志后跳过，
// 不影响其它组件（与 Electron 版 scan 语义一致）。
func (m *Manager) Scan() {
	m.byID = make(map[string]*Component)
	m.scanDir(m.builtinDir, SourceBuiltin, false)
	m.scanDir(m.externalDir, SourceExternal, true)
}

func (m *Manager) scanDir(dir string, source Source, skipDuplicate bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // 目录不存在是常态（首次运行、未安装外部组件）
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "node_modules" {
			continue
		}
		full := filepath.Join(dir, entry.Name())
		manifest, err := readManifest(full)
		if err != nil {
			m.Logf("跳过 %s（%s）: %v", full, source, err)
			continue
		}
		if existing, ok := m.byID[manifest.ID]; ok && skipDuplicate {
			m.Logf("外部组件 %s 与 %s 同 id，跳过", full, existing.Source)
			continue
		}
		m.byID[manifest.ID] = &Component{Manifest: manifest, Source: source}
	}
}

func (m *Manager) Get(id string) *Component {
	return m.byID[id]
}

// Pages 返回全部页面组件（按 id 排序保证菜单稳定）。
func (m *Manager) Pages() []*Component {
	pages := make([]*Component, 0, len(m.byID))
	for _, c := range m.byID {
		pages = append(pages, c)
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Manifest.ID < pages[j].Manifest.ID })
	return pages
}

// RootOf 供 page:// 协议按组件 id 找包根。
func (m *Manager) RootOf(id string) (string, bool) {
	c := m.byID[id]
	if c == nil {
		return "", false
	}
	return c.Manifest.Root(), true
}
