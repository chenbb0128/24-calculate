package wechat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/example/go-service/internal/config"
)

func TestClientExchangeCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("appid") != "test-app" || r.URL.Query().Get("secret") != "test-secret" || r.URL.Query().Get("js_code") != "code-1" {
			t.Fatalf("query = %v", r.URL.Query())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"openid":"openid-1","unionid":"unionid-1"}`))
	}))
	defer server.Close()

	client := NewClient(config.WeChatConfig{
		AppID:      "test-app",
		AppSecret:  "test-secret",
		APIBaseURL: server.URL,
		Timeout:    time.Second,
	})
	result, err := client.ExchangeCode(context.Background(), "code-1")
	if err != nil {
		t.Fatalf("ExchangeCode() error = %v", err)
	}
	if result.OpenID != "openid-1" || result.UnionID != "unionid-1" {
		t.Fatalf("result = %+v", result)
	}
}

func TestClientNotConfigured(t *testing.T) {
	client := NewClient(config.WeChatConfig{})
	_, err := client.ExchangeCode(context.Background(), "code-1")
	if err != ErrNotConfigured {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
}
