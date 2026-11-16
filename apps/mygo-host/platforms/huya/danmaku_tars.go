// 虎牙弹幕帧解析(专用结构,非通用反射):外层 Tars tag0=type,tag1=HYPushMessage;
// uri==1400 → HYMessage{sender.nickName, content, bulletFormat.fontColor};
// uri==8006(在线人数)等其余帧忽略。

package huya

import (
	"fmt"

	"mygo-host/platform"
)

// decodeDanmakuFrame 解析一帧;非弹幕帧或空正文返回 nil。
func decodeDanmakuFrame(data []byte) []platform.DanmakuItem {
	outer := &tarsReader{buf: data}
	if outer.readInt(0) != 7 {
		return nil
	}
	push := &tarsReader{buf: outer.readBytes(1)}
	if push.readInt(1) != 1400 {
		return nil
	}
	message := &tarsReader{buf: push.readBytes(2)}
	user := ""
	if sender := message.readStructAt(0); sender != nil {
		user = sender.readString(2)
	}
	text := message.readString(3)
	var color *string
	if bullet := message.readStructAt(6); bullet != nil {
		if fontColor := bullet.readInt(0); fontColor > 0 {
			color = platform.Str(rgbToHex(fontColor))
		}
	}
	if text == "" {
		return nil
	}
	return []platform.DanmakuItem{{Text: text, User: platform.Str(user), Color: color}}
}

// rgbToHex 0xRRGGBB → #RRGGBB(对齐 platform-kit rgbToHex)。
func rgbToHex(rgb int) string {
	return fmt.Sprintf("#%06x", rgb&0xffffff)
}
