// InnerTube player 请求:四套 client context(VISIONOS / ANDROID_VR / WEB / IOS)。
// 主力 VISIONOS 免 POT;ANDROID_VR 兜底;WEB 只作错误信息更具体的最后尝试;
// IOS 仅用于直播 HLS 兜底。设备字段缺失会被上游静默改判「不可播」。

package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"

	"mygo-host/platform"
)

func fetchPlayerResponse(ctx context.Context, sc *platform.ServiceContext, url string, body any, headers map[string]string) (*playerResponse, error) {
	raw, err := postJSON(ctx, sc, url, body, headers)
	if err != nil {
		return nil, err
	}
	var res playerResponse
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func playerRequestBody(client map[string]any, videoID, cpn string) map[string]any {
	return map[string]any{
		"context":        map[string]any{"client": client},
		"videoId":        videoID,
		"cpn":            cpn,
		"contentCheckOk": true,
		"racyCheckOk":    true,
	}
}

// getAndroidPlayerResponse ANDROID_VR(免 poToken 直链;gapis + t/id 参数)。
func getAndroidPlayerResponse(ctx context.Context, sc *platform.ServiceContext, videoID, visitorData string) (*playerResponse, error) {
	client := map[string]any{
		"clientName": "ANDROID_VR", "clientVersion": androidClientVersion,
		"deviceMake": "Oculus", "deviceModel": "Quest 3", "androidSdkVersion": 32,
		"osName": "Android", "osVersion": "12L", "hl": "en", "visitorData": visitorData,
	}
	url := fmt.Sprintf("%s/player?prettyPrint=false&t=%d&id=%s", youtubeiGapisV1, sc.NowMs(), videoID)
	return fetchPlayerResponse(ctx, sc, url, playerRequestBody(client, videoID, generateCpn()), map[string]string{
		"user-agent": androidUA, "X-Goog-Api-Format-Version": "2",
	})
}

func getWebPlayerResponse(ctx context.Context, sc *platform.ServiceContext, videoID, visitorData string) (*playerResponse, error) {
	client := map[string]any{
		"clientName": "WEB", "clientVersion": webClientVersion, "hl": "en", "visitorData": visitorData,
	}
	return fetchPlayerResponse(ctx, sc, youtubeiV1+"/player?prettyPrint=false", playerRequestBody(client, videoID, generateCpn()), map[string]string{
		"user-agent": desktopChromeUA,
		"referer":    "https://www.youtube.com/watch?v=" + videoID,
	})
}

// getIosPlayerResponse IOS client(直播兜底:VR 无 hls 时用,分离音视频)。
func getIosPlayerResponse(ctx context.Context, sc *platform.ServiceContext, videoID, visitorData string) (*playerResponse, error) {
	client := map[string]any{
		"clientName": "IOS", "clientVersion": iosClientVersion,
		"deviceMake": "Apple", "deviceModel": iosDeviceModel, "hl": "en", "visitorData": visitorData,
	}
	url := fmt.Sprintf("%s/player?prettyPrint=false&t=%d&id=%s", youtubeiGapisV1, sc.NowMs(), videoID)
	return fetchPlayerResponse(ctx, sc, url, playerRequestBody(client, videoID, generateCpn()), map[string]string{
		"user-agent": iosUA, "X-Goog-Api-Format-Version": "2",
	})
}

// visionosPlayerBody 导出供自检钉住形状:设备字段缺失会静默变成「不可播」。
func visionosPlayerBody(videoID, visitorData, cpn string) map[string]any {
	client := map[string]any{
		"clientName": "VISIONOS", "clientVersion": visionosClientVersion,
		"deviceMake": "Apple", "deviceModel": visionosDeviceModel,
		"osName": "visionOS", "osVersion": visionosOSVersion,
		"hl": "en", "gl": "US", "visitorData": visitorData,
	}
	return playerRequestBody(client, videoID, cpn)
}

func getVisionosPlayerResponse(ctx context.Context, sc *platform.ServiceContext, videoID, visitorData string) (*playerResponse, error) {
	url := fmt.Sprintf("%s/player?prettyPrint=false&t=%d&id=%s", youtubeiGapisV1, sc.NowMs(), videoID)
	return fetchPlayerResponse(ctx, sc, url, visionosPlayerBody(videoID, visitorData, generateCpn()), map[string]string{
		"user-agent": visionosUA, "X-Goog-Api-Format-Version": "2",
	})
}

// generateCpn 随机 cpn(content playback nonce,13 位 base64url)。
func generateCpn() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"
	var b strings.Builder
	for i := 0; i < 13; i++ {
		b.WriteByte(chars[rand.Intn(len(chars))])
	}
	return b.String()
}
