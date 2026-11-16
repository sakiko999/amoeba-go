// douban 元数据平台:provider 包成 metadata 协议 ops(search / fetchMetadata /
// fetchEpisodes)注册进网关;与 TS 版一致只支持按 ID 刮削,搜索与每集能力为空。
// 标签 media.metadata 供 media-library 惰性发现。

package douban

import "mygo-host/platform"

const (
	label = "豆瓣"
	tag   = "media.metadata"
)

// New 建 douban 平台扩展(需要 ServiceContext 提供 HTTP 与缓存)。
func New(sc *platform.ServiceContext) platform.Spec {
	return platform.Spec{
		Namespace: "douban",
		Label:     label,
		Tags:      []string{tag},
		Ops:       platform.MetadataOps(newProvider(sc)),
	}
}
