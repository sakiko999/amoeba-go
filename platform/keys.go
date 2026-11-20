// 密钥展示元数据：由平台组件声明自己的密钥项，凭据服务只做存取与脱敏。

package platform

// KeySpec 页面上一条密钥的展示信息。
type KeySpec struct {
	Name  string
	Label string
}
