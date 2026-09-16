package wechat_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/example/go-service/internal/config"
	"github.com/example/go-service/internal/modules/moderation"
	wechatplatform "github.com/example/go-service/internal/platform/wechat"
)

func TestContentSafetyClientChecksTextAndCachesServerToken(t *testing.T) {
	var mu sync.Mutex
	tokenCalls, textCalls := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/cgi-bin/token":
			tokenCalls++
			if r.URL.Query().Get("appid") != "app-id" || r.URL.Query().Get("secret") != "app-secret" {
				t.Fatalf("token query = %s", r.URL.RawQuery)
			}
			_, _ = io.WriteString(w, `{"access_token":"server-token","expires_in":7200}`)
		case "/wxa/msg_sec_check":
			textCalls++
			if r.URL.Query().Get("access_token") != "server-token" || strings.Contains(r.URL.RawQuery, "app-secret") {
				t.Fatalf("content query = %s", r.URL.RawQuery)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["content"] != "玩家" {
				t.Fatalf("content body = %#v, err = %v", body, err)
			}
			_, _ = io.WriteString(w, `{"errcode":0}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := wechatplatform.NewClient(config.WeChatConfig{AppID: "app-id", AppSecret: "app-secret", APIBaseURL: server.URL})
	request := moderation.TextCheckRequest{UserID: 7, Subject: "openid", Source: "profile", Content: "玩家"}
	first, err := client.CheckText(context.Background(), request)
	if err != nil || first.Status != moderation.StatusApproved {
		t.Fatalf("first result = %+v, err = %v", first, err)
	}
	second, err := client.CheckText(context.Background(), request)
	if err != nil || second.Status != moderation.StatusApproved {
		t.Fatalf("second result = %+v, err = %v", second, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if tokenCalls != 1 || textCalls != 2 {
		t.Fatalf("token calls = %d, text calls = %d", tokenCalls, textCalls)
	}
}

func TestContentSafetyClientChecksImageWithMultipartMedia(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/token":
			_, _ = io.WriteString(w, `{"access_token":"image-token","expires_in":7200}`)
		case "/wxa/img_sec_check":
			if r.URL.Query().Get("access_token") != "image-token" || !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data;") {
				t.Fatalf("image request headers = %#v", r.Header)
			}
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatalf("ParseMultipartForm() error = %v", err)
			}
			file, _, err := r.FormFile("media")
			if err != nil {
				t.Fatalf("media part error = %v", err)
			}
			defer file.Close()
			data, err := io.ReadAll(file)
			if err != nil || string(data) != "image-bytes" {
				t.Fatalf("media data = %q, err = %v", data, err)
			}
			_, _ = io.WriteString(w, `{"errcode":0}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := wechatplatform.NewClient(config.WeChatConfig{AppID: "app-id", AppSecret: "app-secret", APIBaseURL: server.URL})
	result, err := client.CheckImage(context.Background(), moderation.ImageCheckRequest{UserID: 7, Source: "avatar", Content: []byte("image-bytes")})
	if err != nil || result.Status != moderation.StatusApproved {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
}

func TestContentSafetyClientMaps87014ToRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cgi-bin/token" {
			_, _ = io.WriteString(w, `{"access_token":"token","expires_in":7200}`)
			return
		}
		_, _ = io.WriteString(w, `{"errcode":87014,"errmsg":"sensitive details"}`)
	}))
	defer server.Close()

	client := wechatplatform.NewClient(config.WeChatConfig{AppID: "app-id", AppSecret: "app-secret", APIBaseURL: server.URL})
	result, err := client.CheckText(context.Background(), moderation.TextCheckRequest{Content: "bad"})
	if err != nil || result.Status != moderation.StatusRejected || result.ReasonCode != "87014" {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
}

func TestContentSafetyClientDoesNotExposeSecretInErrorOrLog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"errcode":-1,"errmsg":"upstream failure"}`)
	}))
	defer server.Close()

	client := wechatplatform.NewClient(config.WeChatConfig{AppID: "app-id", AppSecret: "app-secret", APIBaseURL: server.URL})
	_, err := client.CheckText(context.Background(), moderation.TextCheckRequest{Content: "玩家"})
	if err == nil || strings.Contains(err.Error(), "app-secret") {
		t.Fatalf("error = %v, secret leaked", err)
	}
}

func TestContentSafetyClientRefreshesAfterInvalidAccessToken(t *testing.T) {
	var mu sync.Mutex
	tokenCalls, textCalls := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/cgi-bin/token":
			tokenCalls++
			_, _ = io.WriteString(w, `{"access_token":"token-`+string(rune('0'+tokenCalls))+`","expires_in":7200}`)
		case "/wxa/msg_sec_check":
			textCalls++
			if textCalls == 1 {
				_, _ = io.WriteString(w, `{"errcode":40001,"errmsg":"invalid credential"}`)
				return
			}
			_, _ = io.WriteString(w, `{"errcode":0}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := wechatplatform.NewClient(config.WeChatConfig{AppID: "app-id", AppSecret: "app-secret", APIBaseURL: server.URL})
	result, err := client.CheckText(context.Background(), moderation.TextCheckRequest{Content: "玩家"})
	if err != nil || result.Status != moderation.StatusApproved {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if tokenCalls != 2 || textCalls != 2 {
		t.Fatalf("token calls = %d, text calls = %d", tokenCalls, textCalls)
	}
}
