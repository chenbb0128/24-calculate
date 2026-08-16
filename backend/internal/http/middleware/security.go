package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/example/go-service/internal/config"
)

func Security(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Header("X-Permitted-Cross-Domain-Policies", "none")

		if cfg.Server.MaxRequestBodyBytes > 0 && c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, cfg.Server.MaxRequestBodyBytes)
		}

		c.Next()
	}
}
