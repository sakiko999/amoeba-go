// douyin 上游响应结构:用户资料(iesdouyin)与 enter API 房间/流数据。
// 字段形态不稳的用 any,映射层经 platform.JSString/ToInt 归一。

package douyin

import "encoding/json"

type userInfoResponse struct {
	StatusCode any          `json:"status_code"`
	StatusMsg  any          `json:"status_msg"`
	UserInfo   *userProfile `json:"user_info"`
}

type userProfile struct {
	SecUID      any      `json:"sec_uid"`
	Nickname    any      `json:"nickname"`
	UniqueID    any      `json:"unique_id"`
	AvatarThumb *urlList `json:"avatar_thumb"`
}

// urlList 通用 url_list 容器。
type urlList struct {
	URLList []any `json:"url_list"`
}

// enterResponse enter API 响应(data.data[0] 与 data.room 两种形状)。
type enterResponse struct {
	StatusCode any       `json:"status_code"`
	Prompts    any       `json:"prompts"`
	StatusMsg  any       `json:"status_msg"`
	Data       enterData `json:"data"`
}

type enterData struct {
	Data []enterRoom `json:"data"`
	Room *enterRoom  `json:"room"`
}

type enterRoom struct {
	IDStr     any             `json:"id_str"`
	ID        any             `json:"id"`
	StreamURL json.RawMessage `json:"stream_url"`
}

// reflowResponse reflow/info 响应:流在 data.room.stream_url 或 data.stream_url。
type reflowResponse struct {
	Data reflowData `json:"data"`
}

type reflowData struct {
	Room      *reflowRoom     `json:"room"`
	StreamURL json.RawMessage `json:"stream_url"`
}

type reflowRoom struct {
	StreamURL json.RawMessage `json:"stream_url"`
}

// streamURL 取流字段:room 存在时只认 room.stream_url(对齐 TS data.room ?? data)。
func (r reflowResponse) streamURL() json.RawMessage {
	if r.Data.Room != nil {
		return r.Data.Room.StreamURL
	}
	return r.Data.StreamURL
}

// reflowPullURLs reflow 的 stream_url:flv_pull_url 与 hls_pull_url_map/hls_pull_url。
type reflowPullURLs struct {
	FlvPullURL    map[string]any `json:"flv_pull_url"`
	HlsPullURLMap map[string]any `json:"hls_pull_url_map"`
	HlsPullURL    map[string]any `json:"hls_pull_url"`
}

// room 取房间对象:data.data[0] 优先,否则 data.room。
func (r enterResponse) room() enterRoom {
	if len(r.Data.Data) > 0 {
		return r.Data.Data[0]
	}
	if r.Data.Room != nil {
		return *r.Data.Room
	}
	return enterRoom{}
}

// streamInfo enter 的 stream_url:SDK(stream_data + qualities)与
// flv/hls_pull_url 两种形态共存。
type streamInfo struct {
	LiveCoreSDKData struct {
		PullData struct {
			StreamData string `json:"stream_data"`
			Options    struct {
				Qualities []qualityItem `json:"qualities"`
			} `json:"options"`
		} `json:"pull_data"`
	} `json:"live_core_sdk_data"`
	FlvPullURL    json.RawMessage `json:"flv_pull_url"`
	HlsPullURLMap json.RawMessage `json:"hls_pull_url_map"`
}

type qualityItem struct {
	Level  any `json:"level"`
	Name   any `json:"name"`
	SDKKey any `json:"sdk_key"`
}

type sdkStreamData struct {
	Data map[string]struct {
		Main sdkMain `json:"main"`
	} `json:"data"`
}

type sdkMain struct {
	Flv any `json:"flv"`
	Hls any `json:"hls"`
}
