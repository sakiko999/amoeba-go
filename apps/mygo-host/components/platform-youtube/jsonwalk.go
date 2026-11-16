// 有序 JSON 树:InnerTube 响应里列表顺序与键顺序有语义(频道视频序、首个
// continuation 令牌),encoding/json 的 map 会丢顺序,故用 Decoder 逐 token 建
// 有序树。只覆盖 youtube 解析所需的只读能力,缺失/类型不符一律给空值。
// nullish 语义对齐 JS:显式 null 视同缺失(get 折叠),空数组与缺失区分
// (items 返回非 nil 空切片,供调用方做 ?? 式回退判断)。

package youtube

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type jsonKind uint8

const (
	jsonNull jsonKind = iota
	jsonBool
	jsonNumber
	jsonString
	jsonArray
	jsonObject
)

type jsonValue struct {
	kind jsonKind
	str  string
	num  float64
	flag bool
	arr  []*jsonValue
	obj  []jsonPair
}

type jsonPair struct {
	key string
	val *jsonValue
}

func parseJSONValue(raw []byte) (*jsonValue, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	value, err := readJSONValue(dec)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func readJSONValue(dec *json.Decoder) (*jsonValue, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			value := &jsonValue{kind: jsonObject}
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, _ := keyTok.(string)
				val, err := readJSONValue(dec)
				if err != nil {
					return nil, err
				}
				value.obj = append(value.obj, jsonPair{key: key, val: val})
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return value, nil
		case '[':
			value := &jsonValue{kind: jsonArray}
			for dec.More() {
				item, err := readJSONValue(dec)
				if err != nil {
					return nil, err
				}
				value.arr = append(value.arr, item)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return value, nil
		}
		return nil, fmt.Errorf("json: 意外分隔符 %v", t)
	case string:
		return &jsonValue{kind: jsonString, str: t}, nil
	case json.Number:
		number, _ := t.Float64()
		return &jsonValue{kind: jsonNumber, num: number}, nil
	case bool:
		return &jsonValue{kind: jsonBool, flag: t}, nil
	case nil:
		return &jsonValue{kind: jsonNull}, nil
	default:
		return nil, fmt.Errorf("json: 意外 token %T", tok)
	}
}

func (v *jsonValue) get(key string) *jsonValue {
	if v == nil || v.kind != jsonObject {
		return nil
	}
	for _, pair := range v.obj {
		if pair.key == key {
			// TS `?.`/真值判断下 JSON null 等同键缺失,折叠为 nil
			// (否则 `!= nil` 判断会把 `{key: null}` 当存在)。
			if pair.val != nil && pair.val.kind == jsonNull {
				return nil
			}
			return pair.val
		}
	}
	return nil
}

func (v *jsonValue) path(keys ...string) *jsonValue {
	for _, key := range keys {
		v = v.get(key)
		if v == nil {
			return nil
		}
	}
	return v
}

func (v *jsonValue) items() []*jsonValue {
	if v == nil || v.kind != jsonArray {
		return nil
	}
	if v.arr == nil {
		// Array.isArray([]) 为真:空数组返回非 nil 空切片,与缺失/非数组
		// (nil)区分——调用方的 `== nil` 回退对应 TS `??`,空数组不回退。
		return []*jsonValue{}
	}
	return v.arr
}

func (v *jsonValue) firstItem() *jsonValue {
	items := v.items()
	if len(items) == 0 {
		return nil
	}
	return items[0]
}

// asText 只在字符串类型取值(TS typeof x === "string")。
func (v *jsonValue) asText() (string, bool) {
	if v == nil || v.kind != jsonString {
		return "", false
	}
	return v.str, true
}

// asJSString String(value ?? "") 语义:数字/布尔也转换,其余空串。
func (v *jsonValue) asJSString() string {
	if v == nil {
		return ""
	}
	switch v.kind {
	case jsonString:
		return v.str
	case jsonNumber:
		return strconv.FormatFloat(v.num, 'f', -1, 64)
	case jsonBool:
		if v.flag {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

// asNumber Number(v) 语义:数字直取,数字字符串转换,其余 0。
func (v *jsonValue) asNumber() float64 {
	if v == nil {
		return 0
	}
	switch v.kind {
	case jsonNumber:
		return v.num
	case jsonString:
		number, err := strconv.ParseFloat(strings.TrimSpace(v.str), 64)
		if err != nil {
			return 0
		}
		return number
	default:
		return 0
	}
}

func (v *jsonValue) truthy() bool {
	if v == nil {
		return false
	}
	switch v.kind {
	case jsonNull:
		return false
	case jsonBool:
		return v.flag
	case jsonNumber:
		return v.num != 0
	case jsonString:
		return v.str != ""
	default:
		return true
	}
}

// walkJSON 按文档顺序前序遍历(visit 对每个值调用一次)。
func walkJSON(v *jsonValue, visit func(*jsonValue)) {
	if v == nil {
		return
	}
	visit(v)
	switch v.kind {
	case jsonObject:
		for _, pair := range v.obj {
			walkJSON(pair.val, visit)
		}
	case jsonArray:
		for _, item := range v.arr {
			walkJSON(item, visit)
		}
	}
}

// ytText InnerTube 文本节点:simpleText 优先,runs 拼接兜底(client/discover 同款)。
func ytText(node *jsonValue) (string, bool) {
	if node == nil {
		return "", false
	}
	if text, ok := node.get("simpleText").asText(); ok {
		return text, true
	}
	runs := node.get("runs").items()
	if runs == nil {
		return "", false
	}
	var b strings.Builder
	for _, run := range runs {
		if text, ok := run.get("text").asText(); ok {
			b.WriteString(text)
		}
	}
	joined := b.String()
	return joined, joined != ""
}

// channelText 频道页文本节点:simpleText / 新版 content / runs 拼接。
func channelText(node *jsonValue) (string, bool) {
	if node == nil {
		return "", false
	}
	if text, ok := node.get("simpleText").asText(); ok {
		return text, true
	}
	if text, ok := node.get("content").asText(); ok {
		return text, true
	}
	runs := node.get("runs").items()
	if runs == nil {
		return "", false
	}
	var b strings.Builder
	for _, run := range runs {
		if text, ok := run.get("text").asText(); ok {
			b.WriteString(text)
		}
	}
	joined := b.String()
	return joined, joined != ""
}

// ytThumb 缩略图节点:thumbnails 里取 width 最大的一张(client/discover)。
func ytThumb(node *jsonValue) string {
	return pickMaxThumb(node.get("thumbnails").items())
}

// channelThumb 频道页缩略图:sources 优先,其次 thumbnails。
func channelThumb(node *jsonValue) string {
	if sources := node.get("sources").items(); sources != nil {
		return pickMaxThumb(sources)
	}
	return pickMaxThumb(node.get("thumbnails").items())
}

func pickMaxThumb(list []*jsonValue) string {
	if len(list) == 0 {
		return ""
	}
	best := list[0]
	for _, item := range list[1:] {
		if item.get("width").asNumber() > best.get("width").asNumber() {
			best = item
		}
	}
	url, _ := best.get("url").asText()
	return url
}
