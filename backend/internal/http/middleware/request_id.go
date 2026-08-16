package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/example/go-service/internal/platform/requestid"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(requestid.HeaderName)
		if !requestid.IsValid(id) {
			id = requestid.New()
		}

		c.Request = c.Request.WithContext(requestid.WithContext(c.Request.Context(), id))
		c.Header(requestid.HeaderName, id)
		c.Next()
	}
}
