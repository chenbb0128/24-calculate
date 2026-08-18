package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type revocableTokenStore struct {
	fakeTokenStore
	revokedRefresh string
	revokedAccess  string
}

func (f *revocableTokenStore) RevokeRefreshToken(_ context.Context, jti string) error {
	f.revokedRefresh = jti
	return nil
}

func (f *revocableTokenStore) RevokeAccessToken(_ context.Context, jti string, _ time.Duration) error {
	f.revokedAccess = jti
	return nil
}

func TestLogoutRevokesBothTokensAndIsIdempotent(t *testing.T) {
	manager := newTestJWTManager(t)
	accessToken, accessClaims, err := manager.IssueAccessToken(7)
	if err != nil {
		t.Fatal(err)
	}
	refreshToken, refreshClaims, err := manager.IssueRefreshToken(7)
	if err != nil {
		t.Fatal(err)
	}
	tokens := &revocableTokenStore{}
	service := NewService(&fakeUserStore{}, tokens, manager, time.Minute, time.Hour, nil, nil)
	handler := NewHandler(service)
	router := gin.New()
	router.POST("/logout", handler.Logout)

	call := func() *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/logout", strings.NewReader(`{"refresh_token":"`+refreshToken+`"}`))
		request.Header.Set("Authorization", "Bearer "+accessToken)
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		return recorder
	}
	if recorder := call(); recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"code":0`) {
		t.Fatalf("logout response = %d %s", recorder.Code, recorder.Body.String())
	}
	if tokens.revokedAccess != accessClaims.ID || tokens.revokedRefresh != refreshClaims.ID {
		t.Fatalf("revocation state = %+v", tokens)
	}
	if recorder := call(); recorder.Code != http.StatusOK {
		t.Fatalf("repeated logout status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}
