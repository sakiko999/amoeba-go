// douyu 上游响应结构:字段全可选,形态不稳的字段用 any 承接,
// 由 helpers 按 TS 的 String() / Number() 语义归一。

package douyu

type homeH5EncResponse struct {
	Data map[string]any `json:"data"`
}

type h5PlayResponse struct {
	Data map[string]any `json:"data"`
}

// roomPayload betard/{roomId} 的房间字段(响应可能包一层 room,也可能平铺)。
type roomPayload struct {
	RoomName    any      `json:"room_name"`
	RoomPic     any      `json:"room_pic"`
	OwnerName   any      `json:"owner_name"`
	OwnerAvatar any      `json:"owner_avatar"`
	RoomID      any      `json:"room_id"`
	ShowStatus  any      `json:"show_status"`
	VideoLoop   any      `json:"videoLoop"`
	RoomBizAll  *roomBiz `json:"room_biz_all"`
	ShowDetails any      `json:"show_details"`
	UpID        any      `json:"up_id"`
}

type roomBiz struct {
	Hot any `json:"hot"`
}

type categoryListResponse struct {
	Data struct {
		Cate1Info []map[string]any `json:"cate1Info"`
		Cate2Info []map[string]any `json:"cate2Info"`
	} `json:"data"`
}

type hotListResponse struct {
	Data struct {
		Rl []map[string]any `json:"rl"`
	} `json:"data"`
}

type directoryResponse struct {
	Data struct {
		Rl    []map[string]any `json:"rl"`
		Pgcnt any              `json:"pgcnt"`
	} `json:"data"`
}

type searchShowResponse struct {
	Data struct {
		RelateShow []map[string]any `json:"relateShow"`
	} `json:"data"`
}

type searchUserResponse struct {
	Data struct {
		RelateUser []map[string]any `json:"relateUser"`
	} `json:"data"`
}

// 回放系响应(docs/douyu-replay-research.md):条目字段形态不稳,统一
// map[string]any 承接、映射层按 helpers 的 JS 语义归一。

type replayListResponse struct {
	Error any `json:"error"`
	Msg   any `json:"msg"`
	Data  struct {
		Count any              `json:"count"`
		List  []map[string]any `json:"list"`
	} `json:"data"`
}

type replayPartsResponse struct {
	Error any `json:"error"`
	Data  struct {
		List []map[string]any `json:"list"`
	} `json:"data"`
}

type vodStreamResponse struct {
	Error any `json:"error"`
	Msg   any `json:"msg"`
	Data  struct {
		ThumbVideo map[string]map[string]any `json:"thumb_video"`
	} `json:"data"`
}

type barrageResponse struct {
	Error any `json:"error"`
	Data  struct {
		List    []map[string]any `json:"list"`
		EndTime any              `json:"end_time"`
	} `json:"data"`
}
