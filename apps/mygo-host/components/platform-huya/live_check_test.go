// 真实网络自检(默认跳过):GO_LIVE_CHECK=1 go test ./platforms/huya/ -run Live。
// 覆盖 profileRoom 取数(播放解析共用)与房间 channel 拉取;断网/风控环境不要开。

package huya

import (
	"context"
	"os"
	"testing"

	"mygo-host/platform"
)

func skipUnlessLive(t *testing.T) {
	t.Helper()
	if os.Getenv("GO_LIVE_CHECK") != "1" {
		t.Skip("设置 GO_LIVE_CHECK=1 才跑真实网络自检")
	}
}

func TestLiveProfileRoom(t *testing.T) {
	skipUnlessLive(t)
	data, err := fetchProfileRoom(context.Background(), testContext(t), "11342412")
	if err != nil {
		t.Fatal(err)
	}
	if data.LiveData == nil {
		t.Fatal("liveData 缺失")
	}
	if _, err := playStreams("11342412", data); err != nil {
		t.Logf("当前房间无线路(可能未开播): %v", err)
	}
}

func TestLiveChannelFetch(t *testing.T) {
	skipUnlessLive(t)
	source := newLiveChannel(testContext(t)).GetSource(platform.SourceInfo{"roomIds": "11342412"})
	result, err := source.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].Kind != platform.KindLive {
		t.Errorf("items = %+v", result.Items)
	}
}
