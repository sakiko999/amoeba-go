// 渠道映射测试共用夹具：按 key 取渠道、同步 fetch/fetchMore 包装，
// 以及带 nav/spi 兜底的客户端（签名/指纹请求不落网）。

package bili

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"mygo-host/platform"
)

func channelByKey(t *testing.T, list []platform.DataChannel, key string) platform.DataChannel {
	t.Helper()
	for _, c := range list {
		if c.Key() == key {
			return c
		}
	}
	t.Fatalf("渠道 %s 不存在", key)
	return nil
}

func fetchSource(t *testing.T, ch platform.DataChannel, info platform.SourceInfo) platform.FetchResult {
	t.Helper()
	result, err := ch.GetSource(info).Fetch(context.Background())
	if err != nil {
		t.Fatalf("%s fetch: %v", ch.Key(), err)
	}
	return result
}

func fetchMorePage(t *testing.T, ch platform.DataChannel, info platform.SourceInfo, cursor string) platform.FetchPage {
	t.Helper()
	pageable, ok := ch.GetSource(info).(platform.PageableSource)
	if !ok {
		t.Fatalf("%s 不支持翻页", ch.Key())
	}
	page, err := pageable.FetchMore(context.Background(), cursor, -1)
	if err != nil {
		t.Fatalf("%s fetchMore: %v", ch.Key(), err)
	}
	return page
}

func newServiceChannel(t *testing.T, respond func(*http.Request) (*http.Response, error)) (*client, *fakeServer, []platform.DataChannel) {
	t.Helper()
	fs := &fakeServer{}
	fs.respond = navResponder(testImgKey, testSubKey, func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.String(), "/x/frontend/finger/spi") {
			return jsonResponse(http.StatusOK, `{"code":0,"data":{"b_3":"ANON_B3","b_4":"ANON_B4"}}`), nil
		}
		return respond(req)
	})
	c := newClient(newTestContext(t, fs, testNow))
	return c, fs, channels(c)
}
