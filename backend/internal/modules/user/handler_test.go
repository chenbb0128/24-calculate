package user

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestUploadAvatarHandlerSuccess(t *testing.T) {
	store := &fakeStore{user: testUser()}
	storage := &fakeAvatarStorage{avatar: StoredAvatar{Key: "avatars/7/new.webp", URL: "https://cdn.example.com/avatars/7/new.webp", Width: 256, Height: 256, Format: "webp"}}
	service := NewServiceWithAvatarStorage(store, storage, 2<<20, 4096, time.Minute)
	handler := NewHandler(service)
	router := gin.New()
	router.POST("/me/avatar", func(c *gin.Context) {
		c.Set("auth.user_id", uint64(7))
		handler.UploadAvatar(c)
	})

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("file", "avatar.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(testPNG(t, 300, 300)); err != nil {
		t.Fatal(err)
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/me/avatar", &body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "avatar_url") {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func testUser() db.User {
	return db.User{ID: 7, Username: "alice", Status: StatusActive}
}
