// Tars 二进制编解码(tag<<4|type,非 protobuf):逐行复刻 danmaku-tars.ts 子集。
// 整数按值域选字节宽度(0 用 ZERO_TAG);字符串 STRING1/STRING4;字节数组
// SIMPLE_LIST;struct = BEGIN + 字段 + END。
// 解码侧必须按缓冲长度为界退避:截断帧不得让 skipToStructEnd 变成同步死循环。

package huya

const (
	tarsByte = iota
	tarsShort
	tarsInt
	tarsLong
	tarsFloat
	tarsDouble
	tarsString1
	tarsString4
	tarsMap
	tarsList
	tarsStructBegin
	tarsStructEnd
	tarsZeroTag
	tarsSimpleList
)

// ── 编码(TarsOutputStream) ──────────────────────────────────────────────

type tarsWriter struct{ out []byte }

func (w *tarsWriter) bytes() []byte { return w.out }

func (w *tarsWriter) head(typ, tag int) {
	if tag < 15 {
		w.out = append(w.out, byte(tag<<4|typ))
		return
	}
	w.out = append(w.out, byte(15<<4|typ), byte(tag))
}

func (w *tarsWriter) writeInt(n int64, tag int) {
	switch {
	case n == 0:
		w.head(tarsZeroTag, tag)
	case n >= -128 && n <= 127:
		w.head(tarsByte, tag)
		w.out = append(w.out, byte(n))
	case n >= -32768 && n <= 32767:
		w.head(tarsShort, tag)
		w.appendBE(n, 2)
	case n >= -2147483648 && n <= 2147483647:
		w.head(tarsInt, tag)
		w.appendBE(n, 4)
	default:
		w.head(tarsLong, tag)
		w.appendBE(n, 8)
	}
}

func (w *tarsWriter) writeBool(v bool, tag int) {
	if v {
		w.writeInt(1, tag)
		return
	}
	w.writeInt(0, tag)
}

func (w *tarsWriter) writeString(s string, tag int) {
	raw := []byte(s)
	if len(raw) <= 255 {
		w.head(tarsString1, tag)
		w.out = append(w.out, byte(len(raw)))
	} else {
		w.head(tarsString4, tag)
		w.appendBE(int64(len(raw)), 4)
	}
	w.out = append(w.out, raw...)
}

func (w *tarsWriter) writeBytes(raw []byte, tag int) {
	w.head(tarsSimpleList, tag)
	w.head(tarsByte, 0)
	w.writeInt(int64(len(raw)), 0)
	w.out = append(w.out, raw...)
}

func (w *tarsWriter) appendBE(n int64, width int) {
	u := uint64(n)
	for shift := (width - 1) * 8; shift >= 0; shift -= 8 {
		w.out = append(w.out, byte(u>>uint(shift)))
	}
}

// ── 解码(TarsInputStream) ────────────────────────────────────────────────

type tarsHead struct{ typ, tag int }

type tarsReader struct {
	buf []byte
	p   int
}

// nextByte 越界返回 0 且游标照常前进(对齐 JS buf[p++] 的 undefined 语义)。
func (r *tarsReader) nextByte() byte {
	if r.p < len(r.buf) {
		b := r.buf[r.p]
		r.p++
		return b
	}
	r.p++
	return 0
}

// readBE 读 length 字节大端整数,按 TS 的 `| 0` 语义截断到 int32。
func (r *tarsReader) readBE(length int) int {
	var v uint64
	for i := 0; i < length; i++ {
		v = v<<8 | uint64(r.nextByte())
	}
	return int(int32(uint32(v)))
}

func (r *tarsReader) head() tarsHead {
	b := r.nextByte()
	h := tarsHead{typ: int(b & 15), tag: int(b>>4) & 15}
	if h.tag == 15 {
		h.tag = int(r.nextByte())
	}
	return h
}

func (r *tarsReader) intValue(typ int) int {
	switch typ {
	case tarsByte:
		return r.readBE(1)
	case tarsShort:
		return r.readBE(2)
	case tarsInt:
		return r.readBE(4)
	case tarsLong:
		return r.readBE(8)
	default:
		return 0
	}
}

func (r *tarsReader) skipField(typ int) {
	switch typ {
	case tarsByte:
		r.p++
	case tarsShort:
		r.p += 2
	case tarsInt:
		r.p += 4
	case tarsLong:
		r.p += 8
	case tarsFloat:
		r.p += 4
	case tarsDouble:
		r.p += 8
	case tarsString1:
		old := r.p
		length := int(r.nextByte())
		r.p = old + 1 + length
	case tarsString4:
		old := r.p
		length := r.readBE(4)
		r.p = old + 4 + length
	case tarsSimpleList:
		r.head() // 内层 BYTE 头
		length := r.intValue(r.head().typ)
		r.p += length
	case tarsMap:
		size := r.intValue(r.head().typ)
		for i := 0; i < size*2; i++ {
			r.skipField(r.head().typ)
		}
	case tarsList:
		size := r.intValue(r.head().typ)
		for i := 0; i < size; i++ {
			r.skipField(r.head().typ)
		}
	case tarsStructBegin:
		r.skipToStructEnd()
	}
}

// skipToStructEnd 以缓冲长度为界(截断帧最多循环 len(buf) 轮);
// ZERO_TAG 是零负载字段,不许用「游标未前进就退出」的判据(会漏解析后续字段)。
func (r *tarsReader) skipToStructEnd() {
	for r.p < len(r.buf) {
		h := r.head()
		if h.typ == tarsStructEnd {
			return
		}
		r.skipField(h.typ)
	}
}

// skipToTag 定位到 tag 的字段 head(不消费);字段按 tag 升序编码。
func (r *tarsReader) skipToTag(tag int) bool {
	for r.p < len(r.buf) {
		save := r.p
		h := r.head()
		if h.typ == tarsStructEnd {
			r.p = save
			return false
		}
		if h.tag >= tag {
			r.p = save
			return h.tag == tag
		}
		r.skipField(h.typ)
	}
	return false
}

func (r *tarsReader) readInt(tag int) int {
	if !r.skipToTag(tag) {
		return 0
	}
	return r.intValue(r.head().typ)
}

func (r *tarsReader) readBool(tag int) bool { return r.readInt(tag) != 0 }

func (r *tarsReader) readString(tag int) string {
	if !r.skipToTag(tag) {
		return ""
	}
	h := r.head()
	var length int
	switch h.typ {
	case tarsString1:
		length = int(r.nextByte())
	case tarsString4:
		length = r.readBE(4)
	default:
		return ""
	}
	start := r.p
	out := sliceClamped(r.buf, start, start+length)
	r.p = start + length
	return string(out)
}

func (r *tarsReader) readBytes(tag int) []byte {
	if !r.skipToTag(tag) {
		return nil
	}
	if h := r.head(); h.typ != tarsSimpleList {
		return nil
	}
	r.head() // 内层 BYTE 头
	length := r.intValue(r.head().typ)
	start := r.p
	out := append([]byte(nil), sliceClamped(r.buf, start, start+length)...)
	r.p = start + length
	return out
}

// readStructAt 读 tag 处的 struct,返回子 reader(不含 BEGIN/END 标记)。
func (r *tarsReader) readStructAt(tag int) *tarsReader {
	if !r.skipToTag(tag) {
		return nil
	}
	start := r.p
	if h := r.head(); h.typ != tarsStructBegin {
		r.p = start
		return nil
	}
	bodyStart := r.p
	r.skipToStructEnd()
	body := append([]byte(nil), sliceClamped(r.buf, bodyStart, r.p-1)...)
	return &tarsReader{buf: body}
}

// sliceClamped 取 buf[start:end],越界按 JS subarray 语义收敛。
func sliceClamped(buf []byte, start, end int) []byte {
	if start > len(buf) {
		start = len(buf)
	}
	if end < start {
		end = start
	}
	if end > len(buf) {
		end = len(buf)
	}
	return buf[start:end]
}
