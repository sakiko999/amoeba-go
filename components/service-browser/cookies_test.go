// cookie 读写与 DoAs 隔离:Storage.getCookies 归一到 BrowserCookie、按域名标签
// 边界删除(写入过期时间)、自定义 key 的 target 不与同 origin 的 Do 复用。

package browser

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"mygo-host/platform"
)

func TestAllCookiesAndClear(t *testing.T) {
	f := newFakeCDP(t)
	cookies := []map[string]any{
		{"name": "SUB", "value": "s1", "domain": ".weibo.com", "path": "/"},
		{"name": "user_id", "value": "u1", "domain": ".xiaohongshu.com", "path": "/"},
		{"name": "a1", "value": "u2", "domain": "www.xiaohongshu.com", "path": "/"},
		{"name": "fake", "value": "u3", "domain": "notxiaohongshu.com", "path": "/"},
	}
	f.handle("Storage.getCookies", func(c cdpCommand) {
		f.respond(c.ID, map[string]any{"cookies": cookies})
	})
	var setPayloads []struct {
		Name   string  `json:"name"`
		Domain string  `json:"domain"`
		Path   string  `json:"path"`
		Expiry float64 `json:"expires"`
	}
	f.handle("Storage.setCookies", func(c cdpCommand) {
		var p struct {
			Cookies []struct {
				Name    string  `json:"name"`
				Domain  string  `json:"domain"`
				Path    string  `json:"path"`
				Expires float64 `json:"expires"`
			} `json:"cookies"`
		}
		if err := json.Unmarshal(c.Params, &p); err != nil {
			t.Errorf("setCookies 参数: %v", err)
		}
		for _, ck := range p.Cookies {
			setPayloads = append(setPayloads, struct {
				Name   string  `json:"name"`
				Domain string  `json:"domain"`
				Path   string  `json:"path"`
				Expiry float64 `json:"expires"`
			}{ck.Name, ck.Domain, ck.Path, ck.Expires})
		}
		f.respond(c.ID, map[string]any{})
	})
	b := newFakeBridge(t, f)
	ctx := context.Background()

	all, err := b.AllCookies(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 4 {
		t.Fatalf("AllCookies = %d 条, want 4", len(all))
	}
	if all[0] != (platform.BrowserCookie{Name: "SUB", Value: "s1", Domain: ".weibo.com", Path: "/"}) {
		t.Fatalf("首条 cookie = %+v", all[0])
	}

	n, err := b.ClearCookies(ctx, []string{"xiaohongshu.com"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || len(setPayloads) != 2 {
		t.Fatalf("清除条数 = %d payload=%d, want 2/2", n, len(setPayloads))
	}
	for _, p := range setPayloads {
		if p.Expiry != 1 {
			t.Fatalf("删除未用过期时间: %+v", p)
		}
	}
}

func TestSetCookiesPersists(t *testing.T) {
	f := newFakeCDP(t)
	var got []map[string]any
	f.handle("Storage.setCookies", func(c cdpCommand) {
		var p struct {
			Cookies []map[string]any `json:"cookies"`
		}
		if err := json.Unmarshal(c.Params, &p); err != nil {
			t.Errorf("setCookies 参数: %v", err)
		}
		got = append(got, p.Cookies...)
		f.respond(c.ID, map[string]any{})
	})
	b := newFakeBridge(t, f)
	before := time.Now().Add(179 * 24 * time.Hour).Unix()

	err := b.SetCookies(context.Background(), []platform.BrowserCookie{
		{Name: "sessionid", Value: "s", Domain: ".douyin.com"},
		{Name: "SESSDATA", Value: "x", Domain: ".bilibili.com", Path: "/x"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("写入 cookie = %d 条, want 2", len(got))
	}
	if got[0]["path"] != "/" || got[1]["path"] != "/x" {
		t.Fatalf("path 归一错误: %v", got)
	}
	for _, c := range got {
		if int64(c["expires"].(float64)) <= before {
			t.Fatalf("回灌 cookie 不是持久 cookie: %v", c)
		}
	}
}

func TestDoAsIsolatesKeys(t *testing.T) {
	f := newFakeCDP(t)
	b := newFakeBridge(t, f)
	ctx := context.Background()

	if err := b.DoAs(ctx, "login:x", "https://example.com", func(platform.BrowserPage) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := b.Do(ctx, "https://example.com", func(platform.BrowserPage) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if f.targets != 2 {
		t.Fatalf("DoAs 与 Do 复用了 target: targets=%d", f.targets)
	}
	if err := b.DoAs(ctx, "login:x", "https://example.com", func(platform.BrowserPage) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if f.targets != 2 {
		t.Fatalf("DoAs 同 key 未复用: targets=%d", f.targets)
	}
	if err := b.CloseTabAs(ctx, "login:x"); err != nil {
		t.Fatal(err)
	}
	b.mu.Lock()
	_, still := b.tabs["login:x"]
	b.mu.Unlock()
	if still {
		t.Fatal("CloseTabAs 后 target 缓存未清理")
	}
}
