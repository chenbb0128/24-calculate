package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/example/go-service/internal/config"
	"github.com/example/go-service/internal/platform/jwt"
)

type revokedTokenChecker struct {
	revoked bool
	blocked bool
}

func (f revokedTokenChecker) IsAccessTokenRevoked(context.Context, string) (bool, error) {
	return f.revoked, nil
}

func (f revokedTokenChecker) IsAccountBlocked(context.Context, string, uint64) (bool, error) {
	return f.blocked, nil
}

func TestRequireAuthRejectsRevokedAccessToken(t *testing.T) {
	manager, err := jwt.NewManager(config.JWTConfig{
		Secret: "01234567890123456789012345678901", Algorithm: "HS256", Issuer: "test",
		AccessTTL: time.Minute, RefreshTTL: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := manager.IssueAccessToken(9)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.GET("/protected", RequireAuth(manager, revokedTokenChecker{revoked: true}), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestRoleMiddleware(t *testing.T) {
	tests := []struct {
		name        string
		middleware  func(*jwt.Manager, ...AccessTokenRevocationChecker) gin.HandlerFunc
		issueRole   string
		wantStatus  int
		wantHandler bool
	}{
		{
			name:        "user token rejected by admin middleware",
			middleware:  RequireAdmin,
			issueRole:   jwt.RoleUser,
			wantStatus:  http.StatusForbidden,
			wantHandler: false,
		},
		{
			name:        "admin token rejected by user middleware",
			middleware:  RequireUser,
			issueRole:   jwt.RoleAdmin,
			wantStatus:  http.StatusForbidden,
			wantHandler: false,
		},
		{
			name:        "legacy empty role accepted by user middleware",
			middleware:  RequireUser,
			issueRole:   "",
			wantStatus:  http.StatusNoContent,
			wantHandler: true,
		},
		{
			name:        "admin token accepted by admin middleware",
			middleware:  RequireAdmin,
			issueRole:   jwt.RoleAdmin,
			wantStatus:  http.StatusNoContent,
			wantHandler: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := jwt.NewManager(config.JWTConfig{
				Secret: "01234567890123456789012345678901", Algorithm: "HS256", Issuer: "test",
				AccessTTL: time.Minute, RefreshTTL: time.Hour,
			})
			if err != nil {
				t.Fatal(err)
			}
			token, _, err := manager.IssueAccessTokenWithRole(9, tt.issueRole)
			if err != nil {
				t.Fatal(err)
			}

			handlerRan := false
			router := gin.New()
			router.GET("/protected", tt.middleware(manager), func(c *gin.Context) {
				handlerRan = true
				c.Status(http.StatusNoContent)
			})
			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			request.Header.Set("Authorization", "Bearer "+token)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			if handlerRan != tt.wantHandler {
				t.Fatalf("handler ran = %t, want %t", handlerRan, tt.wantHandler)
			}
		})
	}
}

func TestRequireUserChecksRoleQualifiedAccountBlock(t *testing.T) {
	manager, err := jwt.NewManager(config.JWTConfig{
		Secret: "01234567890123456789012345678901", Algorithm: "HS256", Issuer: "test",
		AccessTTL: time.Minute, RefreshTTL: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := manager.IssueAccessTokenWithRole(9, jwt.RoleUser)
	if err != nil {
		t.Fatal(err)
	}

	for _, check := range []struct {
		name       string
		blocked    bool
		wantStatus int
	}{
		{name: "blocked user", blocked: true, wantStatus: http.StatusForbidden},
		{name: "unblocked user", blocked: false, wantStatus: http.StatusNoContent},
	} {
		t.Run(check.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/protected", RequireUser(manager, revokedTokenChecker{blocked: check.blocked}), func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})
			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			request.Header.Set("Authorization", "Bearer "+token)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != check.wantStatus {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
