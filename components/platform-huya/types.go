// 虎牙上游响应结构(字段全可选:缺字段由映射层兜底)。
// 值域用 flexStr/flexNum/flexBool 吸收上游类型漂移(同一字段可能给字符串或
// 数字),语义对齐 TS 侧的 String()/Number()/truthy 判断。

package huya

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

// flexStr 任意标量 → string(对齐 TS String(v ?? ""))。
type flexStr string

func (s *flexStr) UnmarshalJSON(raw []byte) error {
	v, ok := decodeScalar(raw)
	if !ok {
		*s = ""
		return nil
	}
	*s = flexStr(v)
	return nil
}

// flexNum 任意标量 → float64(对齐 TS Number(v ?? 0))。
type flexNum float64

func (n *flexNum) UnmarshalJSON(raw []byte) error {
	v, ok := decodeScalar(raw)
	if !ok {
		*n = 0
		return nil
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		*n = 0
		return nil
	}
	*n = flexNum(parsed)
	return nil
}

// flexBool 对齐 TS `v === true || String(v) === "true"`。
type flexBool bool

func (b *flexBool) UnmarshalJSON(raw []byte) error {
	v, ok := decodeScalar(raw)
	*b = flexBool(ok && v == "true")
	return nil
}

// decodeScalar 把 JSON 标量归一成字符串;数组/对象视为不可用。
func decodeScalar(raw []byte) (string, bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return "", false
	}
	switch x := v.(type) {
	case string:
		return x, true
	case json.Number:
		return x.String(), true
	case bool:
		return strconv.FormatBool(x), true
	default:
		return "", false
	}
}

// liveListResponse cache.php LiveList(热门 / 分区直播列表)。
type liveListResponse struct {
	Data *liveListData `json:"data"`
}

type liveListData struct {
	Datas []liveRow `json:"datas"`
}

// liveRow 列表行(热门与分区字段同构)。
type liveRow struct {
	ProfileRoom  *flexStr `json:"profileRoom"`
	Introduction *flexStr `json:"introduction"`
	RoomName     *flexStr `json:"roomName"`
	Screenshot   *flexStr `json:"screenshot"`
	Nick         *flexStr `json:"nick"`
	TotalCount   *flexNum `json:"totalCount"`
}

// profileRoomResponse mp.huya.com cache.php Live(房间元数据 + 拉流)。
type profileRoomResponse struct {
	Status *flexNum         `json:"status"`
	Data   *profileRoomData `json:"data"`
}

type profileRoomData struct {
	LiveStatus *flexStr       `json:"liveStatus"`
	LiveData   *profileLive   `json:"liveData"`
	Stream     *profileStream `json:"stream"`
}

type profileLive struct {
	ProfileRoom  *flexStr     `json:"profileRoom"`
	RoomName     *flexStr     `json:"roomName"`
	Introduction *flexStr     `json:"introduction"`
	Screenshot   *flexStr     `json:"screenshot"`
	Nick         *flexStr     `json:"nick"`
	TotalCount   *flexNum     `json:"totalCount"`
	BitRateInfo  *bitRateList `json:"bitRateInfo"`
}

// bitRateList 兼容 bitRateInfo 的两种形态:字符串化 JSON 或数组。
type bitRateList []bitRateRow

func (l *bitRateList) UnmarshalJSON(raw []byte) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var text string
		if json.Unmarshal(trimmed, &text) != nil {
			return nil
		}
		var rows []bitRateRow
		if json.Unmarshal([]byte(text), &rows) == nil {
			*l = rows
		}
		return nil
	}
	var rows []bitRateRow
	if json.Unmarshal(trimmed, &rows) == nil {
		*l = rows
	}
	return nil
}

type bitRateRow struct {
	Name    *flexStr `json:"sDisplayName"`
	BitRate *flexNum `json:"iBitRate"`
}

type profileStream struct {
	Flv *streamGroup `json:"flv"`
	Hls *streamGroup `json:"hls"`
}

type streamGroup struct {
	MultiLine []streamLine `json:"multiLine"`
}

type streamLine struct {
	CdnType *flexStr `json:"cdnType"`
	URL     *flexStr `json:"url"`
}

// bussLiveResponse 二级分区目录。
type bussLiveResponse struct {
	Data []subCategoryRow `json:"data"`
}

type subCategoryRow struct {
	Gid          *flexStr `json:"gid"`
	GameFullName *flexStr `json:"gameFullName"`
}

// searchResponse search.cdn.huya.com。房间桶 "3"、主播桶 "1" 是 {docs:[...]}
// 形态;其余桶(如 1026 视频桶)的值是裸数组等其他结构,若按 searchBucket
// 强解析会让整个响应 unmarshal 失败,故桶值保留原始 JSON、由 bucket 惰性解析。
type searchResponse struct {
	Response map[string]json.RawMessage `json:"response"`
}

type searchBucket struct {
	Docs []searchRow `json:"docs"`
}

// bucket 取指定桶的行;桶形态不是 {docs} 对象视为空桶(非预期桶不参与解析)。
func (r *searchResponse) bucket(key string) []searchRow {
	if r == nil {
		return nil
	}
	raw := r.Response[key]
	if len(raw) == 0 {
		return nil
	}
	var b searchBucket
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil
	}
	return b.Docs
}

// searchRow 房间桶与主播桶共用字段集。
type searchRow struct {
	RoomID           *flexStr  `json:"room_id"`
	UID              *flexStr  `json:"uid"`
	YYID             *flexStr  `json:"yyid"`
	GameIntroduction *flexStr  `json:"game_introduction"`
	GameRoomName     *flexStr  `json:"game_roomName"`
	GameScreenshot   *flexStr  `json:"game_screenshot"`
	GameNick         *flexStr  `json:"game_nick"`
	GameAvatarURL    *flexStr  `json:"game_avatarUrl180"`
	GameTotalCount   *flexNum  `json:"game_total_count"`
	GameLiveOn       *flexBool `json:"gameLiveOn"`
}
