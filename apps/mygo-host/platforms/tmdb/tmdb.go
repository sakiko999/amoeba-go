// tmdb 元数据平台:provider 包成 metadata 协议 ops 注册进网关。
// Go 版对齐 packages/platform-tmdb(v4 read access token 打 v3 端点;
// append_to_response=credits,external_ids;中文标题 language=zh-CN;
// 凭证只从环境变量 TMDB_TOKEN 读)。

package tmdb

import (
	"mygo-host/platform"
)

const (
	label = "TMDB"
	tag   = "media.metadata"
)

// New 建 tmdb 平台扩展(需要 ServiceContext 提供 HTTP 与缓存)。
func New(sc *platform.ServiceContext) platform.Spec {
	return platform.Spec{
		Namespace: "tmdb",
		Label:     label,
		Tags:      []string{tag},
		Ops:       platform.MetadataOps(newProvider(sc)),
	}
}
