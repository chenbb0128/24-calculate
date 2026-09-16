package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAdminHandlerMalformedJSONUsesBadRequestEnvelope(t *testing.T) {
	handler, _ := newAdminHandler(t)
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
	handler, password := newAdminHandler(t)
	router := gin.New()
	router.POST("/login", handler.Login)
	recorder := httptest.NewRecorder()
	body := `{"username":"admin","password":"` + password + `"}`
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	responseBody := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(responseBody, `"code":0`) || !strings.Contains(responseBody, `"access_token"`) || !strings.Contains(responseBody, `"refresh_token"`) || !strings.Contains(responseBody, `"token_type":"Bearer"`) || !strings.Contains(responseBody, `"expires_in":60`) {
		t.Fatalf("login response does not use the token response envelope")
	}
}

func newAdminHandler(t *testing.T) (*AdminAuthHandler, string) {
	t.Helper()
	password := generatedPassword(t)
	service, _, _ := newAdminAuthService(t, activeAdmin(t, 7, password))
	return NewAdminAuthHandler(service), password
}
