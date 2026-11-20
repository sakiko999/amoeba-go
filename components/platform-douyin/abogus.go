// ABOGUS 签名脚本:从 TS 源 lib/abogus.ts 的 ABOGUS_JS 字面量字节精确提取
// (提取脚本见 tmp/port-specs/douyin.md C.3;chars=17241/bytes=17537),经
// embed 指令注入 jsrunner,禁止手改。

package douyin

import _ "embed"

//go:embed abogus.js
var abogusJS string
