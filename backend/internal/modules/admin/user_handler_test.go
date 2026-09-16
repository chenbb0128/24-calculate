package admin

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/example/go-service/internal/config"
	jwtplatform "github.com/example/go-service/internal/platform/jwt"
)

func TestAdminUserHandlerUsesStandardSuccessAndErrorEnvelopes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := NewAdminUserService(&fakeAdminUserStore{users: []AdminUserRecord{{ID: 4, Username: "alice", Platform: "wechat", Status: 1}}}, &fakeAccountBlocker{}, time.Minute)
	handler := NewAdminUserHandler(service)
	router := gin.New()
	router.GET("/users", handler.List)
	router.GET("/users/:id", handler.Detail)

	success := httptest.NewRecorder()
	router.ServeHTTP(success, httptest.NewRequest(http.MethodGet, "/users?page=1&page_size=20", nil))
	assertAdminEnvelope(t, success, http.StatusOK, 0)

	missing := httptest.NewRecorder()
	router.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/users/99", nil))
	assertAdminEnvelope(t, missing, http.StatusNotFound, 10002)
}

func TestAdminUserRoutesRejectNonAdminAndBlockedAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager := newAdminUserJWTManager(t)
	service := NewAdminUserService(&fakeAdminUserStore{}, &fakeAccountBlocker{}, time.Minute)
	handler := NewAdminUserHandler(service)
	router := gin.New()
	group := router.Group("/api/v1")
	checker := fakeAdminRouteChecker{}
	RegisterUserRoutes(group, handler, manager, checker)

	userToken, _, err := manager.IssueAccessTokenWithRole(7, jwtplatform.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	wrongRole := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	request.Header.Set("Authorization", "Bearer "+userToken)
	router.ServeHTTP(wrongRole, request)
	assertAdminEnvelope(t, wrongRole, http.StatusForbidden, 10004)

	blockedToken, _, err := manager.IssueAccessTokenWithRole(9, jwtplatform.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	blockedRouter := gin.New()
	blockedGroup := blockedRouter.Group("/api/v1")
	RegisterUserRoutes(blockedGroup, handler, manager, fakeAdminRouteChecker{blocked: true})
	blocked := httptest.NewRecorder()
	blockedRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	blockedRequest.Header.Set("Authorization", "Bearer "+blockedToken)
	blockedRouter.ServeHTTP(blocked, blockedRequest)
	assertAdminEnvelope(t, blocked, http.StatusForbidden, 10004)
}

func assertAdminEnvelope(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus, wantCode int) {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if recorder.Code != wantStatus || int(body["code"].(float64)) != wantCode {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
	if _, ok := body["data"]; !ok {
		t.Fatalf("response has no data envelope: %s", recorder.Body.String())
	}
}

func newAdminUserJWTManager(t *testing.T) *jwtplatform.Manager {
	t.Helper()
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	manager, err := jwtplatform.NewManager(config.JWTConfig{
		Secret: string(secret), Algorithm: "HS256", Issuer: "test", AccessTTL: time.Minute, RefreshTTL: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

type fakeAdminRouteChecker struct{ blocked bool }

func (fakeAdminRouteChecker) IsAccessTokenRevoked(context.Context, string) (bool, error) {
	return false, nil
}

func (checker fakeAdminRouteChecker) IsAccountBlocked(context.Context, string, uint64) (bool, error) {
	return checker.blocked, nil
}
