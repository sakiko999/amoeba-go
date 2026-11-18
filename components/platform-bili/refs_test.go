// 移植 TS example/bili-channels.check.ts：合集/视频列表/收藏夹参数归一化 + 会员标记。

package bili

import "testing"

func TestParseBiliSeasonRef(t *testing.T) {
	cases := []struct {
		input    string
		seasonID string
		videoID  string
		ok       bool
	}{
		{"2974525", "2974525", "", true},
		{"https://space.bilibili.com/1557073149/channel/collectiondetail?sid=2974525", "2974525", "", true},
		{"bilibili.com/x?season_id=2974525", "2974525", "", true},
		{"https://www.bilibili.com/video/BV1Tb421b7mi", "", "BV1Tb421b7mi", true},
		{"BV1Tb421b7mi", "", "BV1Tb421b7mi", true},
		{"av12345", "", "av12345", true},
		{"https://example.com/video/BV1Tb421b7mi", "", "", false},
		{"hello", "", "", false},
		{"", "", "", false},
	}
	for _, c := range cases {
		ref := parseBiliSeasonRef(c.input)
		if (ref != nil) != c.ok {
			t.Errorf("parseBiliSeasonRef(%q) ok = %v, want %v", c.input, ref != nil, c.ok)
			continue
		}
		if ref == nil {
			continue
		}
		if ref.SeasonID != c.seasonID || ref.VideoID != c.videoID {
			t.Errorf("parseBiliSeasonRef(%q) = %+v, want season=%q video=%q", c.input, ref, c.seasonID, c.videoID)
		}
	}
}

func TestParseBiliSeriesRef(t *testing.T) {
	cases := []struct {
		input    string
		seriesID string
		ok       bool
	}{
		{"358495", "358495", true},
		{"https://space.bilibili.com/1557073149/lists/358495?type=series", "358495", true},
		{"https://space.bilibili.com/1557073149/channel/seriesdetail?sid=358495", "358495", true},
		{"https://www.bilibili.com/list/1557073149?sid=358495", "358495", true},
		{"space.bilibili.com/1557073149/lists/abc", "", false},
		{"https://example.com/lists/358495", "", false},
	}
	for _, c := range cases {
		ref := parseBiliSeriesRef(c.input)
		if (ref != nil) != c.ok || (ref != nil && ref.SeriesID != c.seriesID) {
			t.Errorf("parseBiliSeriesRef(%q) = %+v, want %q/%v", c.input, ref, c.seriesID, c.ok)
		}
	}
}

func TestParseBiliFavlistRef(t *testing.T) {
	cases := []struct {
		input   string
		mediaID string
		ok      bool
	}{
		{"1052622027", "1052622027", true},
		{"https://space.bilibili.com/686127/favlist?fid=1052622027&ftype=create", "1052622027", true},
		{"bilibili.com/x?media_id=1052622027", "1052622027", true},
		{"https://space.bilibili.com/686127/favlist", "", false},
		{"https://example.com/favlist?fid=1052622027", "", false},
	}
	for _, c := range cases {
		ref := parseBiliFavlistRef(c.input)
		if (ref != nil) != c.ok || (ref != nil && ref.MediaID != c.mediaID) {
			t.Errorf("parseBiliFavlistRef(%q) = %+v, want %q/%v", c.input, ref, c.mediaID, c.ok)
		}
	}
}

func TestUGCMembersOnly(t *testing.T) {
	base := ugcInput{Title: "t", Bvid: "BV1xx411c7mD", Pubdate: 1_700_000_000}
	withCharging := base
	withCharging.Charging = true
	if item := ugc("bili:user_video", 0, withCharging); item.MembersOnly == nil || !*item.MembersOnly {
		t.Error("is_charging_arc=true 应标 membersOnly")
	}
	if item := ugc("bili:user_video", 0, base); item.MembersOnly != nil {
		t.Error("非充电不应出标记")
	}
	withRights := base
	withRights.Rights = &biliRights{UgcPay: fptr(1)}
	if item := ugc("bili:user_video", 0, withRights); item.MembersOnly == nil || !*item.MembersOnly {
		t.Error("rights.ugc_pay=1 应标 membersOnly")
	}
	zeroRights := base
	zeroRights.Rights = &biliRights{Pay: fptr(0), ArcPay: fptr(0)}
	if item := ugc("bili:user_video", 0, zeroRights); item.MembersOnly != nil {
		t.Error("rights 全 0 不应误标")
	}
}

func fptr(v float64) *float64 { return &v }
