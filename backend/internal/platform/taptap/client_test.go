package taptap

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
		query := r.URL.Query()
		if query.Get("appid") != "mini-app" || query.Get("secret") != "mini-secret" || query.Get("js_code") != "code-1" || query.Get("grant_type") != "authorization_code" {
			t.Fatalf("query = %v", query)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"openid":"openid-1","unionid":"unionid-1"}`))
	}))
	defer server.Close()

	client := NewClient(config.TapTapConfig{
		AppID: "mini-app", AppSecret: "mini-secret", APIBaseURL: server.URL, Timeout: time.Second,
	})
	result, err := client.ExchangeCode(context.Background(), "code-1")
	if err != nil {
		t.Fatalf("ExchangeCode() error = %v", err)
	}
	if result.OpenID != "openid-1" || result.UnionID != "unionid-1" {
		t.Fatalf("result = %+v", result)
	}
}

func TestClientMapsInvalidCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":1040029,"errmsg":"invalid code"}`))
	}))
	defer server.Close()

	client := NewClient(config.TapTapConfig{AppID: "mini-app", AppSecret: "mini-secret", APIBaseURL: server.URL, Timeout: time.Second})
	_, err := client.ExchangeCode(context.Background(), "bad-code")
	if !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("error = %v, want ErrInvalidCode", err)
	}
}

func TestClientDoesNotLeakAppSecretOnTransportError(t *testing.T) {
	client := NewClient(config.TapTapConfig{
		AppID: "mini-app", AppSecret: "mini-secret", APIBaseURL: "https://cloud-miniapp.tapapis.cn", Timeout: time.Second,
	})
	client.httpClient.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("dial failed")
	})

	_, err := client.ExchangeCode(context.Background(), "code-1")
	if !errors.Is(err, ErrRemoteFailure) {
		t.Fatalf("error = %v, want ErrRemoteFailure", err)
	}
	if strings.Contains(err.Error(), "mini-secret") {
		t.Fatalf("transport error leaked AppSecret: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
