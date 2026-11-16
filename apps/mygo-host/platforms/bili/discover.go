// bili 直播发现层：分区树与按分区列房（需登录 + w_webid）。
// 分区目录匿名可读；second/getList 匿名一律 -352，故先判登录再发请求（-352 会写
// 平台冷却，一次「未登录」会把用户所有 bili 渠道一起打死）。
// 搜索端点见 discover_search.go。

package bili

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"mygo-host/platform"
)

const webIDTTL = 10 * 60_000

type biliCategory struct {
	ID       string
	Name     string
	Children []biliCategoryChild
}

type biliCategoryChild struct {
	ID   string
	Name string
}

var accessIDRe = regexp.MustCompile(`"access_id":"(.*?)"`)

// fetchBiliCategories 列全部直播分区（一级含二级；匿名 + buvid）。
func (c *client) fetchBiliCategories(ctx context.Context) ([]biliCategory, error) {
	url := "https://api.live.bilibili.com/room/v1/Area/getList?need_entrance=1&parent_id=0"
	raw, _, err := c.getJSON(ctx, url, reqOptions{Referer: liveBase + "/", Buvid: true})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []map[string]any `json:"data"`
	}
	if err := platform.UnmarshalJSON(raw, &resp); err != nil {
		return nil, err
	}
	categories := []biliCategory{}
	for _, parent := range resp.Data {
		id := jstr(parent["id"])
		if id == "" {
			continue
		}
		cat := biliCategory{ID: id, Name: jstr(parent["name"])}
		for _, sub := range jlist(parent["list"]) {
			child := jobj(sub)
			cat.Children = append(cat.Children, biliCategoryChild{ID: jstr(child["id"]), Name: jstr(child["name"])})
		}
		categories = append(categories, cat)
	}
	return categories, nil
}

// getWebID w_webid（access_id）短时效 token，取自直播页 HTML；TTL 内复用。
func (c *client) getWebID(ctx context.Context, cookie *string) (string, error) {
	c.webIDMu.Lock()
	if c.webID != "" && c.now()-c.webIDAt < webIDTTL {
		value := c.webID
		c.webIDMu.Unlock()
		return value, nil
	}
	c.webIDMu.Unlock()

	headers := map[string]string{"referer": liveBase + "/"}
	if ck := c.resolveCookie(cookie); ck != "" {
		headers["cookie"] = ck
	}
	html, err := c.sc.HTTP.GetText(ctx, liveBase+"/lol", headers)
	if err != nil {
		return "", err
	}
	id := ""
	if m := accessIDRe.FindStringSubmatch(html); len(m) == 2 {
		id = strings.ReplaceAll(m[1], `\`, "")
	}
	if id == "" {
		return "", fmt.Errorf("bilibili live page: no access_id (w_webid unavailable)")
	}
	c.webIDMu.Lock()
	c.webID, c.webIDAt = id, c.now()
	c.webIDMu.Unlock()
	return id, nil
}

func (c *client) invalidateWebID() {
	c.webIDMu.Lock()
	c.webID = ""
	c.webIDMu.Unlock()
}

type biliAreaRooms struct {
	List    []any
	HasMore bool
}

// fetchBiliAreaRooms 按分区列房（wbi 签名 + buvid + w_webid；必须登录态）。
func (c *client) fetchBiliAreaRooms(ctx context.Context, parentAreaID, areaID string, page int, cookie *string) (biliAreaRooms, error) {
	ck := c.resolveCookie(cookie)
	if ck == "" {
		return biliAreaRooms{}, fmt.Errorf("B站直播分区列房需要登录：请到「账号与凭证」扫码登录")
	}
	request := func(webID string) (map[string]any, error) {
		query := fmt.Sprintf("platform=web&parent_area_id=%s&area_id=%s&sort_type=online&page=%d&w_webid=%s",
			parentAreaID, areaID, page, webID)
		signed, err := c.signWeb(ctx, query, &ck)
		if err != nil {
			return nil, err
		}
		url := "https://api.live.bilibili.com/xlive/web-interface/v1/second/getList?" + signed
		_, data, err := c.getJSON(ctx, url, reqOptions{
			Referer: liveBase + "/", Buvid: true, Cookie: &ck,
			AllowCodeError: true, NoRiskSignal: true,
		})
		return data, err
	}
	webID, err := c.getWebID(ctx, &ck)
	if err != nil {
		return biliAreaRooms{}, err
	}
	res, err := request(webID)
	if err != nil {
		return biliAreaRooms{}, err
	}
	if jnum(res["code"]) == -352 {
		// w_webid 过期/被风控 → 逐出缓存重取一次（首个 -352 不写冷却）。
		c.invalidateWebID()
		webID, err = c.getWebID(ctx, &ck)
		if err != nil {
			return biliAreaRooms{}, err
		}
		if res, err = request(webID); err != nil {
			return biliAreaRooms{}, err
		}
	}
	if code := jnum(res["code"]); code != 0 {
		return biliAreaRooms{}, fmt.Errorf("bilibili API %s: %s", jsNum(code), jstrOr(res["message"], "unknown error"))
	}
	data := jobj(res["data"])
	var list []any
	for _, raw := range jlist(data["list"]) {
		if row := jobj(raw); row != nil {
			list = append(list, row)
		}
	}
	return biliAreaRooms{List: list, HasMore: jnum(data["has_more"]) == 1}, nil
}

func liveStatusPtr(live bool) *platform.LiveStatus {
	status := platform.LiveStatusOffline
	if live {
		status = platform.LiveStatusLive
	}
	return &status
}
