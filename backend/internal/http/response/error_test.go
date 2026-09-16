package response

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/example/go-service/internal/apperror"
)

func TestWriteErrorSerializesBusinessCodeAsString(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/business", func(c *gin.Context) {
		WriteError(c, apperror.NewBusiness("NICKNAME_EDIT_DISABLED", http.StatusBadRequest, "昵称暂时无法修改", nil))
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/business", nil))
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":"NICKNAME_EDIT_DISABLED"`) {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestWriteErrorKeepsNumericCodeForExistingErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/bad-request", func(c *gin.Context) {
		WriteError(c, apperror.BadRequest("bad request", nil))
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/bad-request", nil))
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":10001`) {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}
