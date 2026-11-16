// services —— 内置 Go 服务的装配层:构建 ServiceContext(宿主能力注入),
// 解析各平台 Spec(需要宿主能力的平台走 OpsFn),注册进网关并自检。
// 服务组件 = 编译进宿主的 Go 包(§5.2 决策);不再支持外部安装 kind=service。

package services

import (
	"sort"
	"strings"

	"mygo-host/gateway"
	"mygo-host/platform"

	"mygo-host/platforms/apple"
	"mygo-host/platforms/bangumi"
	"mygo-host/platforms/bili"
	"mygo-host/platforms/douban"
	"mygo-host/platforms/douyu"
	"mygo-host/platforms/huya"
	"mygo-host/platforms/tmdb"
	"mygo-host/platforms/twitch"
	"mygo-host/platforms/youtube"
)

// Activate 建网关并注册全部内置平台;logf 记注册与自检(对齐 TS 侧
// "api registered: <ns> (ops)" / "self-check ok" 日志面)。
func Activate(sc *platform.ServiceContext) (*gateway.Gateway, error) {
	gw := gateway.New(sc.Logf)
	specs := []platform.Spec{
		tmdb.New(sc),
		douban.New(sc),
		bangumi.New(sc),
		apple.New(sc),
		youtube.New(sc),
		bili.New(sc),
		douyu.New(sc),
		huya.New(sc),
		twitch.New(sc),
	}
	for _, spec := range specs {
		if err := gw.Register(spec); err != nil {
			return nil, err
		}
		sc.Log("api registered: %s (%s)", spec.Namespace, opNames(spec))
	}
	return gw, nil
}

func opNames(spec platform.Spec) string {
	names := make([]string, 0, len(spec.Ops))
	for name := range spec.Ops {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
