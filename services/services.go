// services —— 内置服务组件的装配层:api-gateway 为根服务,各平台服务
// inject api.gateway;按 provides/inject 拓扑序激活(Activate = 经网关
// 注册 ops),并把服务清单登记进组件管理器(与 page 组件平级)。
// 服务组件 = 编译进宿主的 Go 包(§5.2 决策);不支持外部安装 kind=service。

package services

import (
	gateway "mygo-host/components/service-api-gateway"
	"mygo-host/platform"
	"mygo-host/registry"

	"mygo-host/components/platform-apple"
	"mygo-host/components/platform-bangumi"
	"mygo-host/components/platform-bili"
	"mygo-host/components/platform-douban"
	"mygo-host/components/platform-douyin"
	"mygo-host/components/platform-douyu"
	"mygo-host/components/platform-huya"
	"mygo-host/components/platform-tmdb"
	"mygo-host/components/platform-twitch"
	"mygo-host/components/platform-weibo"
	"mygo-host/components/platform-xhs"
	"mygo-host/components/platform-youtube"
)

// Activate 建网关并按拓扑序激活全部内置服务;每项服务的 manifest 平级登记
// 进组件管理器(组件清单/依赖图的数据源)。comps 必传(激活清单是装配契约);
// extra 是由宿主构造的额外服务组件(如 jsrunner——实现对象经 ServiceContext
// 供给平台,这里只做拓扑参与与清单登记)。
func Activate(sc *platform.ServiceContext, comps *registry.Manager, extra ...platform.Service) (*gateway.Gateway, error) {
	gw := gateway.New(sc.Logf)
	root, err := gateway.NewService(gw)
	if err != nil {
		return nil, err
	}
	// 内置服务工厂清单(编译期绑定,§5.2 决策;manifest 由各组件目录的
	// component.json 自描述)。
	factories := []func(*platform.ServiceContext) (platform.Service, error){
		tmdb.NewService, douban.NewService, bangumi.NewService, apple.NewService,
		youtube.NewService, bili.NewService, douyu.NewService, huya.NewService, twitch.NewService,
		weibo.NewService, xhs.NewService, douyin.NewService,
	}
	list := append([]platform.Service{root}, extra...)
	for _, newService := range factories {
		svc, err := newService(sc)
		if err != nil {
			return nil, err
		}
		list = append(list, svc)
	}
	ordered, err := topoSort(list)
	if err != nil {
		return nil, err
	}
	for _, svc := range ordered {
		if err := svc.Activate(gw); err != nil {
			return nil, err
		}
		m := svc.Manifest()
		sc.Log("service activated: %s", m.ID)
		comps.RegisterService(m)
	}
	return gw, nil
}
