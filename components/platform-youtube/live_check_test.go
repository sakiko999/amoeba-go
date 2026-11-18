// 真实网络检查(默认跳过):GO_LIVE_CHECK=1 时跑一次 VISIONOS/ANDROID_VR 播放解析。
// TS 侧对应 src/example 的联网自检;CI/单测不默认执行。

package youtube

import (
	"context"
	"os"
	"testing"
)

func TestLiveResolveStreams(t *testing.T) {
	if os.Getenv("GO_LIVE_CHECK") != "1" {
		t.Skip("设置 GO_LIVE_CHECK=1 才跑真实网络检查")
	}
	resetPackageState()
	defer resetPackageState()
	sc := testContext(t)
	streams, err := resolveYoutubeStreams(context.Background(), sc, "dQw4w9WgXcQ")
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) == 0 {
		t.Fatal("未解析出可播流")
	}
}
