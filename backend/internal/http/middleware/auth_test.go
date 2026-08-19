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

type revokedTokenChecker struct{ revoked bool }

func (f revokedTokenChecker) IsAccessTokenRevoked(context.Context, string) (bool, error) {
	return f.revoked, nil
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
