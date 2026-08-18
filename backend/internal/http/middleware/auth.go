package middleware

import (
	"context"
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
const accessClaimsKey = "auth.access_claims"

type AccessTokenRevocationChecker interface {
	IsAccessTokenRevoked(context.Context, string) (bool, error)
}

func RequireAuth(manager *jwtplatform.Manager, checkers ...AccessTokenRevocationChecker) gin.HandlerFunc {
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
		if len(checkers) > 0 && checkers[0] != nil {
			revoked, checkErr := checkers[0].IsAccessTokenRevoked(c.Request.Context(), claims.ID)
			if checkErr != nil {
				response.WriteError(c, apperror.ServiceUnavailable("认证服务暂时不可用", checkErr))
				c.Abort()
				return
			}
			if revoked {
				response.WriteError(c, apperror.New(20002, http.StatusUnauthorized, "Token 已失效", nil))
				c.Abort()
				return
			}
		}

		c.Set(userIDKey, claims.UserID)
		c.Set(accessClaimsKey, claims)
		c.Next()
	}
}

func AccessTokenClaims(c *gin.Context) (*jwtplatform.Claims, error) {
	value, exists := c.Get(accessClaimsKey)
	if !exists {
		return nil, apperror.New(20002, http.StatusUnauthorized, "Token 无效", nil)
	}
	claims, ok := value.(*jwtplatform.Claims)
	if !ok || claims == nil || claims.UserID == 0 || claims.ID == "" {
		return nil, apperror.New(20002, http.StatusUnauthorized, "Token 无效", nil)
	}
	return claims, nil
}

func BearerToken(c *gin.Context) (string, error) {
	parts := strings.Fields(c.GetHeader("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", apperror.New(20001, http.StatusUnauthorized, "Token 缺失", nil)
	}
	return parts[1], nil
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
