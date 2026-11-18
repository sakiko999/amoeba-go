// 虎牙 HTTP 端点、请求 UA 与 profileRoom 取数。
// profileRoom 是房间元数据(房间 channel)与拉流解析共用的单一数据源;
// 所有请求走宿主 HTTP 面(非 2xx 由 GetJSON/GetText 报错)。

package huya

import (
	"context"
	"fmt"
	"net/url"

	"mygo-host/platform"
)

const (
	mHuya       = "https://m.huya.com"
	profileAPI  = "https://mp.huya.com/cache.php?m=Live&do=profileRoom"
	liveListAPI = "https://www.huya.com/cache.php?m=LiveList&do=getLiveListByPage&tagAll=0"
	bussLiveAPI = "https://live.cdn.huya.com/liveconfig/game/bussLive"
	searchAPI   = "https://search.cdn.huya.com/?m=Search&do=getSearchContent"

	uaMobile  = "Mozilla/5.0 (Linux; Android 10; SM-G973F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.91 Mobile Safari/537.36"
	uaAndroid = "Mozilla/5.0 (Linux; Android 11; Pixel 5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.91 Mobile Safari/537.36 Edg/117.0.0.0"
	uaDesktop = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36"

	// playUA HLS CDN 只认这条播放器 UA(浏览器 UA 回 403)。
	playUA = "HYSDK(Windows, 30000002)_APP(pc_exe&7060000&official)_SDK(trans&2.32.3.5646)"
)

// fetchProfileRoom 取房间详情(播放解析与房间 channel 共用)。
func fetchProfileRoom(ctx context.Context, sc *platform.ServiceContext, roomID string) (*profileRoomData, error) {
	endpoint := fmt.Sprintf("%s&roomid=%s&showSecret=1", profileAPI, url.QueryEscape(roomID))
	raw, err := sc.HTTP.GetJSON(ctx, endpoint, map[string]string{
		"user-agent": uaMobile,
		"referer":    "https://www.huya.com/",
		"origin":     "https://www.huya.com",
	})
	if err != nil {
		return nil, err
	}
	var res profileRoomResponse
	if err := platform.UnmarshalJSON(raw, &res); err != nil {
		return nil, err
	}
	if res.Status == nil || numVal(res.Status) != 200 || res.Data == nil {
		return nil, fmt.Errorf("huya: profileRoom 失败(room %s)", roomID)
	}
	return res.Data, nil
}
