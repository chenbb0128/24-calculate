package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/example/go-service/internal/apperror"
	"github.com/example/go-service/internal/config"
	"github.com/example/go-service/internal/http/response"
)

func CORS(cfg *config.Config) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(cfg.Server.CORSAllowedOrigins))
	for _, origin := range cfg.Server.CORSAllowedOrigins {
		allowed[origin] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}

		if _, ok := allowed[origin]; !ok {
			if c.Request.Method == http.MethodOptions {
				response.WriteError(c, apperror.New(10004, http.StatusForbidden, "跨域请求来源不被允许", nil))
				c.Abort()
				return
			}
			c.Next()
			return
		}

		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
		c.Header("Vary", "Origin")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
