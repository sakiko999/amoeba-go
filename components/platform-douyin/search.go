// douyin 搜索 op(scope=entity):先按抖音号/主页链接/sec_uid 匿名查号
// (命中只返回单条),查不到再走关键词浏览器被动捕获。两条路径都不使用
// a_bogus(TS 原文结论:给用户搜索加签名无效且偏离原行为)。

package douyin

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"mygo-host/platform"
)

const searchSourceID = "douyin:user:search"

// errAccountNotFound 查无此人;只有它能回退关键词,网络/风控错误原样抛。
var errAccountNotFound = errors.New("找不到抖音号")

var (
	secUIDInURLRe   = regexp.MustCompile(`(?i)douyin\.com/user/(MS4wLjAB[\w-]+)`)
	bareSecUIDRe    = regexp.MustCompile(`^MS4wLjAB[\w-]+$`)
	accountPrefixRe = regexp.MustCompile(`^抖音号[：:]\s*`)
	whitespaceRe    = regexp.MustCompile(`\s`)
)

func searchOps(sc *platform.ServiceContext) platform.Ops {
	return platform.SearchOps(platform.SearchSpec{
		Scopes: []platform.SearchScope{platform.SearchEntity},
		Search: func(ctx context.Context, p platform.SearchParams) (platform.FetchPage, error) {
			items, err := searchUsers(ctx, sc, p.Keyword)
			return platform.FetchPage{Items: items}, err
		},
	})
}

// searchUsers 查号命中即返回单条,否则关键词捕获(需要登录门禁)。
func searchUsers(ctx context.Context, sc *platform.ServiceContext, keyword string) ([]platform.Item, error) {
	hit, err := lookupByAccount(ctx, sc, keyword)
	if err != nil {
		return nil, err
	}
	if hit != nil {
		return []platform.Item{userItem(*hit, sc.NowMs())}, nil
	}
	return searchByKeyword(ctx, sc, keyword)
}

// lookupByAccount 归一输入:主页链接/裸 sec_uid 直接命中(昵称取末 8 位),
// 含空白视为关键词;否则按抖音号查匿名资料接口。
func lookupByAccount(ctx context.Context, sc *platform.ServiceContext, raw string) (*userHit, error) {
	s := strings.TrimLeft(raw, "@")
	s = accountPrefixRe.ReplaceAllString(s, "")
	if m := secUIDInURLRe.FindStringSubmatch(s); len(m) >= 2 {
		return &userHit{secUID: m[1], nickname: lastN(m[1], 8)}, nil
	}
	if bareSecUIDRe.MatchString(s) {
		return &userHit{secUID: s, nickname: lastN(s, 8)}, nil
	}
	if whitespaceRe.MatchString(s) {
		return nil, nil
	}
	hit, err := fetchUserByUniqueID(ctx, sc, s)
	if err != nil {
		if errors.Is(err, errAccountNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return hit, nil
}
