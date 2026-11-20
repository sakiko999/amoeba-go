// 平台 cookie provider(对齐 platform-kit/runtime/platform/credentials.ts):
// 调用点显式 cookie > 显式空串(强制匿名)> 宿主注册的平台默认(settings 里
// 扫码/手填的登录态)。文件落 <userData>/credentials.json。

package platform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Credentials 平台默认 cookie 存储与解析(并发安全)。
type Credentials struct {
	mu   sync.Mutex
	path string
	data map[string]string // platform id → cookie("k=v; k2=v2")
}

// NewCredentials 建存储;path 形如 <userData>/credentials.json。
func NewCredentials(path string) *Credentials {
	c := &Credentials{path: path, data: map[string]string{}}
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &c.data)
	}
	return c
}

// Lookup 平台默认 cookie(未注册/无值 → false)。
func (c *Credentials) Lookup(platformID string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.data[platformID]
	return v, ok
}

// Set 写平台默认 cookie 并持久化(空串 = 清除该平台)。
func (c *Credentials) Set(platformID, cookie string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cookie == "" {
		delete(c.data, platformID)
	} else {
		c.data[platformID] = cookie
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(c.data, "", "  ")
	if err != nil {
		return err
	}
	// 0600 在 Unix 上限制为当前用户读写;Windows 的 Go 实现不落 ACL,
	// 依赖 userData 目录本身的用户级权限。
	return os.WriteFile(c.path, raw, 0o600)
}

// ResolveCookie 统一入口:显式非空 > 显式空串(强制匿名,false)> 平台默认。
// 各平台 client 拿到显式 cookie 参数后先过这里,禁止手写 fallback。
// explicit 为 nil 表示调用点未传(TS undefined);&"" 强制匿名。
func (c *Credentials) ResolveCookie(explicit *string, platformID string) (string, bool) {
	if explicit != nil {
		if *explicit == "" {
			return "", false
		}
		return *explicit, true
	}
	return c.Lookup(platformID)
}
