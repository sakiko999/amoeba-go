// UGC 视频共享映射（对齐 TS impl/video-common.ts 的 ugc）：
// 上游视频对象 → Video item（kind=video，stream 留空——playurl 直链由下游懒解析）。

package bili

import (
	"strconv"

	"mygo-host/platform"
)

// bvidTime 该时间点之后的稿件有 bvid，链接优先走 /video/BV…。
const bvidTime = 1_589_990_400

// ugcInput 归一后的视频字段（各渠道从上游取数后填充）。
type ugcInput struct {
	Title     string
	Pic       string
	Desc      string
	Pubdate   int64
	Aid       int64
	Bvid      string
	Owner     string
	Duration  *float64
	Rights    *biliRights
	Charging  any
	ForceBvid bool
}

// ugc 上游视频 → Video item。playable stream 由下游按 bvid 懒解析。
func ugc(sourceID string, t int64, v ugcInput) platform.Item {
	useBvid := (v.Pubdate >= bvidTime || v.ForceBvid) && v.Bvid != ""
	link := "https://www.bilibili.com/video/av" + strconv.FormatInt(v.Aid, 10)
	if useBvid {
		link = "https://www.bilibili.com/video/" + v.Bvid
	}
	id := v.Bvid
	if id == "" {
		id = "av" + strconv.FormatInt(v.Aid, 10)
	}
	item := platform.Item{
		ID:        id,
		SourceID:  sourceID,
		Kind:      platform.KindVideo,
		Title:     v.Title,
		URL:       platform.Str(link),
		FetchedAt: t,
	}
	if v.Desc != "" {
		item.Summary = platform.Str(v.Desc)
	}
	if v.Pic != "" {
		item.Thumbnail = platform.Str(v.Pic)
		item.Poster = platform.Str(v.Pic)
	}
	if v.Owner != "" {
		item.Author = &platform.Author{Name: v.Owner}
		item.Channel = &platform.ItemChannel{Name: v.Owner}
	}
	if v.Pubdate != 0 {
		item.PublishedAt = platform.Int64(v.Pubdate * 1000)
	}
	if v.Duration != nil {
		item.Duration = v.Duration
	}
	if isPaywalled(v.Rights) || isCharging(v.Charging) {
		item.MembersOnly = platform.Bool(true)
	}
	return item
}

// videoInput videoRaw → ugcInput（各渠道可覆盖个别字段后调用）。
func videoInput(v videoRaw) ugcInput {
	in := ugcInput{
		Title:    derefOr(v.Title, "(untitled)"),
		Pic:      deref(v.Pic),
		Desc:     deref(v.Desc),
		Pubdate:  int64(jnumPtr(v.Pubdate)),
		Aid:      int64(jnumPtr(v.Aid)),
		Bvid:     deref(v.Bvid),
		Duration: v.Duration,
		Rights:   v.Rights,
	}
	if v.Owner != nil {
		in.Owner = deref(v.Owner.Name)
	}
	return in
}

// isPaywalled rights 任一付费位为 1（列表可见但播放受限 → 统一会员徽标）。
func isPaywalled(r *biliRights) bool {
	if r == nil {
		return false
	}
	return jnumPtr(r.Pay) == 1 || jnumPtr(r.ArcPay) == 1 || jnumPtr(r.UgcPay) == 1
}

// isCharging is_charging_arc 的两种形态（bool true / number 1）。
func isCharging(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x == 1
	case int:
		return x == 1
	}
	return false
}

func jnumPtr(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}
