// 登录配方与密钥声明的装配面：各平台组件自带配方，宿主只汇总，
// service-credentials 不感知具体平台。

package services

import (
	"mygo-host/components/platform-bangumi"
	"mygo-host/components/platform-bili"
	"mygo-host/components/platform-douban"
	"mygo-host/components/platform-douyin"
	"mygo-host/components/platform-tmdb"
	"mygo-host/components/platform-weibo"
	"mygo-host/components/platform-xhs"
	"mygo-host/platform"
	"mygo-host/platform/login"
)

// LoginFlows 全部平台的扫码登录配方（顺序即页面卡片顺序）。
func LoginFlows() []login.Flow {
	return []login.Flow{
		weibo.LoginFlow(),
		xhs.LoginFlow(),
		douyin.LoginFlow(),
		douban.LoginFlow(),
		bili.LoginFlow(),
	}
}

// KeySpecs 全部平台声明的 API 密钥。
func KeySpecs() []platform.KeySpec {
	return []platform.KeySpec{
		{Name: tmdb.KeyName, Label: tmdb.KeyLabel},
		{Name: bangumi.KeyName, Label: bangumi.KeyLabel},
	}
}
