package user

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	db "github.com/example/go-service/internal/store/sqlc"
)

func TestGetMeHandlerSuccess(t *testing.T) {
	service := NewService(&fakeStore{user: testUser()})
	handler := NewHandler(service)
	router := gin.New()
	router.GET("/me", func(c *gin.Context) {
		c.Set("auth.user_id", uint64(7))
		handler.GetMe(c)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/me", nil))

	if recorder.Code != http.StatusOK || strings.Contains(recorder.Body.String(), "password_hash") {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestUpdateMeHandlerBadRequest(t *testing.T) {
	service := NewService(&fakeStore{user: testUser()})
	handler := NewHandler(service)
	router := gin.New()
	router.PATCH("/me", func(c *gin.Context) {
		c.Set("auth.user_id", uint64(7))
		handler.UpdateMe(c)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/me", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func testUser() db.User {
	return db.User{ID: 7, Username: "alice", Status: StatusActive}
}
