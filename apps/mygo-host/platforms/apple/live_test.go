// 真实网络检查(默认跳过):设置 GO_LIVE_CHECK=1 才跑 Apple 目录端到端。

package apple

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestLiveCatalog(t *testing.T) {
	if os.Getenv("GO_LIVE_CHECK") != "1" {
		t.Skip("设置 GO_LIVE_CHECK=1 才执行真实网络检查")
	}
	sc := testContext(t, &http.Client{Timeout: 30 * time.Second})
	c := &catalog{sc: sc}
	ctx := context.Background()

	chart, err := c.chart(ctx, 26, 3, "cn")
	if err != nil {
		t.Fatal(err)
	}
	if len(chart) == 0 {
		t.Fatal("榜单返回 0 条")
	}
	for _, p := range chart {
		if p.Title == "" || p.FeedURL == "" {
			t.Errorf("榜单条目字段不完整: %+v", p)
		}
	}

	found, err := c.search(ctx, "凹凸电波", 3, "cn")
	if err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 || found[0].FeedURL == "" {
		t.Fatalf("搜索结果为空或缺少 feedUrl: %+v", found)
	}
}
