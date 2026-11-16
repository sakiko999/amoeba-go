// bili 视频榜渠道：入站必刷 / 排行榜 / 每周必看（均单页，无翻页）。
// 入站必刷与每周必看是人工榜单：pubdate 置 0，排序按接口顺序而非发布时间。

package bili

import (
	"context"
	"fmt"
	"sort"

	"mygo-host/platform"
)

// ridTable 排行榜英文分区名 → rid。
var ridTable = map[string]string{
	"all": "0", "anime": "1", "guochuang": "4", "documentary": "3", "movie": "2",
	"tv": "5", "variety": "7", "douga": "1005", "game": "1008", "kichiku": "1007",
	"music": "1003", "dance": "1004", "cinephile": "1001", "ent": "1002",
	"knowledge": "1010", "tech": "1012", "food": "1020", "car": "1013",
	"fashion": "1014", "sports": "1018", "animal": "1024",
}

func preciousChannel(c *client) platform.DataChannel {
	return platform.MakeChannel(platform.ChannelSpec{
		Key:  "bili:precious",
		Name: "B站入站必刷",
		Kind: platform.KindVideo,
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			raw, _, err := c.getJSON(ctx, apiMain+"/x/web-interface/popular/precious", reqOptions{
				Referer: "https://www.bilibili.com/v/popular/history",
				Cookie:  cookiePtr(info),
			})
			if err != nil {
				return platform.PageItems{}, err
			}
			var resp struct {
				Data *struct {
					List []videoRaw `json:"list"`
				} `json:"data"`
			}
			if err := platform.UnmarshalJSON(raw, &resp); err != nil {
				return platform.PageItems{}, err
			}
			var list []videoRaw
			if resp.Data != nil {
				list = resp.Data.List
			}
			t := c.now()
			items := make([]platform.Item, 0, len(list))
			for _, v := range list {
				in := videoInput(v)
				in.Pubdate = 0
				in.ForceBvid = true
				items = append(items, ugc("bili:precious", t, in))
			}
			return platform.PageItems{Items: items}, nil
		},
	})
}

func rankingChannel(c *client) platform.DataChannel {
	return platform.MakeChannel(platform.ChannelSpec{
		Key:     "bili:ranking",
		Name:    "B站排行榜",
		Kind:    platform.KindVideo,
		InfoTpl: []platform.SourceInfoField{optionalField("rid", "分区(all/douga/…)")},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			rid := "all"
			if v, ok := info["rid"]; ok {
				rid = v
			}
			numericRid := "0"
			if digitsRe.MatchString(rid) {
				numericRid = rid
			} else if mapped, ok := ridTable[rid]; ok {
				numericRid = mapped
			}
			url := fmt.Sprintf("%s/x/web-interface/ranking/v2?rid=%s&type=all&web_location=333.934", apiMain, numericRid)
			raw, _, err := c.getJSON(ctx, url, reqOptions{
				Referer: "https://www.bilibili.com/v/popular/rank/" + rid,
				Cookie:  cookiePtr(info),
			})
			if err != nil {
				return platform.PageItems{}, err
			}
			var resp struct {
				Data *struct {
					List []videoRaw `json:"list"`
				} `json:"data"`
			}
			if err := platform.UnmarshalJSON(raw, &resp); err != nil {
				return platform.PageItems{}, err
			}
			var list []videoRaw
			if resp.Data != nil {
				list = resp.Data.List
			}
			t := c.now()
			items := make([]platform.Item, 0, len(list))
			for _, v := range list {
				in := videoInput(v)
				if v.Desc == nil || *v.Desc == "" {
					in.Desc = deref(v.Title)
				}
				in.Pubdate = int64(jnumPtr(v.Ctime))
				items = append(items, ugc("bili:ranking", t, in))
			}
			return platform.PageItems{Items: items}, nil
		},
	})
}

// weeklySeries /x/v2/show/popular/selected/series 响应（data 直接是数组）。
type weeklySeries struct {
	Data []struct {
		Number *float64 `json:"number"`
		Name   *string  `json:"name"`
	} `json:"data"`
}

func weeklyChannel(c *client) platform.DataChannel {
	return platform.MakeChannel(platform.ChannelSpec{
		Key:  "bili:weekly",
		Name: "B站每周必看",
		Kind: platform.KindVideo,
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			raw, _, err := c.getJSON(ctx,
				"https://app.bilibili.com/x/v2/show/popular/selected/series?type=weekly_selected",
				reqOptions{Referer: "https://www.bilibili.com/h5/weekly-recommend", Cookie: cookiePtr(info)})
			if err != nil {
				return platform.PageItems{}, err
			}
			var series weeklySeries
			if err := platform.UnmarshalJSON(raw, &series); err != nil {
				return platform.PageItems{}, err
			}
			// 取期号最大的一期：接口顺序无契约，series[0] 可能是旧一期。
			if len(series.Data) == 0 {
				return platform.PageItems{}, fmt.Errorf("bilibili weekly: no series")
			}
			sort.SliceStable(series.Data, func(i, j int) bool {
				return jnumPtr(series.Data[i].Number) > jnumPtr(series.Data[j].Number)
			})
			head := series.Data[0]
			number := jnumPtr(head.Number)
			url := fmt.Sprintf("https://app.bilibili.com/x/v2/show/popular/selected?type=weekly_selected&number=%s", jsNum(number))
			referer := fmt.Sprintf("https://www.bilibili.com/h5/weekly-recommend?num=%s&navhide=1", jsNum(number))
			raw, _, err = c.getJSON(ctx, url, reqOptions{Referer: referer, Cookie: cookiePtr(info)})
			if err != nil {
				return platform.PageItems{}, err
			}
			var resp struct {
				Data *struct {
					List []videoRaw `json:"list"`
				} `json:"data"`
			}
			if err := platform.UnmarshalJSON(raw, &resp); err != nil {
				return platform.PageItems{}, err
			}
			var list []videoRaw
			if resp.Data != nil {
				list = resp.Data.List
			}
			t := c.now()
			items := make([]platform.Item, 0, len(list))
			for _, v := range list {
				in := videoInput(v)
				in.Pic = deref(v.Cover)
				in.Desc = fmt.Sprintf("%s %s - %s", deref(head.Name), deref(v.Title), deref(v.RcmdReason))
				in.Pubdate = 0
				in.Aid = int64(jnumPtr(v.Param))
				in.Owner = ""
				in.ForceBvid = number > 60
				items = append(items, ugc("bili:weekly", t, in))
			}
			return platform.PageItems{Items: items}, nil
		},
	})
}
