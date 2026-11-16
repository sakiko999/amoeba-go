// client 请求体自检(移植 src/example/youtube-clients.check.ts):VISIONOS 设备字段
// 与 visitorData 必须透传(缺失会静默变成「不可播」);cpn 生成基本形状。

package youtube

import (
	"strings"
	"testing"
)

func TestVisionosPlayerBody(t *testing.T) {
	body := visionosPlayerBody("TxeCpgGK9Ok", "VISITOR_DATA_X", "CPN1234567890")
	client, ok := body["context"].(map[string]any)["client"].(map[string]any)
	if !ok {
		t.Fatal("context.client 缺失")
	}
	checks := map[string]string{
		"clientName":    "VISIONOS",
		"clientVersion": visionosClientVersion,
		"deviceMake":    "Apple",
		"deviceModel":   "RealityDevice17,1",
		"osName":        "visionOS",
		"visitorData":   "VISITOR_DATA_X",
	}
	for key, want := range checks {
		if got, _ := client[key].(string); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if version, _ := client["osVersion"].(string); !strings.HasPrefix(version, "26.") {
		t.Errorf("osVersion = %q", version)
	}
	if body["videoId"] != "TxeCpgGK9Ok" || body["cpn"] != "CPN1234567890" {
		t.Errorf("videoId/cpn = %v/%v", body["videoId"], body["cpn"])
	}
	if body["contentCheckOk"] != true || body["racyCheckOk"] != true {
		t.Error("contentCheckOk / racyCheckOk 都应开启")
	}
}

func TestGenerateCpn(t *testing.T) {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"
	cpn := generateCpn()
	if len(cpn) != 13 {
		t.Fatalf("cpn 长度 = %d", len(cpn))
	}
	for _, c := range cpn {
		if !strings.ContainsRune(chars, c) {
			t.Errorf("cpn 含非法字符 %q", c)
		}
	}
}
