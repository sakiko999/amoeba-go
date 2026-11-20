// searchOps 契约测试:参数校验、默认值、Items 非 nil、tags 与 MergeOps。

package platform

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestSearchOpsValidation(t *testing.T) {
	spec := SearchSpec{
		Scopes: []SearchScope{SearchLive, SearchEntity},
		Search: func(ctx context.Context, p SearchParams) (FetchPage, error) {
			return FetchPage{Items: []Item{{ID: "x", Title: "t", FetchedAt: 1}}}, nil
		},
	}
	op := SearchOps(spec)["search"]

	cases := []struct {
		name     string
		params   string
		contains string
	}{
		{"空 keyword", `{"keyword":"  ","scope":"live"}`, "keyword"},
		{"缺 scope", `{"keyword":"x"}`, "不支持 scope"},
		{"未知 scope", `{"keyword":"x","scope":"video"}`, "不支持 scope"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := op.Invoke(context.Background(), json.RawMessage(tc.params)); err == nil || !strings.Contains(err.Error(), tc.contains) {
				t.Errorf("err = %v, want 包含 %q", err, tc.contains)
			}
		})
	}
}

func TestSearchOpsDefaultsAndItems(t *testing.T) {
	var got SearchParams
	spec := SearchSpec{
		Scopes: []SearchScope{SearchEntity},
		Search: func(ctx context.Context, p SearchParams) (FetchPage, error) {
			got = p
			return FetchPage{}, nil // Items nil → 应归一为 []
		},
	}
	result, err := SearchOps(spec)["search"].Invoke(context.Background(), json.RawMessage(`{"keyword":" x ","scope":"entity"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Keyword != "x" || got.Page != 1 || got.PageSize != defaultSearchPageSize {
		t.Errorf("params = %+v, want keyword 去空格 + page 1 + pageSize %d", got, defaultSearchPageSize)
	}
	page := result.(FetchPage)
	if page.Items == nil || len(page.Items) != 0 {
		t.Errorf("items = %#v, want 非 nil 空切片", page.Items)
	}
	raw, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"items":[]`) {
		t.Errorf("json = %s, want items:[]", raw)
	}
}

func TestSearchTags(t *testing.T) {
	tags := SearchTags(SearchLive, SearchEntity)
	if len(tags) != 2 || tags[0] != "search.live" || tags[1] != "search.entity" {
		t.Errorf("tags = %v", tags)
	}
}

func TestMergeOps(t *testing.T) {
	base := Ops{"a": {Description: "a"}}
	extra := Ops{"b": {Description: "b"}, "a": {Description: "a2"}}
	merged := MergeOps(base, extra)
	if len(merged) != 2 || merged["a"].Description != "a2" || merged["b"].Description != "b" {
		t.Errorf("merged = %+v, want extra 覆盖同名", merged)
	}
}
