// signature:YouTube base.js 的 n 参数(节流)解密。n 不解会限速 ~50KB/s 或 403。
// 步骤:URL 提取 n → 8 个正则挖函数名 → lexer 截函数体 → fixup 删 early-return →
// sc.JS.Call 执行(等价 new Function(code + return fn(...args))(...args))。
// base.js 与解密器按 URL 缓存(上游轮换时自动失效)。

package youtube

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"mygo-host/platform"
)

// youtubeOrigin 运行时为官方站点;测试可替换为本地 server。
var youtubeOrigin = "https://www.youtube.com"

var playerJsURLRe = regexp.MustCompile(`(?i)"jsUrl"\s*:\s*"((?:/[^"/\\]+/|)(?:[^"\\]+/)?(?:player/)?(?:[^"\\]+?/)?base(?:\.js)?(?:\?[^"\\]*)?)"`)

func extractPlayerJsURL(html string) string {
	match := playerJsURLRe.FindStringSubmatch(html)
	if len(match) < 2 {
		return ""
	}
	path := strings.ReplaceAll(match[1], `\/`, "/")
	path = strings.ReplaceAll(path, `\u002F`, "/")
	if strings.HasPrefix(path, "http") {
		return path
	}
	return youtubeOrigin + path
}

var playerCodeState struct {
	mu   sync.Mutex
	url  string
	code string
}

func getPlayerCode(ctx context.Context, sc *platform.ServiceContext, playerJsURL string) (string, error) {
	playerCodeState.mu.Lock()
	if playerCodeState.url == playerJsURL && playerCodeState.code != "" {
		code := playerCodeState.code
		playerCodeState.mu.Unlock()
		return code, nil
	}
	playerCodeState.mu.Unlock()
	code, err := sc.HTTP.GetText(ctx, playerJsURL, nil)
	if err != nil {
		return "", err
	}
	playerCodeState.mu.Lock()
	playerCodeState.url, playerCodeState.code = playerJsURL, code
	playerCodeState.mu.Unlock()
	return code, nil
}

func hasThrottlingParam(rawURL string) bool {
	return strings.Contains(rawURL, "&n=") || strings.Contains(rawURL, "?n=")
}

var nParamRe = regexp.MustCompile(`[&?]n=([^&]+)`)

func extractNParam(rawURL string) (string, bool) {
	match := nParamRe.FindStringSubmatch(rawURL)
	if len(match) < 2 {
		return "", false
	}
	return match[1], true
}

// 8 个函数名正则,逐条对照 NewPipe YoutubeThrottlingParameterUtils。
var nameRegexes = []*regexp.Regexp{
	regexp.MustCompile(`([A-Za-z0-9_$]{2,})=function.*return [A-Z]\[\d+\]`),
	regexp.MustCompile(`[a-zA-Z0-9$_]="nn"\[\+[a-zA-Z0-9$_]+\.[a-zA-Z0-9$_]+],[a-zA-Z0-9$_]+\([a-zA-Z0-9$_]+\),[a-zA-Z0-9$_]+=[a-zA-Z0-9$_]+\.[a-zA-Z0-9$_]+\[[a-zA-Z0-9$_]+]\|\|null\)&&\([a-zA-Z0-9$_]+=([a-zA-Z0-9$_]+)\[(\d+)]`),
	regexp.MustCompile(`[a-zA-Z0-9$_]="nn"\[\+[a-zA-Z0-9$_]+\.[a-zA-Z0-9$_]+],[a-zA-Z0-9$_]+\([a-zA-Z0-9$_]+\),[a-zA-Z0-9$_]+=[a-zA-Z0-9$_]+\.[a-zA-Z0-9$_]+\[[a-zA-Z0-9$_]+]\|\|null\).+\|\|([a-zA-Z0-9$_]+)\(""\)`),
	regexp.MustCompile(`,[a-zA-Z0-9$_]+\([a-zA-Z0-9$_]+\),[a-zA-Z0-9$_]+=[a-zA-Z0-9$_]+\.[a-zA-Z0-9$_]+\[[a-zA-Z0-9$_]+]\|\|null\)&&\([a-zA-Z0-9$_]+=([a-zA-Z0-9$_]+)\[(\d+)]\([a-zA-Z0-9$_]+\),[a-zA-Z0-9$_]+\.set\((?:"n+"|[a-zA-Z0-9$_]+),[a-zA-Z0-9$_]+\)`),
	regexp.MustCompile(`[a-zA-Z0-9$_]="nn"\[\+[a-zA-Z0-9$_]+\.[a-zA-Z0-9$_]+],[a-zA-Z0-9$_]+=[a-zA-Z0-9$_]+\.get\([a-zA-Z0-9$_]+\)\).+\|\|([a-zA-Z0-9$_]+)\(""\)`),
	regexp.MustCompile(`[a-zA-Z0-9$_]="nn"\[\+[a-zA-Z0-9$_]+\.[a-zA-Z0-9$_]+],[a-zA-Z0-9$_]+=[a-zA-Z0-9$_]+\.get\([a-zA-Z0-9$_]+\)\)&&\([a-zA-Z0-9$_]+=([a-zA-Z0-9$_]+)\[(\d+)]`),
	regexp.MustCompile(`\([a-zA-Z0-9$_]+=String\.fromCharCode\(110\),[a-zA-Z0-9$_]+=[a-zA-Z0-9$_]+\.get\([a-zA-Z0-9$_]+\)\)&&\([a-zA-Z0-9$_]+=([a-zA-Z0-9$_]+)(?:\[(\d+)])?\([a-zA-Z0-9$_]+\)`),
	regexp.MustCompile(`\.get\("n"\)\)&&\([a-zA-Z0-9$_]+=([a-zA-Z0-9$_]+)(?:\[(\d+)])?\([a-zA-Z0-9$_]+\)`),
}

func extractFunctionName(code string) (string, *int, bool) {
	for _, re := range nameRegexes {
		match := re.FindStringSubmatchIndex(code)
		if match == nil {
			continue
		}
		name := code[match[2]:match[3]]
		var arrayIndex *int
		if len(match) >= 6 && match[4] >= 0 && match[5] > match[4] {
			value, err := strconv.Atoi(code[match[4]:match[5]])
			if err == nil {
				arrayIndex = &value
			}
		}
		return name, arrayIndex, true
	}
	return "", nil, false
}

func resolveArrayFunctionName(code, name string, arrayIndex *int) string {
	if arrayIndex == nil {
		return name
	}
	arrRe := regexp.MustCompile(`var\s+` + regexp.QuoteMeta(name) + `\s*=\s*\[([^\]]+)]\s*;`)
	match := arrRe.FindStringSubmatch(code)
	if match == nil {
		return name
	}
	names := strings.Split(match[1], ",")
	for i := range names {
		names[i] = strings.Trim(strings.TrimSpace(names[i]), `'"`)
	}
	if *arrayIndex >= 0 && *arrayIndex < len(names) && names[*arrayIndex] != "" {
		return names[*arrayIndex]
	}
	return name
}

func matchToClosingBrace(code, start string) string {
	idx := strings.Index(code, start)
	if idx < 0 {
		return ""
	}
	openIdx := strings.Index(code[idx:], "{")
	if openIdx < 0 {
		return ""
	}
	openIdx += idx
	depth := 0
	var quote byte
	inTemplate := false
	for i := openIdx; i < len(code); i++ {
		ch := code[i]
		var prev byte
		if i > 0 {
			prev = code[i-1]
		}
		if quote != 0 {
			if ch == quote && prev != '\\' {
				quote = 0
			}
			continue
		}
		if inTemplate {
			if ch == '`' && prev != '\\' {
				inTemplate = false
			}
			continue
		}
		switch ch {
		case '"', '\'':
			quote = ch
		case '`':
			inTemplate = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return code[idx : i+1]
			}
		}
	}
	return ""
}

func extractFunction(code, name string) (string, error) {
	fnName := regexp.QuoteMeta(name)
	lexerBody := matchToClosingBrace(code, name+"=function")
	if lexerBody != "" {
		re := regexp.MustCompile(`(?s)` + fnName + `=function(.*)$`)
		if match := re.FindStringSubmatch(lexerBody); match != nil {
			return "function " + name + match[1], nil
		}
	}
	re := regexp.MustCompile(`(?s)` + fnName + `=\s*function(.*?}\s*return [\w$]+?\.join\(""\)\s*\};)`)
	if match := re.FindStringSubmatch(code); match != nil {
		return "function " + name + match[1], nil
	}
	return "", fmt.Errorf("YouTube n 参数解密函数提取失败(%s)", name)
}

// fixupFunction 删掉 `if (typeof x === "undefined") return arg;`(单独执行会误返)。
// TS 版正则要求 `=function(` 而 extractFunction 产出的是 `function name(...)`,
// 实际是 no-op;这里按注释意图直接匹配函数签名(见移植报告的差异说明)。
func fixupFunction(fn string) string {
	argRe := regexp.MustCompile(`function(?:\s+[a-zA-Z0-9_$]+)?\s*\(\s*([^)]*)\s*\)`)
	match := argRe.FindStringSubmatch(fn)
	if match == nil {
		return fn
	}
	firstArg := strings.TrimSpace(strings.Split(match[1], ",")[0])
	if firstArg == "" {
		return fn
	}
	earlyRe := regexp.MustCompile(`;\s*if\s*\(\s*typeof\s+[a-zA-Z0-9_$]+\s*===?\s*(?:"undefined"|'undefined')\s*\)\s*return\s+` +
		regexp.QuoteMeta(firstArg) + `\s*;`)
	return earlyRe.ReplaceAllString(fn, ";")
}

type nDeobfuscator struct {
	url    string
	code   string
	fnName string
}

var deobfState struct {
	mu    sync.Mutex
	cache *nDeobfuscator
}

func getNDeobfuscator(ctx context.Context, sc *platform.ServiceContext, playerJsURL string) (*nDeobfuscator, error) {
	deobfState.mu.Lock()
	if cache := deobfState.cache; cache != nil && cache.url == playerJsURL {
		deobfState.mu.Unlock()
		return cache, nil
	}
	deobfState.mu.Unlock()

	code, err := getPlayerCode(ctx, sc, playerJsURL)
	if err != nil {
		return nil, err
	}
	name, arrayIndex, ok := extractFunctionName(code)
	if !ok {
		return nil, errors.New("YouTube n 参数解密函数名未找到")
	}
	realName := resolveArrayFunctionName(code, name, arrayIndex)
	fn, err := extractFunction(code, realName)
	if err != nil {
		return nil, err
	}
	deobf := &nDeobfuscator{url: playerJsURL, code: fixupFunction(fn), fnName: realName}

	deobfState.mu.Lock()
	deobfState.cache = deobf
	deobfState.mu.Unlock()
	return deobf, nil
}

func fetchPlayerJsURL(ctx context.Context, sc *platform.ServiceContext, videoID string) string {
	html, err := sc.HTTP.GetText(ctx, youtubeOrigin+"/watch?v="+videoID+"&hl=en", nil)
	if err != nil {
		return ""
	}
	return extractPlayerJsURL(html)
}

// deobfuscateNParam 解密 URL 的 n 参数并替换回;失败返回 error(上层放弃该流)。
func deobfuscateNParam(ctx context.Context, sc *platform.ServiceContext, rawURL, videoID string) (string, error) {
	n, ok := extractNParam(rawURL)
	if !ok {
		return rawURL, nil
	}
	playerJsURL := fetchPlayerJsURL(ctx, sc, videoID)
	if playerJsURL == "" {
		return "", errors.New("YouTube base.js URL 获取失败")
	}
	deobf, err := getNDeobfuscator(ctx, sc, playerJsURL)
	if err != nil {
		return "", err
	}
	result, err := sc.JS.Call(ctx, deobf.code, deobf.fnName, []any{n})
	if err != nil {
		return "", err
	}
	decoded := jsString(result)
	if decoded == "" {
		return "", errors.New("YouTube n 参数解密返回空")
	}
	encoded := encodeURIComponent(decoded)
	return nParamRe.ReplaceAllStringFunc(rawURL, func(match string) string {
		return match[:1] + "n=" + encoded
	}), nil
}

func encodeURIComponent(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}
