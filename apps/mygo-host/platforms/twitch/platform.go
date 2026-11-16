// PLACEHOLDER —— twitch 平台 Go 移植占位(由移植任务替换)。
// 目标:对齐 packages/platform-twitch/src 的渠道/播放/弹幕能力,经 platform.FeedOps /
// platform.MetadataOps 注册;TS 测试(tests/platform.test.ts 或 provider.test.ts)
// 移植为 _test.go。

package twitch

import (
	"context"
	"encoding/json"

	"mygo-host/platform"
)

// New 建 twitch 平台扩展。
func New(sc *platform.ServiceContext) platform.Spec {
	return platform.Spec{
		Namespace: "twitch",
		Label:     "Twitch",
		Tags:      []string{"platform.data"},
		Ops: platform.Ops{
			"pending": {
				Description: "Go 移植进行中(占位 op)",
				Invoke: func(ctx context.Context, params json.RawMessage) (any, error) {
					return nil, errPending
				},
			},
		},
	}
}

var errPending = &platform.PlatformOpError{Platform: "twitch", Op: "pending", Cause: errNotPorted}

type notPortedError struct{}

func (notPortedError) Error() string { return "twitch: Go 移植未完成" }

var errNotPorted = notPortedError{}
