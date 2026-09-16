package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	jwtplatform "github.com/example/go-service/internal/platform/jwt"
)

func TestAdminRoutesRegisterAndEnforceAdminRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager := newAdminUserJWTManager(t)
	userHandler := NewAdminUserHandler(NewAdminUserService(&fakeAdminUserStore{}, &fakeAccountBlocker{}, time.Minute))
	authHandler, _ := newAdminHandler(t)
	router := gin.New()
	group := router.Group("/api/v1")
	RegisterAuthRoutes(group, authHandler)
	RegisterUserRoutes(group, userHandler, manager, fakeAdminRouteChecker{})

	t.Run("login route is registered", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/login", strings.NewReader(`{}`))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		if recorder.Code == http.StatusNotFound {
			t.Fatalf("login route was not registered: %d %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("missing bearer is unauthorized", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil))
		assertAdminEnvelope(t, recorder, http.StatusUnauthorized, 20001)
	})

	t.Run("user role is forbidden", func(t *testing.T) {
		token, _, err := manager.IssueAccessTokenWithRole(7, jwtplatform.RoleUser)
		if err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
		request.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(recorder, request)
		assertAdminEnvelope(t, recorder, http.StatusForbidden, 10004)
	})

	t.Run("admin role reaches the list handler", func(t *testing.T) {
		token, _, err := manager.IssueAccessTokenWithRole(8, jwtplatform.RoleAdmin)
		if err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
		request.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(recorder, request)
		assertAdminEnvelope(t, recorder, http.StatusOK, 0)
	})
}
