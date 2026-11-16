// bili 渠道清单：顺序与 TS impl/register.ts 的 biliChannels 完全一致（16 个）。
// 单端点声明式渠道用 platform.MakeChannel；dynamic 的 offset 字符串游标与
// live 的多房间聚合需要自定义 DataSource（见各自文件）。

package bili

import (
	"encoding/json"

	"mygo-host/platform"
)

// channels 返回 bili 全部渠道实例。
func channels(c *client) []platform.DataChannel {
	return []platform.DataChannel{
		squareChannel(c),
		popularChannel(c),
		preciousChannel(c),
		rankingChannel(c),
		weeklyChannel(c),
		userVideoChannel(c),
		liveChannel(c),
		liveHotChannel(c),
		liveAreasChannel(c),
		liveAreaChannel(c),
		seasonChannel(c),
		seriesChannel(c),
		favlistChannel(c),
		&dynamicChannel{c: c},
		anchorChannel(c),
		userChannel(c),
	}
}

// requiredField 必填参数（JSON 输出 required:true）。
func requiredField(key, label string) platform.SourceInfoField {
	return platform.SourceInfoField{Key: key, Label: label, Required: platform.Bool(true)}
}

// optionalField 显式非必填（TS 声明了 required:false，JSON 输出 required:false）。
func optionalField(key, label string) platform.SourceInfoField {
	return platform.SourceInfoField{Key: key, Label: label, Required: platform.Bool(false)}
}

// rawJSON 上游对象原样透传到 Item.Raw（序列化失败则丢弃）。
func rawJSON(v any) platform.RawJSON {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return raw
}
