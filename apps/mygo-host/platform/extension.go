// 平台组件入口工厂(definePlatformExtension 的 Go 版)+ 统一 op 出口:
// 每个 op 失败都包装成 PlatformOpError(平台.操作 + cause),消费方按类型判别。

package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// Op 一个平台操作:params 由调用方给出(RawMessage,平台自行解码),
// 返回值原样透出(网关层 JSON 序列化)。
type Op struct {
	// Description 人读说明(list 内省用)。
	Description string
	Invoke      func(ctx context.Context, params json.RawMessage) (any, error)
}

// Ops 操作表。
type Ops map[string]Op

// ServiceContext 平台 activate 期可用的宿主能力(全部由宿主注入)。
type ServiceContext struct {
	Logf  func(format string, args ...any)
	HTTP  *HostClient
	Cache *Cache // 可为 nil(metadata 平台缺省直连)
	Creds *Credentials
	JS    JSRunner // 可为 ErrJSRunnerUnavailable 的实现(签名平台降级报错)
	Now   func() int64
}

// Log 缺省丢弃日志。
func (sc *ServiceContext) Log(format string, args ...any) {
	if sc.Logf != nil {
		sc.Logf(format, args...)
	}
}

// NowMs 当前 epoch ms(缺省用真实时钟)。
func (sc *ServiceContext) NowMs() int64 {
	if sc.Now != nil {
		return sc.Now()
	}
	return nowReal()
}

// Spec 平台扩展规格(对应 TS PlatformExtensionSpec + manifest 派生字段)。
type Spec struct {
	// Namespace 平台 id(单段小写,如 "tmdb" / "bili";组件 id 去 platform- 前缀)。
	Namespace string
	// Label 人读标签(如 "TMDB")。
	Label string
	// Tags 能力/协议标签(消费方按 tags 发现平台;可为空)。
	Tags []string
	// Ops 静态操作表。
	Ops Ops
	// OpsFn 需要宿主能力(ServiceContext)延迟构建时用,与 Ops 二选一;两者都给时合并(OpsFn 优先)。
	OpsFn func(sc *ServiceContext) (Ops, error)
}

// GuardOps 给每个 op 的失败包上 PlatformOpError(已是该类型的错误原样透出)。
func GuardOps(namespace string, ops Ops) Ops {
	guarded := make(Ops, len(ops))
	for name, op := range ops {
		opName, opInvoke := name, op.Invoke
		guarded[name] = Op{
			Description: op.Description,
			Invoke: func(ctx context.Context, params json.RawMessage) (any, error) {
				result, err := opInvoke(ctx, params)
				if err != nil {
					var opErr *PlatformOpError
					if errors.As(err, &opErr) {
						return result, err
					}
					return result, &PlatformOpError{Platform: namespace, Op: opName, Cause: err}
				}
				return result, nil
			},
		}
	}
	return guarded
}

// DecodeParams 把网关 params 解码进结构;nil params 解码为零值。
func DecodeParams(params json.RawMessage, out any) error {
	if len(params) == 0 {
		return nil
	}
	if err := json.Unmarshal(params, out); err != nil {
		return fmt.Errorf("params 解码失败: %w", err)
	}
	return nil
}
