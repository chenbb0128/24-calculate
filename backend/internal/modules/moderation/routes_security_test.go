package moderation_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/example/go-service/internal/apperror"
	"github.com/example/go-service/internal/config"
	httpapi "github.com/example/go-service/internal/http/response"
	"github.com/example/go-service/internal/modules/user"
	jwtplatform "github.com/example/go-service/internal/platform/jwt"
)

func TestAvatarRouteStillRequiresBearerAndMultipartFileField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager, err := jwtplatform.NewManager(config.JWTConfig{
		Secret: "01234567890123456789012345678901", Algorithm: "HS256", Issuer: "go-service", AccessTTL: time.Minute, RefreshTTL: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	group := router.Group("/api/v1")
	user.RegisterRoutes(group, user.NewHandler(user.NewService(nil)), manager)

	withoutToken := httptest.NewRecorder()
	router.ServeHTTP(withoutToken, httptest.NewRequest(http.MethodPost, "/api/v1/users/me/avatar", nil))
	if withoutToken.Code != http.StatusUnauthorized {
		t.Fatalf("without token status = %d, want 401", withoutToken.Code)
	}

	token, _, err := manager.IssueAccessToken(7)
	if err != nil {
		t.Fatal(err)
	}
	withoutFile := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/users/me/avatar", strings.NewReader("not multipart"))
	request.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(withoutFile, request)
	if withoutFile.Code != http.StatusBadRequest {
		t.Fatalf("without file status = %d, want 400", withoutFile.Code)
	}
	if !strings.Contains(withoutFile.Body.String(), "file") {
		t.Fatalf("without file response = %s", withoutFile.Body.String())
	}
}

func TestModerationErrorsUseCodeMessageDataEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/error", func(c *gin.Context) {
		httpapi.WriteError(c, apperror.NewBusiness("AVATAR_REJECTED", http.StatusBadRequest, "头像未通过内容审核，请更换后再试", nil))
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/error", nil))
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["code"] != "AVATAR_REJECTED" || payload["message"] == "" {
		t.Fatalf("error payload = %#v", payload)
	}
	if _, ok := payload["data"]; !ok {
		t.Fatalf("error payload has no data field = %#v", payload)
	}
}

func TestNoBackendSourceContainsKnownSecretAssignments(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	var suspicious []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".go" && ext != ".yaml" && ext != ".yml" && ext != ".env" && ext != ".md" {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(path), "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		content := string(data)
		for _, marker := range []string{
			"eyJhbGciOiJIUzI1NiIs", "GO_SERVICE_WECHAT_APP_SECRET=\"", "GO_SERVICE_JWT_SECRET=\"ey",
		} {
			if strings.Contains(content, marker) {
				suspicious = append(suspicious, path+":"+marker)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(suspicious) != 0 {
		t.Fatalf("backend contains suspicious secret assignments: %v", suspicious)
	}
}
