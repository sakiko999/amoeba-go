// douyin 抖音号查号:iesdouyin 匿名资料接口 + 进程内缓存(键=原始抖音号,
// 无 TTL、不缓存未命中;与 TS lib/profile.ts 一致)。

package douyin

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"mygo-host/platform"
)

type userCacheState struct {
	mu sync.Mutex
	m  map[string]userHit
}

var userCache = userCacheState{m: map[string]userHit{}}

func (c *userCacheState) get(key string) (userHit, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	hit, ok := c.m[key]
	return hit, ok
}

func (c *userCacheState) put(key string, hit userHit) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = hit
}

// fetchUserByUniqueID 按抖音号查资料;查无此人返回 errAccountNotFound 包装,
// JSON 解析失败等错误原样上抛。
func fetchUserByUniqueID(ctx context.Context, sc *platform.ServiceContext, uniqueID string) (*userHit, error) {
	if hit, ok := userCache.get(uniqueID); ok {
		return &hit, nil
	}
	url := userInfoAPI + "?unique_id=" + encodeURIComponent(uniqueID)
	raw, err := sc.HTTP.GetJSON(ctx, url, map[string]string{
		"user-agent": desktopChromeUA,
		"referer":    "https://www.douyin.com/",
	})
	if err != nil {
		return nil, err
	}
	var res userInfoResponse
	if err := platform.UnmarshalJSON(raw, &res); err != nil {
		return nil, err
	}
	hit, err := hitFromUserInfo(uniqueID, res)
	if err != nil {
		return nil, err
	}
	userCache.put(uniqueID, *hit)
	return hit, nil
}

// hitFromUserInfo status_code 必须 === 0 且 sec_uid/nickname 非空。
func hitFromUserInfo(uniqueID string, res userInfoResponse) (*userHit, error) {
	detail := platform.JSString(res.StatusMsg)
	if res.UserInfo == nil {
		return nil, notFoundError(uniqueID, detail)
	}
	secUID := strings.TrimSpace(platform.JSString(res.UserInfo.SecUID))
	nickname := strings.TrimSpace(platform.JSString(res.UserInfo.Nickname))
	if n, ok := platform.JSNumber(res.StatusCode); !ok || n != 0 || secUID == "" || nickname == "" {
		return nil, notFoundError(uniqueID, detail)
	}
	hit := &userHit{
		secUID:   secUID,
		nickname: nickname,
		uniqueID: strings.TrimSpace(platform.JSString(res.UserInfo.UniqueID)),
	}
	if thumb := res.UserInfo.AvatarThumb; thumb != nil && len(thumb.URLList) > 0 && platform.JSTruthy(thumb.URLList[0]) {
		hit.avatar = platform.JSString(thumb.URLList[0])
	}
	return hit, nil
}

func notFoundError(uniqueID, detail string) error {
	if detail == "" {
		detail = "资料接口无此用户"
	}
	return fmt.Errorf("%w「%s」：%s（请检查抖音号，或改用用户主页链接）", errAccountNotFound, uniqueID, detail)
}
