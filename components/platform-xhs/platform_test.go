// 平台装配与 search op 面测试:namespace/label/tags、scope 校验、keyword 归一。

package xhs

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"mygo-host/platform"
)

func TestPlatformSpec(t *testing.T) {
	fake, page := newFakeBrowser()
	page.pageLogged = true
	page.state = twoUsersState
	spec := New(testContext(fake))

	if spec.Namespace != "xhs" || spec.Label != "小红书" {
		t.Errorf("namespace/label = %q/%q", spec.Namespace, spec.Label)
	}
	wantTags := []string{"platform.data", "search.entity"}
	if len(spec.Tags) != len(wantTags) {
		t.Fatalf("tags = %v, want %v", spec.Tags, wantTags)
	}
	for i, tag := range wantTags {
		if spec.Tags[i] != tag {
			t.Errorf("tags[%d] = %q, want %q", i, spec.Tags[i], tag)
		}
	}
	if len(spec.Ops) != 1 {
		t.Fatalf("ops = %v, want 仅 search", spec.Ops)
	}
	search, ok := spec.Ops["search"]
	if !ok {
		t.Fatal("缺少 search op")
	}
	if search.Description == "" || search.Invoke == nil {
		t.Errorf("search op 不完整: %+v", search)
	}
}

func TestSearchOpKeywordTrim(t *testing.T) {
	fake, page := newFakeBrowser()
	page.pageLogged = true
	page.state = twoUsersState
	ops := searchOps(testContext(fake))

	result, err := ops["search"].Invoke(context.Background(),
		json.RawMessage(`{"keyword":"  美食  ","scope":"entity","page":2,"pageSize":5}`))
	if err != nil {
		t.Fatal(err)
	}
	pageResult, ok := result.(platform.FetchPage)
	if !ok || len(pageResult.Items) != 2 {
		t.Fatalf("result = %T %+v", result, result)
	}
	if !strings.Contains(page.navCaptureURL, "keyword=%E7%BE%8E%E9%A3%9F&source=") {
		t.Errorf("navURL = %q, want 去掉首尾空格的关键词", page.navCaptureURL)
	}
}

func TestSearchOpValidation(t *testing.T) {
	cases := []struct {
		name   string
		params string
		want   string
	}{
		{"不支持的 scope", `{"keyword":"kw","scope":"live"}`, "search 不支持 scope"},
		{"缺少 scope", `{"keyword":"kw"}`, "search 不支持 scope"},
		{"空关键词", `{"keyword":"   ","scope":"entity"}`, "search 需要 keyword 参数"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake, _ := newFakeBrowser()
			_, err := searchOps(testContext(fake))["search"].Invoke(context.Background(),
				json.RawMessage(tc.params))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want 含 %q", err, tc.want)
			}
			if len(fake.calls) != 0 {
				t.Errorf("参数校验失败仍触发浏览器: %v", fake.calls)
			}
		})
	}
}
