// 上游 payload 解析与榜单/Lookup 合并(纯函数,fixture 测试覆盖)。
// 合并规则照 TS catalog.ts:按榜单顺序输出,Lookup 缺封面/分类时用榜单信息兜底。

package apple

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"mygo-host/platform"
)

// chartPick 榜单条目 + 榜单展示字段(封面/分类兜底用)。
type chartPick struct {
	id    string
	entry chartEntry
}

func parseChart(raw platform.RawJSON) ([]chartPick, error) {
	var res chartResponse
	if err := platform.UnmarshalJSON(raw, &res); err != nil {
		return nil, err
	}
	if res.Feed == nil {
		return []chartPick{}, nil
	}
	picks := make([]chartPick, 0, len(res.Feed.Entry))
	for _, entry := range res.Feed.Entry {
		if id := entry.imID(); id != "" {
			picks = append(picks, chartPick{id: id, entry: entry})
		}
	}
	return picks, nil
}

func (e chartEntry) imID() string {
	if e.ID == nil || e.ID.Attributes == nil || e.ID.Attributes.IMID == nil {
		return ""
	}
	return strings.TrimSpace(*e.ID.Attributes.IMID)
}

func parseLookup(raw platform.RawJSON) ([]podcast, error) {
	var res lookupResponse
	if err := platform.UnmarshalJSON(raw, &res); err != nil {
		return nil, err
	}
	return podcastsOf(res), nil
}

func podcastsOf(res lookupResponse) []podcast {
	out := make([]podcast, 0, len(res.Results))
	for _, result := range res.Results {
		if p, ok := toPodcast(result); ok {
			out = append(out, p)
		}
	}
	return out
}

// toPodcast 归一单条 Lookup/Search 结果;没有 feedUrl 或 id 的条目直接丢弃
// (接不进 rss:podcast,留着只能显示不能订阅)。
func toPodcast(r lookupResult) (podcast, bool) {
	feedURL := ""
	if r.FeedURL != nil {
		feedURL = strings.TrimSpace(*r.FeedURL)
	}
	if feedURL == "" || r.CollectionID == nil || *r.CollectionID == 0 {
		return podcast{}, false
	}
	p := podcast{
		ID:           *r.CollectionID,
		Title:        trimmed(r.CollectionName),
		Author:       trimmed(r.ArtistName),
		FeedURL:      feedURL,
		PageURL:      r.CollectionViewURL,
		Genre:        r.PrimaryGenreName,
		EpisodeCount: r.TrackCount,
	}
	if r.ArtworkURL600 != nil {
		p.Artwork = r.ArtworkURL600
	} else if r.ArtworkURL100 != nil {
		p.Artwork = r.ArtworkURL100
	}
	return p, true
}

func trimmed(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

// mergeChart 按榜单顺序输出(不依赖 Lookup 返回顺序)。
func mergeChart(entries []chartPick, found []podcast) []podcast {
	byID := make(map[int64]podcast, len(found))
	for _, p := range found {
		byID[p.ID] = p
	}
	out := make([]podcast, 0, len(entries))
	for _, entry := range entries {
		id, ok := entryID(entry.id)
		if !ok {
			continue
		}
		p, ok := byID[id]
		if !ok {
			continue
		}
		if p.Artwork == nil {
			p.Artwork = pickArtwork(entry.entry)
		}
		if p.Genre == nil {
			p.Genre = pickGenre(entry.entry)
		}
		out = append(out, p)
	}
	return out
}

// entryID 近似 JS Number(id) 后按数值匹配 collectionId。
func entryID(raw string) (int64, bool) {
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	id := int64(value)
	if float64(id) != value {
		return 0, false
	}
	return id, true
}

// pickArtwork 取最大尺寸的封面(旧接口按 55/60/170 升序给,按高度降序取第一个)。
func pickArtwork(entry chartEntry) *string {
	images := append([]chartImage(nil), entry.Images...)
	if len(images) == 0 {
		return nil
	}
	sort.SliceStable(images, func(i, j int) bool {
		return imageHeight(images[i]) > imageHeight(images[j])
	})
	return images[0].Label
}

func imageHeight(image chartImage) float64 {
	if image.Attributes == nil || image.Attributes.Height == nil {
		return 0
	}
	return jsNumber(*image.Attributes.Height)
}

func pickGenre(entry chartEntry) *string {
	if entry.Category == nil || entry.Category.Attributes == nil {
		return nil
	}
	return entry.Category.Attributes.Label
}
