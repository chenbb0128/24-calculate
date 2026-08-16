package middleware

import (
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/example/go-service/internal/apperror"
	"github.com/example/go-service/internal/platform/requestid"
)

func AccessLog(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		attrs := []any{
			"request_id", requestid.FromContext(c.Request.Context()),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency", time.Since(start).String(),
			"client_ip", c.ClientIP(),
			"user_agent", c.Request.UserAgent(),
			"user_id", userID(c),
		}
		if last := c.Errors.Last(); last != nil {
			logErr := last.Err
			var appErr *apperror.AppError
			if errors.As(last.Err, &appErr) && appErr.Err != nil {
				logErr = appErr.Err
			}
			attrs = append(attrs, "error", logErr)
		}

		logger.InfoContext(c.Request.Context(), "http request", attrs...)
	}
}

func userID(c *gin.Context) string {
	value, ok := c.Get(userIDKey)
	if !ok {
		return ""
	}
	if id, ok := value.(uint64); ok {
		return strconv.FormatUint(id, 10)
	}
	return ""
}
