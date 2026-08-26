package wechat

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestClientDoesNotIncludeAppSecretInTransportError(t *testing.T) {
	client := NewClient(config.WeChatConfig{
		AppID:      "test-app",
		AppSecret:  "test-secret",
		APIBaseURL: "https://api.weixin.qq.com",
		Timeout:    time.Second,
	})
	client.httpClient.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("dial failed")
	})

	_, err := client.ExchangeCode(context.Background(), "code-1")
	if !errors.Is(err, ErrRemoteFailure) {
		t.Fatalf("error = %v, want ErrRemoteFailure", err)
	}
	if strings.Contains(err.Error(), "test-secret") {
		t.Fatalf("transport error leaked AppSecret: %v", err)
	}
}

func TestClientMapsWeChatErrorCodes(t *testing.T) {
	for _, test := range []struct {
		name        string
		body        string
		wantInvalid bool
	}{
		{name: "invalid code", body: `{"errcode":40029,"errmsg":"invalid code"}`, wantInvalid: true},
		{name: "used code", body: `{"errcode":40163,"errmsg":"code been used"}`, wantInvalid: true},
		{name: "upstream failure", body: `{"errcode":45011,"errmsg":"api freq outfreq"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()

			client := NewClient(config.WeChatConfig{AppID: "test-app", AppSecret: "test-secret", APIBaseURL: server.URL, Timeout: time.Second})
			_, err := client.ExchangeCode(context.Background(), "code-1")
			if test.wantInvalid {
				if !errors.Is(err, ErrInvalidCode) {
					t.Fatalf("error = %v, want ErrInvalidCode", err)
				}
				return
			}
			if !errors.Is(err, ErrRemoteFailure) {
				t.Fatalf("error = %v, want ErrRemoteFailure", err)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
