// bangumi 元数据平台:bgm.tv v0 API 的 provider 包成 metadata 协议 ops
// (search / fetchMetadata / fetchEpisodes)注册进网关。Go 版对齐
// 搜索 POST + JSON body、条目 type 过滤、中文名/别名
// 取名规则、每集一次请求拿全集。

package bangumi

import "mygo-host/platform"

const (
	label = "Bangumi"
	tag   = "media.metadata"
)

// New 建 bangumi 平台扩展(需要 ServiceContext 提供 HTTP 与缓存)。
func New(sc *platform.ServiceContext) platform.Spec {
	return platform.Spec{
		Namespace: "bangumi",
		Label:     label,
		Tags:      []string{tag},
		Ops:       platform.MetadataOps(newProvider(sc)),
	}
}
