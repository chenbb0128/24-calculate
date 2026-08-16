package middleware

import (
	"log/slog"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/example/go-service/internal/apperror"
	"github.com/example/go-service/internal/http/response"
)

func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.ErrorContext(c.Request.Context(), "panic recovered", "panic", recovered, "stack", string(debug.Stack()))
				response.WriteError(c, apperror.Internal(nil))
				c.Abort()
			}
		}()

		c.Next()
	}
}
