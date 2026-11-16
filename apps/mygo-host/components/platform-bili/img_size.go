// 图片宽高解析（对齐 platform-kit utils/img-size）：Range 预取文件头后按格式解析，
// 供动态 channel 的瀑布流补全缺宽高的图。平台运行时没有该工具，局部移植；
// 防盗链 Referer 规则只保留 sinaimg（其余图床空 Referer 可读）。

package bili

import (
	"context"
	"regexp"
	"strings"
	"sync"

	"mygo-host/platform"
)

// imageSize 图片像素尺寸。
type imageSize struct {
	Width  int
	Height int
}

// headerBytes JPEG 的 SOF 段可能被 Exif 推到几百字节后，1024 稳妥。
const headerBytes = 1024

var sizeCache sync.Map // url → imageSize

// fillImageSizes 批量补缺宽高的图（失败静默，调用方按默认比例退化）。
func fillImageSizes(ctx context.Context, hc *platform.HostClient, images []platform.SocialImage) {
	var wg sync.WaitGroup
	for i := range images {
		if images[i].Width != nil && images[i].Height != nil && *images[i].Width != 0 && *images[i].Height != 0 {
			continue
		}
		wg.Add(1)
		go func(img *platform.SocialImage) {
			defer wg.Done()
			size := fetchImageSize(ctx, hc, img.URL)
			if size != nil {
				img.Width = platform.Int(size.Width)
				img.Height = platform.Int(size.Height)
			}
		}(&images[i])
	}
	wg.Wait()
}

func fetchImageSize(ctx context.Context, hc *platform.HostClient, url string) *imageSize {
	if hit, ok := sizeCache.Load(url); ok {
		size := hit.(imageSize)
		return &size
	}
	headers := map[string]string{"Range": "bytes=0-1023"}
	if referer := mediaReferrerFor(url); referer != "" {
		headers["Referer"] = referer
	}
	res, err := hc.Do(ctx, platform.Request{URL: url, Headers: headers})
	if err != nil || res.Body == "" {
		return nil
	}
	buf := []byte(res.Body)
	if len(buf) > headerBytes {
		buf = buf[:headerBytes]
	}
	size := parseImageSize(buf)
	if size != nil {
		sizeCache.Store(url, *size)
	}
	return size
}

var mediaHostRe = regexp.MustCompile(`^https?://([^/]+)`)

// mediaReferrerFor 图床防盗链 Referer（Range 请求也要带，否则 403 解析不出宽高）。
func mediaReferrerFor(url string) string {
	m := mediaHostRe.FindStringSubmatch(url)
	if len(m) == 2 && strings.Contains(m[1], "sinaimg.cn") {
		return "https://weibo.com/"
	}
	return ""
}

// parseImageSize 从文件头解析宽高；支持 PNG / JPEG / WebP / GIF，无法识别返回 nil。
func parseImageSize(buf []byte) *imageSize {
	if len(buf) < 8 {
		return nil
	}
	if hasPrefix(buf, 0, []byte{0x89, 'P', 'N', 'G'}) && hasPrefix(buf, 12, []byte("IHDR")) && len(buf) >= 24 {
		return &imageSize{Width: beU32(buf, 16), Height: beU32(buf, 20)}
	}
	if buf[0] == 0xff && buf[1] == 0xd8 {
		return parseJPEGSize(buf)
	}
	if hasPrefix(buf, 0, []byte("RIFF")) && hasPrefix(buf, 8, []byte("WEBP")) {
		return parseWebPSize(buf)
	}
	if hasPrefix(buf, 0, []byte("GIF8")) && len(buf) >= 10 {
		return &imageSize{Width: int(buf[6]) | int(buf[7])<<8, Height: int(buf[8]) | int(buf[9])<<8}
	}
	return nil
}

func parseJPEGSize(buf []byte) *imageSize {
	off := 2
	for off+9 < len(buf) {
		if buf[off] != 0xff {
			off++
			continue
		}
		marker := buf[off+1]
		if (marker >= 0xd0 && marker <= 0xd9) || marker == 0x01 {
			off += 2
			continue
		}
		segLen := int(buf[off+2])<<8 | int(buf[off+3])
		isSOF := marker >= 0xc0 && marker <= 0xcf && marker != 0xc4 && marker != 0xc8 && marker != 0xcc
		if isSOF && off+9 <= len(buf) {
			return &imageSize{
				Height: int(buf[off+5])<<8 | int(buf[off+6]),
				Width:  int(buf[off+7])<<8 | int(buf[off+8]),
			}
		}
		off += 2 + segLen
	}
	return nil
}

func parseWebPSize(buf []byte) *imageSize {
	switch {
	case hasPrefix(buf, 12, []byte("VP8 ")) && len(buf) >= 30:
		return &imageSize{Width: int(buf[26])<<8 | int(buf[25]), Height: int(buf[28])<<8 | int(buf[27])}
	case hasPrefix(buf, 12, []byte("VP8L")) && len(buf) >= 25:
		w := 1 + ((int(buf[21])&0x3f)<<8 | int(buf[20]))
		h := 1 + ((int(buf[23])&0x0f)<<8 | int(buf[22]))
		return &imageSize{Width: w, Height: h}
	case hasPrefix(buf, 12, []byte("VP8X")) && len(buf) >= 30:
		u24le := func(o int) int { return int(buf[o]) | int(buf[o+1])<<8 | int(buf[o+2])<<16 }
		return &imageSize{Width: 1 + u24le(24), Height: 1 + u24le(27)}
	}
	return nil
}

func hasPrefix(buf []byte, off int, prefix []byte) bool {
	if off+len(prefix) > len(buf) {
		return false
	}
	for i, b := range prefix {
		if buf[off+i] != b {
			return false
		}
	}
	return true
}

func beU32(buf []byte, off int) int {
	return int(buf[off])<<24 | int(buf[off+1])<<16 | int(buf[off+2])<<8 | int(buf[off+3])
}
