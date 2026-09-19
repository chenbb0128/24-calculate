package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/example/go-service/internal/config"
	"github.com/example/go-service/internal/modules/player"
	"github.com/example/go-service/internal/modules/user"
	jwtplatform "github.com/example/go-service/internal/platform/jwt"
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

func TestSuccessReturnsSuccess(t *testing.T) {
	router, err := NewRouter(testConfig(), slog.Default(), RouterOptions{})
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/success", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if body["code"] != float64(0) || !ok || data["status"] != "success" {
		t.Fatalf("success envelope = %s", recorder.Body.String())
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

func TestUnknownAdminRouteReturnsStandardJSONNotFound(t *testing.T) {
	router, err := NewRouter(testConfig(), slog.Default(), RouterOptions{})
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/not-found", nil))

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["code"] != float64(10002) || body["data"] != nil {
		t.Fatalf("not-found envelope = %s", recorder.Body.String())
	}
}

func TestOrdinaryRoutesRejectAdminTokens(t *testing.T) {
	manager := newRouterJWTManager(t)
	router, err := NewRouter(testConfig(), slog.Default(), RouterOptions{
		APIRoutes: func(group *gin.RouterGroup) {
			user.RegisterRoutes(group, nil, manager)
			player.RegisterRoutes(group, nil, manager)
		},
	})
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	token, _, err := manager.IssueAccessTokenWithRole(9, jwtplatform.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/api/v1/users/me", "/api/v1/player/bootstrap"} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.Header.Set("Authorization", "Bearer "+token)
			router.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusForbidden, recorder.Body.String())
			}
		})
	}
}

func TestWeChatProfileSyncRouteIsRegisteredAtExactPath(t *testing.T) {
	manager := newRouterJWTManager(t)
	router, err := NewRouter(testConfig(), slog.Default(), RouterOptions{
		APIRoutes: func(group *gin.RouterGroup) {
			user.RegisterRoutes(group, nil, manager)
		},
	})
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/me/wechat-profile/sync", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d for registered POST-only path: %s", recorder.Code, http.StatusMethodNotAllowed, recorder.Body.String())
	}
}

func TestAvatarStaticFileIsServedFromConfiguredStorage(t *testing.T) {
	root := t.TempDir()
	avatarPath := filepath.Join(root, "avatars", "7", "test.webp")
	if err := os.MkdirAll(filepath.Dir(avatarPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(avatarPath, []byte("webp-test"), 0o640); err != nil {
		t.Fatal(err)
	}

	router, err := NewRouter(testConfig(), slog.Default(), RouterOptions{AvatarStorageDir: root})
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/avatars/7/test.webp", nil))

	if recorder.Code != http.StatusOK || recorder.Body.String() != "webp-test" {
		t.Fatalf("status = %d, body = %q", recorder.Code, recorder.Body.String())
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

func newRouterJWTManager(t *testing.T) *jwtplatform.Manager {
	t.Helper()
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	manager, err := jwtplatform.NewManager(config.JWTConfig{
		Secret: string(secret), Algorithm: "HS256", Issuer: "router-test", AccessTTL: time.Minute, RefreshTTL: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	return manager
}
