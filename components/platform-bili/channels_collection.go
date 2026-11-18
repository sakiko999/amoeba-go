// bili 合集 / 视频列表 / 收藏夹订阅（video，页码翻页）。
// 合集与列表的 ID 归一化是纯函数（refs.go），视频链接输入要联网换 seasonId，
// 结果按输入 memo（core 每次 refresh/loadMore 都新建 source，实例级缓存无效）。
// 收藏夹 attr!=0 是失效/私密条目，必须过滤（点开必碎）。

package bili

import (
	"context"
	"fmt"

	"mygo-host/platform"
)

const (
	seasonPageSize  = 30
	favlistPageSize = 20
)

func seasonChannel(c *client) platform.DataChannel {
	return platform.MakeChannel(platform.ChannelSpec{
		Key:      "bili:season",
		Name:     "B站视频合集",
		Kind:     platform.KindVideo,
		InfoTpl:  []platform.SourceInfoField{requiredField("seasonId", "合集 ID 或视频链接")},
		Pageable: &platform.PageableSpec{First: 2, Step: 1},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			rawInput := info["seasonId"]
			seasonID, err := c.seasonMemo.get(rawInput, func() (string, error) {
				return c.resolveSeasonID(ctx, rawInput)
			})
			if err != nil {
				return platform.PageItems{}, err
			}
			url := fmt.Sprintf("%s/x/polymer/web-space/seasons_archives_list?mid=0&season_id=%s&sort_reverse=true&page_num=%d&page_size=%d",
				apiMain, encodeURIComponent(seasonID), page, seasonPageSize)
			// 必须带 Referer（不带 -352）；匿名可读，不带 cookie。
			raw, _, err := c.getJSON(ctx, url, reqOptions{Referer: "https://www.bilibili.com/"})
			if err != nil {
				return platform.PageItems{}, err
			}
			var resp struct {
				Data *struct {
					Archives []videoRaw `json:"archives"`
				} `json:"data"`
			}
			if err := platform.UnmarshalJSON(raw, &resp); err != nil {
				return platform.PageItems{}, err
			}
			var archives []videoRaw
			if resp.Data != nil {
				archives = resp.Data.Archives
			}
			t := c.now()
			items := make([]platform.Item, 0, len(archives))
			for _, a := range archives {
				if deref(a.Bvid) == "" && jnumPtr(a.Aid) == 0 {
					continue
				}
				in := videoInput(a)
				in.Desc = ""
				in.Owner = ""
				items = append(items, ugc("bili:season", t, in))
			}
			return platform.PageItems{Items: items}, nil
		},
	})
}

// resolveSeasonID 合集输入 → seasonId（视频链接要联网换一次）。
func (c *client) resolveSeasonID(ctx context.Context, input string) (string, error) {
	ref := parseBiliSeasonRef(input)
	if ref == nil {
		return "", fmt.Errorf("bili:season 需要合集 ID 或视频链接（space.bilibili.com/<mid>/channel/collectiondetail?sid=…）")
	}
	if ref.SeasonID != "" {
		return ref.SeasonID, nil
	}
	videoID := ref.VideoID
	url := apiMain + "/x/web-interface/view?" + viewIDParam(videoID)
	raw, _, err := c.getJSON(ctx, url, reqOptions{Referer: "https://www.bilibili.com/video/" + videoID})
	if err != nil {
		return "", err
	}
	var resp viewResp
	if err := platform.UnmarshalJSON(raw, &resp); err != nil {
		return "", err
	}
	if resp.Data == nil || resp.Data.UgcSeason == nil || jnumPtr(resp.Data.UgcSeason.ID) == 0 {
		return "", fmt.Errorf("视频 %s 不属于任何合集", videoID)
	}
	return jsNum(jnumPtr(resp.Data.UgcSeason.ID)), nil
}

func seriesChannel(c *client) platform.DataChannel {
	return platform.MakeChannel(platform.ChannelSpec{
		Key:      "bili:series",
		Name:     "B站视频列表",
		Kind:     platform.KindVideo,
		InfoTpl:  []platform.SourceInfoField{requiredField("seriesId", "视频列表 ID 或链接")},
		Pageable: &platform.PageableSpec{First: 2, Step: 1},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			ref := parseBiliSeriesRef(info["seriesId"])
			if ref == nil {
				return platform.PageItems{}, fmt.Errorf("bili:series 需要视频列表 ID 或链接（space.bilibili.com/<mid>/lists/<sid>?type=series）")
			}
			mid, err := c.seriesMemo.get(ref.SeriesID, func() (string, error) {
				return c.fetchSeriesMid(ctx, ref.SeriesID)
			})
			if err != nil {
				return platform.PageItems{}, err
			}
			url := fmt.Sprintf("%s/x/series/archives?mid=%s&series_id=%s&pn=%d&ps=20",
				apiMain, encodeURIComponent(mid), encodeURIComponent(ref.SeriesID), page)
			raw, _, err := c.getJSON(ctx, url, reqOptions{Referer: "https://www.bilibili.com/"})
			if err != nil {
				return platform.PageItems{}, err
			}
			var resp struct {
				Data *struct {
					Archives []videoRaw `json:"archives"`
				} `json:"data"`
			}
			if err := platform.UnmarshalJSON(raw, &resp); err != nil {
				return platform.PageItems{}, err
			}
			var archives []videoRaw
			if resp.Data != nil {
				archives = resp.Data.Archives
			}
			t := c.now()
			items := make([]platform.Item, 0, len(archives))
			for _, a := range archives {
				if deref(a.Bvid) == "" && jnumPtr(a.Aid) == 0 {
					continue
				}
				in := videoInput(a)
				in.Owner = ""
				items = append(items, ugc("bili:series", t, in))
			}
			return platform.PageItems{Items: items}, nil
		},
	})
}

// fetchSeriesMid 查列表作者 mid（mid 传错会 code 0 但 archives 空）。
func (c *client) fetchSeriesMid(ctx context.Context, seriesID string) (string, error) {
	url := apiMain + "/x/series/series?series_id=" + encodeURIComponent(seriesID)
	raw, _, err := c.getJSON(ctx, url, reqOptions{Referer: "https://www.bilibili.com/"})
	if err != nil {
		return "", err
	}
	var resp struct {
		Data *struct {
			Meta *struct {
				Mid *float64 `json:"mid"`
			} `json:"meta"`
		} `json:"data"`
	}
	if err := platform.UnmarshalJSON(raw, &resp); err != nil {
		return "", err
	}
	if resp.Data == nil || resp.Data.Meta == nil || jnumPtr(resp.Data.Meta.Mid) == 0 {
		return "", fmt.Errorf("bili:series 找不到视频列表 %s", seriesID)
	}
	return jsNum(jnumPtr(resp.Data.Meta.Mid)), nil
}

func favlistChannel(c *client) platform.DataChannel {
	return platform.MakeChannel(platform.ChannelSpec{
		Key:      "bili:favlist",
		Name:     "B站收藏夹",
		Kind:     platform.KindVideo,
		InfoTpl:  []platform.SourceInfoField{requiredField("mediaId", "收藏夹 ID 或链接")},
		Pageable: &platform.PageableSpec{First: 2, Step: 1},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			ref := parseBiliFavlistRef(info["mediaId"])
			if ref == nil {
				return platform.PageItems{}, fmt.Errorf("bili:favlist 需要收藏夹 ID 或链接（space.bilibili.com/<mid>/favlist?fid=…；仅公开收藏夹）")
			}
			url := fmt.Sprintf("%s/x/v3/fav/resource/list?media_id=%s&platform=web&pn=%d&ps=%d",
				apiMain, encodeURIComponent(ref.MediaID), page, favlistPageSize)
			raw, _, err := c.getJSON(ctx, url, reqOptions{
				Referer: "https://www.bilibili.com/",
				Cookie:  cookiePtr(info),
			})
			if err != nil {
				return platform.PageItems{}, err
			}
			var resp struct {
				Data *struct {
					Medias  []videoRaw `json:"medias"`
					HasMore *bool      `json:"has_more"`
				} `json:"data"`
			}
			if err := platform.UnmarshalJSON(raw, &resp); err != nil {
				return platform.PageItems{}, err
			}
			var medias []videoRaw
			var hasMore *bool
			if resp.Data != nil {
				medias = resp.Data.Medias
				hasMore = resp.Data.HasMore
			}
			t := c.now()
			items := make([]platform.Item, 0, len(medias))
			for _, m := range medias {
				if jnumPtr(m.Attr) != 0 || (deref(m.Bvid) == "" && jnumPtr(m.ID) == 0) {
					continue
				}
				in := videoInput(m)
				in.Pic = deref(m.Cover)
				in.Desc = deref(m.Intro)
				in.Pubdate = int64(jnumPtr(m.Pubtime))
				in.Aid = int64(jnumPtr(m.ID))
				in.Owner = ""
				if m.Upper != nil && deref(m.Upper.Name) != "" {
					in.Owner = *m.Upper.Name
				}
				in.ForceBvid = true
				items = append(items, ugc("bili:favlist", t, in))
			}
			return platform.PageItems{Items: items, HasMore: hasMore}, nil
		},
	})
}
