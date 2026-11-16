// B 站上游响应结构（字段全可选：缺字段由映射层兜底）。
// videoRaw 是各视频列表接口（热门/排行/每周/收藏夹/合集/系列/投稿）的字段并集，
// 各渠道只读自己关心的字段。

package bili

// biliRights 稿件权限（pay=付费，arc_pay=大会员/付费，ugc_pay=充电专属）。
type biliRights struct {
	Pay    *float64 `json:"pay"`
	ArcPay *float64 `json:"arc_pay"`
	UgcPay *float64 `json:"ugc_pay"`
}

// ownerJSON 作者/UP 主（owner / upper 同形）。
type ownerJSON struct {
	Name *string `json:"name"`
}

// videoRaw 视频列表条目字段并集。
type videoRaw struct {
	Title         *string     `json:"title"`
	Pic           *string     `json:"pic"`
	Cover         *string     `json:"cover"`
	Desc          *string     `json:"desc"`
	Description   *string     `json:"description"`
	Intro         *string     `json:"intro"`
	Pubdate       *float64    `json:"pubdate"`
	Ctime         *float64    `json:"ctime"`
	Pubtime       *float64    `json:"pubtime"`
	Created       *float64    `json:"created"`
	Aid           *float64    `json:"aid"`
	ID            *float64    `json:"id"`
	Param         *float64    `json:"param"`
	Bvid          *string     `json:"bvid"`
	Duration      *float64    `json:"duration"`
	Rights        *biliRights `json:"rights"`
	Owner         *ownerJSON  `json:"owner"`
	Upper         *ownerJSON  `json:"upper"`
	Author        *string     `json:"author"`
	IsChargingArc any         `json:"is_charging_arc"`
	Attr          *float64    `json:"attr"`
	RcmdReason    *string     `json:"rcmd_reason"`
}

// navData nav 接口（wbi_img + 登录 mid；未登录 code=-101 仍带 wbi_img）。
type navData struct {
	Data *struct {
		Mid    *float64 `json:"mid"`
		WbiImg *struct {
			ImgURL *string `json:"img_url"`
			SubURL *string `json:"sub_url"`
		} `json:"wbi_img"`
	} `json:"data"`
}

// viewResp 视频详情（cid / 时长 / 所属合集）。
type viewResp struct {
	Data *struct {
		Cid       *float64 `json:"cid"`
		Duration  *float64 `json:"duration"`
		UgcSeason *struct {
			ID *float64 `json:"id"`
		} `json:"ugc_season"`
	} `json:"data"`
}

// playurlResp /x/player/playurl 响应。
type playurlResp struct {
	Data *playurlData `json:"data"`
}

type playurlData struct {
	AcceptQuality     []any      `json:"accept_quality"`
	AcceptDescription []any      `json:"accept_description"`
	Dash              *dashData  `json:"dash"`
	Durl              []durlItem `json:"durl"`
}

type durlItem struct {
	URL *string `json:"url"`
}

type dashData struct {
	Duration      *float64    `json:"duration"`
	MinBufferTime *float64    `json:"minBufferTime"`
	Video         []dashVideo `json:"video"`
	Audio         []dashAudio `json:"audio"`
}

type segmentBase struct {
	Initialization *string `json:"Initialization"`
	IndexRange     *string `json:"indexRange"`
}

type dashVideo struct {
	ID          *float64     `json:"id"`
	BaseURL     *string      `json:"baseUrl"`
	Width       *float64     `json:"width"`
	Height      *float64     `json:"height"`
	Codecs      *string      `json:"codecs"`
	Bandwidth   *float64     `json:"bandwidth"`
	SegmentBase *segmentBase `json:"SegmentBase"`
	SegmentLow  *segmentBase `json:"segment_base"`
}

type dashAudio struct {
	ID          *float64     `json:"id"`
	BaseURL     *string      `json:"baseUrl"`
	Codecs      *string      `json:"codecs"`
	Bandwidth   *float64     `json:"bandwidth"`
	SegmentBase *segmentBase `json:"SegmentBase"`
	SegmentLow  *segmentBase `json:"segment_base"`
}

// livePlayResp 直播 getRoomPlayInfo 响应（只取档位与直链所需字段）。
type livePlayResp struct {
	Data *livePlayData `json:"data"`
}

type livePlayData struct {
	PlayurlInfo *livePlayurlInfo `json:"playurl_info"`
}

type livePlayurlInfo struct {
	Playurl *livePlayurl `json:"playurl"`
}

type livePlayurl struct {
	Stream  []liveStream `json:"stream"`
	GQnDesc []qnDesc     `json:"g_qn_desc"`
}

type qnDesc struct {
	Qn   *float64 `json:"qn"`
	Desc *string  `json:"desc"`
}

type liveStream struct {
	Format []liveFormat `json:"format"`
}

type liveFormat struct {
	Codec []liveCodec `json:"codec"`
}

type liveCodec struct {
	CodecName *string  `json:"codec_name"`
	BaseURL   *string  `json:"base_url"`
	AcceptQn  []any    `json:"accept_qn"`
	CurrentQn *float64 `json:"current_qn"`
	URLInfo   []struct {
		Host  *string `json:"host"`
		Extra *string `json:"extra"`
	} `json:"url_info"`
}

// liveRoomRaw 直播热门 / 分区列房的行（同形）。
type liveRoomRaw struct {
	Roomid any     `json:"roomid"`
	Cover  *string `json:"cover"`
	Title  *string `json:"title"`
	Uname  *string `json:"uname"`
	Online any     `json:"online"`
}

// playerV2Resp /x/player/v2（UP 主章节 view_points）。
type playerV2Resp struct {
	Data *struct {
		ViewPoints []struct {
			From    *float64 `json:"from"`
			Content *string  `json:"content"`
		} `json:"view_points"`
	} `json:"data"`
}
