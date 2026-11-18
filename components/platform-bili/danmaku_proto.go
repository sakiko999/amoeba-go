// 手写 protobuf wire 解码（仅 B 站 seg.so 视频弹幕需要）：
// DanmakuElem 字段号（实测 hex 分解确认）：progress=2 / mode=3 / color=5 / content=7。
// 守护点：截断/畸形帧必须返回而非死循环（长度声明超出缓冲时 clamp + 未前进即 break）。

package bili

import "strconv"

// danmakuElem 一条视频弹幕。
type danmakuElem struct {
	progress int // 出现位置，毫秒
	mode     int // 1/2/3 滚动，4 底，5 顶，6 逆向，7 高级
	color    int // 十进制 ARGB
	content  string
}

type protoReader struct {
	buf []byte
	p   int
}

func (r *protoReader) varint() uint64 {
	var v uint64
	var shift uint
	for r.p < len(r.buf) {
		b := r.buf[r.p]
		r.p++
		v |= uint64(b&0x7f) << shift
		if b&0x80 == 0 {
			return v
		}
		shift += 7
	}
	return v
}

func (r *protoReader) skip(wire int) {
	switch wire {
	case 0:
		r.varint()
	case 1:
		r.p += 8
	case 2:
		r.p += int(r.varint())
	case 5:
		r.p += 4
	}
}

// decodeDanmakuSeg 解析 seg.so 的 DmSegMobileReply，返回弹幕列表。
func decodeDanmakuSeg(buf []byte) []danmakuElem {
	out := []danmakuElem{}
	r := &protoReader{buf: buf}
	for r.p < len(buf) {
		before := r.p
		tag := r.varint()
		field, wire := int(tag>>3), int(tag&7)
		if field == 1 && wire == 2 {
			end := r.p + int(r.varint())
			if end > len(buf) {
				end = len(buf)
			}
			if elem, ok := r.decodeElem(end); ok {
				out = append(out, elem)
			}
		} else {
			r.skip(wire)
		}
		if r.p <= before {
			break
		}
	}
	return out
}

// decodeElem 解一条 DanmakuElem（[end] 为消息边界；任何未推进都立即停）。
func (r *protoReader) decodeElem(end int) (danmakuElem, bool) {
	progress, mode, color, content := 0, 1, 0xffffff, ""
	for r.p < end {
		before := r.p
		tag := r.varint()
		field, wire := int(tag>>3), int(tag&7)
		switch wire {
		case 0:
			v := r.varint()
			switch field {
			case 2:
				progress = int(v)
			case 3:
				mode = int(v)
			case 5:
				color = int(v)
			}
		case 2:
			length := int(r.varint())
			start, stop := r.p, r.p+length
			if stop > end || stop < start {
				stop = end
			}
			if field == 7 {
				content = string(r.buf[start:stop])
			}
			r.p = start + length
			if r.p < start {
				r.p = end
			}
		default:
			if wire == 1 {
				r.p += 8
			} else {
				r.p += 4
			}
		}
		if r.p <= before {
			break
		}
	}
	if content == "" {
		return danmakuElem{}, false
	}
	return danmakuElem{progress: progress, mode: mode, color: color, content: content}, true
}

// argbToHex 十进制 ARGB → #RRGGBB（4 位补 00、6 位原样，非法返回白色）。
func argbToHex(n int) string {
	if n < 0 {
		return "#ffffff"
	}
	hex := strconv.FormatInt(int64(n), 16)
	switch len(hex) {
	case 8:
		return "#" + hex[2:]
	case 4:
		return "#00" + hex
	case 6:
		return "#" + hex
	}
	return "#ffffff"
}
