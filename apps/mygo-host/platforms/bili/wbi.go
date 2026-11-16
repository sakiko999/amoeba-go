// wbi 签名（web 端）与直播端签名（signLiveParams）：
//   - signWeb：URLSearchParams sort → md5(串&wts+key)，输出 `...&w_rid&wts`
//   - signLiveParams：record sort → strip `!'()*` → encodeURIComponent join →
//     md5(query+key)，wts 参与排序、无独立 wts
// key 取自 nav 的 wbi_img（按 cookie 分 key，cookie 未传时回落 credentials provider）。

package bili

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
)

// mixinKeyEncTab 混排表（与 TS 完全一致）。
var mixinKeyEncTab = []int{
	46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35, 27, 43, 5, 49, 33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41,
	13, 37, 48, 7, 16, 24, 55, 40, 61, 26, 17, 0, 1, 60, 51, 30, 4, 22, 25, 54, 21, 56, 59, 6, 63, 57, 62, 11, 36, 20, 34,
	44, 52,
}

func (c *client) mixinKey(ctx context.Context, cookie string) (string, error) {
	data, err := c.nav(ctx, cookie)
	if err != nil {
		return "", err
	}
	imgKey, subKey := "", ""
	if data.Data != nil && data.Data.WbiImg != nil {
		imgKey = fileStem(deref(data.Data.WbiImg.ImgURL))
		subKey = fileStem(deref(data.Data.WbiImg.SubURL))
	}
	if imgKey == "" || subKey == "" {
		return "", fmt.Errorf("bilibili nav returned no wbi_img (signing unavailable)")
	}
	origin := imgKey + subKey
	var b strings.Builder
	for _, i := range mixinKeyEncTab {
		if i < len(origin) {
			b.WriteByte(origin[i])
		}
	}
	key := b.String()
	if len(key) > 32 {
		key = key[:32]
	}
	return key, nil
}

// signWeb web 端签名（返回值保留原始 query 串，签名覆盖排序编码后的参数）。
func (c *client) signWeb(ctx context.Context, query string, cookieOpt *string) (string, error) {
	key, err := c.mixinKey(ctx, c.resolveCookie(cookieOpt))
	if err != nil {
		return "", err
	}
	wts := strconv.FormatInt(c.now()/1000, 10)
	wRid := md5Hex(sortedFormEncode(parseFormPairs(query)) + "&wts=" + wts + key)
	sep := ""
	if query != "" {
		sep = "&"
	}
	return query + sep + "w_rid=" + wRid + "&wts=" + wts, nil
}

// spaceFpQuery space 系接口（x/space/wbi/*）的 dm_* 客户端指纹参数
// （只校验形态，不要求真实 WebGL；必须参与签名）。
func (c *client) spaceFpQuery() string {
	x := 1000 + rand.Intn(500)
	y := 1000 + rand.Intn(500)
	list := fmt.Sprintf(`[{"x":%d,"y":%d,"z":0,"timestamp":%d,"type":0}]`, 3*x+2*y, 4*x-5*y, 1+rand.Intn(60))
	inter := fmt.Sprintf(`{"ds":[],"wh":[%d,%d,0],"of":[0,0,0]}`, x, y)
	return encodePairs([][2]string{
		{"dm_img_list", list},
		{"dm_img_str", "bm8gd2ViZ2w"},
		{"dm_cover_img_str", "bm8gd2ViZ2w"},
		{"dm_img_inter", inter},
	})
}

// signSpace space 系 wbi 签名（在 signWeb 基础上补 dm_* 指纹）。
func (c *client) signSpace(ctx context.Context, query string, cookieOpt *string) (string, error) {
	return c.signWeb(ctx, query+"&"+c.spaceFpQuery(), cookieOpt)
}

// signLiveParams 直播端签名（wts 内联参与排序，无独立 wts 参数）。
func (c *client) signLiveParams(ctx context.Context, params map[string]string, cookieOpt *string) (string, error) {
	key, err := c.mixinKey(ctx, c.resolveCookie(cookieOpt))
	if err != nil {
		return "", err
	}
	all := make(map[string]string, len(params)+1)
	for k, v := range params {
		all[k] = v
	}
	all["wts"] = strconv.FormatInt(c.now()/1000, 10)
	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + encodeURIComponent(stripLiveChars(all[k]))
	}
	query := strings.Join(parts, "&")
	return query + "&w_rid=" + md5Hex(query+key), nil
}

func stripLiveChars(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '!', '\'', '(', ')', '*':
			return -1
		}
		return r
	}, s)
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// formPair 一份 URLSearchParams 条目（解码后的 name/value）。
type formPair struct {
	name  string
	value string
}

// parseFormPairs 按 URLSearchParams 语义解析 query（'+'→空格，%XX 解码）。
func parseFormPairs(query string) []formPair {
	var out []formPair
	for _, part := range strings.Split(query, "&") {
		if part == "" {
			continue
		}
		name, value := part, ""
		if i := strings.IndexByte(part, '='); i >= 0 {
			name, value = part[:i], part[i+1:]
		}
		out = append(out, formPair{decodeFormComponent(name), decodeFormComponent(value)})
	}
	return out
}

// sortedFormEncode 排序后按 application/x-www-form-urlencoded 序列化
// （URLSearchParams.sort + toString；实现见 encodeFormComponent）。
func sortedFormEncode(pairs []formPair) string {
	sorted := append([]formPair(nil), pairs...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].name < sorted[j].name })
	parts := make([]string, len(sorted))
	for i, p := range sorted {
		parts[i] = encodeFormComponent(p.name) + "=" + encodeFormComponent(p.value)
	}
	return strings.Join(parts, "&")
}

// encodePairs 按插入顺序序列化键值对（对应 new URLSearchParams(obj).toString()）。
func encodePairs(pairs [][2]string) string {
	parts := make([]string, len(pairs))
	for i, p := range pairs {
		parts[i] = encodeFormComponent(p[0]) + "=" + encodeFormComponent(p[1])
	}
	return strings.Join(parts, "&")
}

// encodeFormComponent 对齐 URLSearchParams 的编码集合（保留 * - . _，空格→+）。
func encodeFormComponent(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch >= '0' && ch <= '9',
			ch == '*', ch == '-', ch == '.', ch == '_':
			b.WriteByte(ch)
		case ch == ' ':
			b.WriteByte('+')
		default:
			fmt.Fprintf(&b, "%%%02X", ch)
		}
	}
	return b.String()
}

// decodeFormComponent '+'→空格 + %XX 解码（非法序列原样保留）。
func decodeFormComponent(s string) string {
	s = strings.ReplaceAll(s, "+", " ")
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			hi, ok1 := unhex(s[i+1])
			lo, ok2 := unhex(s[i+2])
			if ok1 && ok2 {
				b.WriteByte(hi<<4 | lo)
				i += 2
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func unhex(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

// encodeURIComponent 对齐 JS encodeURIComponent（非 ASCII 按 UTF-8 字节百分号编码）。
func encodeURIComponent(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' ||
			ch == '-' || ch == '_' || ch == '.' || ch == '!' || ch == '~' || ch == '*' ||
			ch == '\'' || ch == '(' || ch == ')' {
			b.WriteByte(ch)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", ch)
	}
	return b.String()
}
