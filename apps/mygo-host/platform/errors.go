// platform 包是 platform-kit 运行时与 @tabs/platform-contracts 的 Go 版:
// 平台组件(内置于宿主)依赖这里的统一 HTTP/WS/JS 执行面、feed/metadata 契约、
// gateway 协议 ops 装配(feedOps/metadataOps)与错误形状。
// JSON 字段名与 TS 侧严格对齐(camelCase + omitempty),消费方(页面/域服务)
// 不感知平台实现语言。

package platform

import (
	"encoding/json"
	"fmt"
)

// ProviderError 媒体 provider 通用错误(source 前缀)。
type ProviderError struct {
	Source  string
	Message string
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Source, e.Message)
}

// NotFoundError 条目不存在(404 语义)。
type NotFoundError struct {
	Message string
}

func (e *NotFoundError) Error() string { return e.Message }

// PlatformOpError 网关 op 失败的统一出口:携带 平台.操作 上下文与原始 cause。
// 消费方按错误类型判别,不解析文案。
type PlatformOpError struct {
	Platform string
	Op       string
	Cause    error
}

func (e *PlatformOpError) Error() string {
	return fmt.Sprintf("[%s.%s] %v", e.Platform, e.Op, e.Cause)
}

func (e *PlatformOpError) Unwrap() error { return e.Cause }

// RawJSON 是「上游原样 payload」字段的类型(Item.Raw 等):延迟解码,诊断用。
type RawJSON = json.RawMessage
