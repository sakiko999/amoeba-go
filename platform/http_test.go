// GetBytes 必须保留原始响应字节:二维码 PNG 经过字符串转码会退化成空白图。

package platform

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetBytesPreservesBinary(t *testing.T) {
	payload := []byte{0x89, 'P', 'N', 'G', 0x00, 0xff, 0xfe, 0x80, 0x01}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	got, contentType, err := NewHostClient().GetBytes(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("二进制 body 被改写: got %x, want %x", got, payload)
	}
	if contentType != "image/png" {
		t.Fatalf("Content-Type = %q, want image/png", contentType)
	}
}

// 用真实结构的最小 PNG 走一遍:string 中转若丢字节,这里会解不出 PNG 头。
func TestGetBytesPreservesPNG(t *testing.T) {
	payload := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	got, _, err := NewHostClient().GetBytes(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("PNG 字节被改写: got %x, want %x", got, payload)
	}
}
