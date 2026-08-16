package middleware

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/example/go-service/internal/apperror"
	"github.com/example/go-service/internal/http/response"
	jwtplatform "github.com/example/go-service/internal/platform/jwt"
)

const userIDKey = "auth.user_id"

func RequireAuth(manager *jwtplatform.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		value := c.GetHeader("Authorization")
		parts := strings.Fields(value)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.WriteError(c, apperror.New(20001, http.StatusUnauthorized, "Token 缺失", nil))
			c.Abort()
			return
		}

		claims, err := manager.ParseAccessToken(parts[1])
		if errors.Is(err, jwtplatform.ErrTokenExpired) {
			response.WriteError(c, apperror.New(20003, http.StatusUnauthorized, "Token 已过期", err))
			c.Abort()
			return
		}
		if err != nil {
			response.WriteError(c, apperror.New(20002, http.StatusUnauthorized, "Token 无效", err))
			c.Abort()
			return
		}

		c.Set(userIDKey, claims.UserID)
		c.Next()
	}
}

func UserID(c *gin.Context) (uint64, error) {
	value, exists := c.Get(userIDKey)
	if !exists {
		return 0, apperror.New(20002, http.StatusUnauthorized, "Token 无效", nil)
	}
	switch userID := value.(type) {
	case uint64:
		return userID, nil
	case string:
		parsed, err := strconv.ParseUint(userID, 10, 64)
		if err == nil && parsed > 0 {
			return parsed, nil
		}
	}
	return 0, apperror.New(20002, http.StatusUnauthorized, "Token 无效", nil)
}
