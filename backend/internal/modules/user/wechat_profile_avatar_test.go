package user

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWeChatAvatarFetcherRejectsRedirectOutsideQlogo(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "not-an-avatar")
	}))
	defer target.Close()

	fetcher := newWeChatAvatarFetcherForTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", target.URL)
		w.WriteHeader(http.StatusFound)
	})
	if _, err := fetcher.Fetch(context.Background(), "https://thirdwx.qlogo.cn/mmopen/example/132", 2<<20); err == nil {
		t.Fatal("Fetch() error = nil, want redirect rejection")
	}
}

func TestWeChatAvatarFetcherRejectsOversizedBody(t *testing.T) {
	fetcher := newWeChatAvatarFetcherForTest(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "123456789")
	})
	if _, err := fetcher.Fetch(context.Background(), "https://thirdwx.qlogo.cn/mmopen/example/132", 8); err == nil {
		t.Fatal("Fetch() error = nil, want oversized-body rejection")
	}
}

func TestWeChatAvatarFetcherRejectsEmptyBody(t *testing.T) {
	fetcher := newWeChatAvatarFetcherForTest(t, func(w http.ResponseWriter, r *http.Request) {})
	if _, err := fetcher.Fetch(context.Background(), "https://thirdwx.qlogo.cn/mmopen/example/132", 2<<20); err == nil {
		t.Fatal("Fetch() error = nil, want empty-body rejection")
	}
}

func TestWeChatAvatarFetcherRejectsNonQlogoAndHTTPURLs(t *testing.T) {
	fetcher := newWeChatAvatarFetcherForTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("transport should not be called for invalid URL")
	})
	for _, rawURL := range []string{
		"http://thirdwx.qlogo.cn/mmopen/example/132",
		"https://example.com/avatar.png",
		"https://qlogo.cn.evil.example/avatar",
	} {
		t.Run(rawURL, func(t *testing.T) {
			if _, err := fetcher.Fetch(context.Background(), rawURL, 2<<20); err == nil {
				t.Fatalf("Fetch(%q) error = nil, want URL rejection", rawURL)
			}
		})
	}
}

func newWeChatAvatarFetcherForTest(t *testing.T, handler http.HandlerFunc) *HTTPWeChatAvatarFetcher {
	t.Helper()
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Scheme != "https" || !strings.HasSuffix(request.URL.Hostname(), ".qlogo.cn") {
			return nil, errors.New("unexpected request URL")
		}
		recorder := httptest.NewRecorder()
		handler(recorder, request)
		return recorder.Result(), nil
	})
	client := &http.Client{Transport: transport}
	return NewWeChatAvatarFetcher(client, time.Second)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
