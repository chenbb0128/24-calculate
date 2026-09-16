package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAdminHandlerMalformedJSONUsesBadRequestEnvelope(t *testing.T) {
	handler := newAdminHandler(t)
	router := gin.New()
	router.POST("/login", handler.Login)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"username":`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":10001`) {
		t.Fatalf("malformed JSON response = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestAdminHandlerLoginReturnsTokenResponseEnvelope(t *testing.T) {
	handler := newAdminHandler(t)
	router := gin.New()
	router.POST("/login", handler.Login)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"username":"admin","password":"correct-password"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(body, `"code":0`) || !strings.Contains(body, `"access_token"`) || !strings.Contains(body, `"refresh_token"`) || !strings.Contains(body, `"token_type":"Bearer"`) || !strings.Contains(body, `"expires_in":60`) {
		t.Fatalf("login response does not use the token response envelope")
	}
}

func newAdminHandler(t *testing.T) *AdminAuthHandler {
	t.Helper()
	service, _, _ := newAdminAuthService(t, activeAdmin(t, 7, "correct-password"))
	return NewAdminAuthHandler(service)
}
