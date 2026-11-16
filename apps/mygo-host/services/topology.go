// 内置服务的 Kahn 拓扑排序:inject 的提供者先于消费者激活(移植 TS
// services.ts 的拓扑语义,静态清单版)。悬空 inject 与环是清单声明错误,
// 直接报错;同层保持声明顺序(Kahn FIFO + 升序入队)保证结果稳定。

package services

import (
	"fmt"
	"sort"
	"strings"

	"mygo-host/platform"
)

func topoSort(list []platform.Service) ([]platform.Service, error) {
	providedBy := map[string]string{}
	index := map[string]int{}
	for i, svc := range list {
		m := svc.Manifest()
		if _, dup := index[m.ID]; dup {
			return nil, fmt.Errorf("服务清单 id 重复:%s", m.ID)
		}
		index[m.ID] = i
		for _, p := range m.Provides {
			providedBy[p.Name] = m.ID
		}
	}

	n := len(list)
	edges := make([][]int, n) // 提供者 → 消费者
	indeg := make([]int, n)
	for i, svc := range list {
		for _, dep := range svc.Manifest().Inject {
			provider, ok := providedBy[dep]
			if !ok {
				return nil, fmt.Errorf("服务 %s inject 的 %s 无提供者", svc.Manifest().ID, dep)
			}
			p := index[provider]
			edges[p] = append(edges[p], i)
			indeg[i]++
		}
	}

	queue := make([]int, 0, n)
	for i := 0; i < n; i++ {
		if indeg[i] == 0 {
			queue = append(queue, i)
		}
	}
	ordered := make([]platform.Service, 0, n)
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		ordered = append(ordered, list[i])
		for _, j := range edges[i] {
			indeg[j]--
			if indeg[j] == 0 {
				queue = append(queue, j)
			}
		}
	}
	if len(ordered) != n {
		cycle := make([]string, 0)
		for i, d := range indeg {
			if d > 0 {
				cycle = append(cycle, list[i].Manifest().ID)
			}
		}
		sort.Strings(cycle)
		return nil, fmt.Errorf("服务依赖存在环(%s)", strings.Join(cycle, ", "))
	}
	return ordered, nil
}
