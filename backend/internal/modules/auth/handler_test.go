package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterHandlerBadRequest(t *testing.T) {
	handler := NewHandler(newTestService(&fakeUserStore{}, &fakeTokenStore{}))
	router := gin.New()
	router.POST("/register", handler.Register)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{"username":"a"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestRegisterHandlerSuccess(t *testing.T) {
	handler := NewHandler(newTestService(&fakeUserStore{}, &fakeTokenStore{}))
	router := gin.New()
	router.POST("/register", handler.Register)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{"username":"alice","password":"password123"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}
}
