package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/example/go-service/internal/apperror"
	"github.com/example/go-service/internal/platform/requestid"
)

type ErrorResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Data      any    `json:"data"`
}

// WriteError keeps the HTTP response shape separate from the application
// error implementation. Business packages can return AppError without
// importing Gin or any HTTP package.
func WriteError(c *gin.Context, err error) {
	if err != nil {
		_ = c.Error(err)
	}
	appErr := apperror.From(err)
	status := appErr.HTTPStatus
	if status < http.StatusBadRequest || status > http.StatusInternalServerError+99 {
		status = http.StatusInternalServerError
	}

	c.JSON(status, ErrorResponse{
		Code: appErr.Code, Message: appErr.Message,
		RequestID: requestid.FromContext(c.Request.Context()), Data: nil,
	})
}
