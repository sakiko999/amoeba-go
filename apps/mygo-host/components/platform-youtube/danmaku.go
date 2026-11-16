// YouTube 直播弹幕:抓直播间页面提 ytcfg(apiKey/context)+ ytInitialData(初始
// continuation)→ 递归 POST get_live_chat 轮询增量文本。不是 WebSocket。
// 轮询语义:失败前 3 次 warn,之后静默指数退避(上限 60s);continuations 用尽 =
// 直播结束,立即终止(否则每 5s 一条 warn 永不停止)。

package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"mygo-host/platform"
)

const (
	chatAPI          = "https://www.youtube.com/youtubei/v1/live_chat/get_live_chat"
	defaultTimeoutMs = 5000
	maxBackoffMs     = 60000
	maxWarnFailures  = 3
)

var chatEndedRe = regexp.MustCompile(`无下一 ?continuation`)

// nextPollDelay 前 2 次按服务端 timeoutMs,之后指数退避(上限 60s)。
func nextPollDelay(timeoutMs, failures int) int {
	if failures > 1 {
		delay := float64(timeoutMs) * math.Pow(2, float64(failures-1))
		if delay >= maxBackoffMs {
			return maxBackoffMs
		}
		return int(delay)
	}
	return timeoutMs
}

// isChatEndedError continuations 用尽 = 直播真的结束;瞬时无 continuation 不算。
func isChatEndedError(message string) bool {
	return chatEndedRe.MatchString(message)
}

type chatState struct {
	token      string
	timeoutMs  int
	contextRaw json.RawMessage
	apiKey     string
}

// findReloadContinuation 递归搜任意子树里的 reloadContinuationData.continuation。
func findReloadContinuation(node *jsonValue, depth int) string {
	if node == nil || depth > 10 {
		return ""
	}
	switch node.kind {
	case jsonArray:
		for _, item := range node.arr {
			if found := findReloadContinuation(item, depth+1); found != "" {
				return found
			}
		}
	case jsonObject:
		for _, pair := range node.obj {
			if pair.key == "reloadContinuationData" {
				if value := pair.val.get("continuation").asJSString(); value != "" {
					return value
				}
			}
			if found := findReloadContinuation(pair.val, depth+1); found != "" {
				return found
			}
		}
	}
	return ""
}

func initialContinuation(data *jsonValue) string {
	direct, _ := data.path("contents", "twoColumnWatchNextResults", "conversationBar",
		"liveChatRenderer", "continuations").firstItem().path("reloadContinuationData", "continuation").asText()
	if direct != "" {
		return direct
	}
	return findReloadContinuation(data, 0)
}

// chatRunsToText message.runs → 纯文本(text 直取,emoji 取 emojiId)。
func chatRunsToText(runs *jsonValue) string {
	var b strings.Builder
	for _, run := range runs.items() {
		if text, ok := run.get("text").asText(); ok {
			b.WriteString(text)
			continue
		}
		if emoji, ok := run.path("emoji", "emojiId").asText(); ok {
			b.WriteString(emoji)
		}
	}
	return strings.TrimSpace(b.String())
}

func fetchChatPage(ctx context.Context, sc *platform.ServiceContext, liveID string) (string, error) {
	return sc.HTTP.GetText(ctx, "https://www.youtube.com/live/"+liveID, map[string]string{
		"user-agent": desktopChromeUA, "accept-language": "en-US,en;q=0.9",
	})
}

func initChat(ctx context.Context, sc *platform.ServiceContext, liveID string) (*chatState, error) {
	html, err := fetchChatPage(ctx, sc, liveID)
	if err != nil {
		return nil, err
	}
	// 对象形式 marker:页面顶部还有单 key 调用 ytcfg.set('KEY', value),裸 marker 会错位。
	cfgRaw, err := extractInlineJSON(html, "ytcfg.set({", "YouTube ytcfg")
	if err != nil {
		return nil, err
	}
	var cfg struct {
		APIKey  any             `json:"INNERTUBE_API_KEY"`
		Context json.RawMessage `json:"INNERTUBE_CONTEXT"`
	}
	if err := json.Unmarshal([]byte(cfgRaw), &cfg); err != nil {
		return nil, err
	}
	apiKey := jsString(cfg.APIKey)
	initialRaw, err := extractInlineJSON(html, "ytInitialData", "YouTube ytInitialData")
	if err != nil {
		return nil, err
	}
	initial, err := parseJSONValue([]byte(initialRaw))
	if err != nil {
		return nil, err
	}
	token := initialContinuation(initial)
	if apiKey == "" || token == "" {
		return nil, errors.New("youtube live chat: 未开播或无聊天(页面无 liveChat continuation)")
	}
	contextRaw := cfg.Context
	if len(contextRaw) == 0 || string(contextRaw) == "null" {
		contextRaw = json.RawMessage("{}")
	}
	return &chatState{token: token, timeoutMs: defaultTimeoutMs, contextRaw: contextRaw, apiKey: apiKey}, nil
}

func pollOnce(ctx context.Context, sc *platform.ServiceContext, liveID string, state *chatState, onItems func([]platform.DanmakuItem)) error {
	body := map[string]any{"context": state.contextRaw, "continuation": state.token}
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}
	res, err := sc.HTTP.Do(ctx, platform.Request{
		URL:    chatAPI + "?key=" + encodeURIComponent(state.apiKey),
		Method: "POST",
		Headers: map[string]string{
			"user-agent":   desktopChromeUA,
			"content-type": "application/json",
			"origin":       "https://www.youtube.com",
			"referer":      "https://www.youtube.com/live/" + liveID,
		},
		Body: string(encoded),
	})
	if err != nil {
		return err
	}
	if res.Status != 200 {
		return fmt.Errorf("youtube live chat HTTP %d", res.Status)
	}
	data, err := parseJSONValue([]byte(res.Body))
	if err != nil {
		return err
	}
	liveChat := data.path("continuationContents", "liveChatContinuation")
	if liveChat == nil {
		return errors.New("youtube live chat: 无 continuation(直播结束?)")
	}
	if items := chatItems(liveChat); len(items) > 0 {
		onItems(items)
	}
	token, timeoutMs, err := chatContinuation(liveChat)
	if err != nil {
		return err
	}
	state.token, state.timeoutMs = token, timeoutMs
	return nil
}

// chatItems 从 actions 提取增量文本消息(emoji run 用 emojiId)。
func chatItems(liveChat *jsonValue) []platform.DanmakuItem {
	items := []platform.DanmakuItem{}
	for _, action := range liveChat.get("actions").items() {
		message := action.path("addChatItemAction", "item", "liveChatTextMessageRenderer")
		if message == nil {
			continue
		}
		text := chatRunsToText(message.path("message", "runs"))
		if text == "" {
			continue
		}
		user, _ := message.path("authorName", "simpleText").asText()
		items = append(items, platform.DanmakuItem{Text: text, User: &user})
	}
	return items
}

// chatContinuation 下一 token + 轮询间隔;无 continuation = 直播真的结束。
func chatContinuation(liveChat *jsonValue) (string, int, error) {
	continuations := liveChat.get("continuations").items()
	var cont *jsonValue
	if len(continuations) > 0 {
		cont = continuations[0].get("timedContinuationData")
		if cont == nil {
			cont = continuations[0].get("invalidationContinuationData")
		}
	}
	if cont == nil {
		return "", 0, errors.New("youtube live chat: 直播结束(无下一 continuation)")
	}
	token, ok := cont.get("continuation").asText()
	if !ok || token == "" {
		return "", 0, errors.New("youtube live chat: 直播结束(无下一 continuation)")
	}
	timeoutMs := defaultTimeoutMs
	if timeout := cont.get("timeoutMs"); timeout != nil {
		timeoutMs = int(timeout.asNumber())
	}
	return token, timeoutMs, nil
}

// createLiveChatPoller 返回 (onItems) => unsubscribe,启动轮询循环。
func createLiveChatPoller(sc *platform.ServiceContext, liveID string) platform.DanmakuStream {
	return func(onItems func(items []platform.DanmakuItem)) func() {
		ctx, cancel := context.WithCancel(context.Background())
		go runChatPoller(ctx, sc, liveID, onItems)
		return cancel
	}
}

func runChatPoller(ctx context.Context, sc *platform.ServiceContext, liveID string, onItems func([]platform.DanmakuItem)) {
	state, err := initChat(ctx, sc, liveID)
	if err != nil {
		if ctx.Err() == nil {
			sc.Log("youtube: live chat 初始化失败: %v", err)
		}
		return
	}
	failures := 0
	for {
		timer := time.NewTimer(time.Duration(nextPollDelay(state.timeoutMs, failures)) * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if err := pollOnce(ctx, sc, liveID, state, onItems); err != nil {
			if ctx.Err() != nil {
				return
			}
			message := err.Error()
			if isChatEndedError(message) {
				sc.Log("youtube: live chat 已结束,停止轮询")
				return
			}
			failures++
			if failures <= maxWarnFailures {
				sc.Log("youtube: live chat 轮询失败: %s", message)
			}
			continue
		}
		if failures > 0 {
			sc.Log("youtube: live chat 轮询恢复(此前连续失败 %d 次)", failures)
		}
		failures = 0
	}
}
