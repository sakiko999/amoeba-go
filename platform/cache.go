// provider 缓存(getJsonCached/postJsonCached 的 Go 等价):metadata 平台共用,
// TTL 7 天。TS 侧落 node:sqlite;Go 侧用文件存储(sha256(key) 为文件名),
// 语义等价(同 key 命中、TTL 过期丢弃),不引入 sqlite 依赖。

package platform

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const cacheTTL = 7 * 24 * time.Hour

// Cache provider 响应缓存(并发安全)。
type Cache struct {
	dir string
}

// NewCache 建缓存;dir 形如 <userData>/provider-cache。
func NewCache(dir string) (*Cache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Cache{dir: dir}, nil
}

type cacheEntry struct {
	Value     string `json:"value"`
	CreatedAt int64  `json:"created_at"`
}

func (c *Cache) path(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(c.dir, hex.EncodeToString(sum[:])+".json")
}

func (c *Cache) get(key string) (string, bool) {
	data, err := os.ReadFile(c.path(key))
	if err != nil {
		return "", false
	}
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return "", false
	}
	if time.Now().UnixMilli()-entry.CreatedAt > cacheTTL.Milliseconds() {
		return "", false
	}
	return entry.Value, true
}

func (c *Cache) set(key, value string) {
	entry := cacheEntry{Value: value, CreatedAt: time.Now().UnixMilli()}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_ = os.WriteFile(c.path(key), data, 0o644)
}

// CacheKey 缓存 key:parts 用 "/" 连接(与 TS cacheKey 一致)。
func CacheKey(parts []string) string { return strings.Join(parts, "/") }

func (c *Cache) cached(ctx context.Context, source string, key string, fetch func() (string, error)) (RawJSON, error) {
	if v, ok := c.get(key); ok {
		return RawJSON(v), nil
	}
	text, err := fetch()
	if err != nil {
		return nil, err
	}
	c.set(key, text)
	return RawJSON(text), nil
}

// providerRequest 对齐 TS provider/http.ts 的 request:404 → NotFoundError,
// 非 2xx → ProviderError(body 截 200 字)。
func (c *Cache) providerRequest(ctx context.Context, client *HostClient, source, url string, req Request) (string, error) {
	res, err := client.Do(ctx, req)
	if err != nil {
		return "", err
	}
	if res.Status == 404 {
		return "", &NotFoundError{Message: fmt.Sprintf("%s 中不存在该条目:%s", source, url)}
	}
	if res.Status < 200 || res.Status >= 300 {
		body := res.Body
		if len(body) > 200 {
			body = body[:200]
		}
		return "", &ProviderError{Source: source, Message: fmt.Sprintf("HTTP %d: %s", res.Status, body)}
	}
	return res.Body, nil
}

// GetJSONCached GET JSON(带缓存)。headers 前会补默认 UA + Accept: application/json
// (对齐 TS provider 基座)。
func (c *Cache) GetJSONCached(ctx context.Context, client *HostClient, source string, keyParts []string, url string, headers map[string]string) (RawJSON, error) {
	return c.cached(ctx, source, CacheKey(keyParts), func() (string, error) {
		merged := map[string]string{"Accept": "application/json"}
		maps.Copy(merged, headers)
		return c.providerRequest(ctx, client, source, url, Request{URL: url, Headers: merged})
	})
}

// PostJSONCached POST JSON body(带缓存;Bangumi v0 搜索是 POST)。headers
// 与 Content-Type/Accept 合并(如可选的 Authorization)。
func (c *Cache) PostJSONCached(ctx context.Context, client *HostClient, source string, keyParts []string,
	url string, body any, headers map[string]string) (RawJSON, error) {
	return c.cached(ctx, source, CacheKey(keyParts), func() (string, error) {
		encoded, err := json.Marshal(body)
		if err != nil {
			return "", err
		}
		merged := map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
		}
		maps.Copy(merged, headers)
		req := Request{URL: url, Method: "POST", Body: string(encoded), Headers: merged}
		return c.providerRequest(ctx, client, source, url, req)
	})
}
