package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/example/go-service/internal/config"
)

func TestHealthReturnsSuccessAndRequestID(t *testing.T) {
	router, err := NewRouter(testConfig(), slog.New(slog.NewTextHandler(httptest.NewRecorder(), nil)), RouterOptions{
		Readiness: ReadinessCheckerFunc(func(context.Context) error { return nil }),
	})
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("X-Request-ID", "test-request-123")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("X-Request-ID"); got != "test-request-123" {
		t.Fatalf("request id = %q, want %q", got, "test-request-123")
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := body["code"]; got != float64(0) {
		t.Fatalf("response code = %v, want 0", got)
	}
}

func TestReadyReturnsServiceUnavailableWhenCheckFails(t *testing.T) {
	router, err := NewRouter(testConfig(), slog.Default(), RouterOptions{
		Readiness: ReadinessCheckerFunc(func(context.Context) error {
			return context.Canceled
		}),
	})
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ready", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestUnknownRouteReturnsJSONNotFound(t *testing.T) {
	router, err := NewRouter(testConfig(), slog.Default(), RouterOptions{})
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/not-found", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType == "" {
		t.Fatal("Content-Type header is empty")
	}
}

func testConfig() *config.Config {
	gin.SetMode(gin.TestMode)
	return &config.Config{
		Server: config.ServerConfig{
			MaxRequestBodyBytes: 2 << 20,
		},
	}
}
