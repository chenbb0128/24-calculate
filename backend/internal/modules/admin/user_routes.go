package admin

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/example/go-service/internal/apperror"
	"github.com/example/go-service/internal/http/middleware"
	"github.com/example/go-service/internal/http/response"
	jwtplatform "github.com/example/go-service/internal/platform/jwt"
)

type adminAccountBlockChecker interface {
	IsAccountBlocked(context.Context, string, uint64) (bool, error)
}

func RegisterUserRoutes(group *gin.RouterGroup, handler *AdminUserHandler, manager *jwtplatform.Manager, checkers ...middleware.AccessTokenRevocationChecker) {
	routes := group.Group("/admin/users")
	routes.Use(middleware.RequireAdmin(manager, checkers...))
	routes.Use(requireAdminAccountNotBlocked(checkers...))
	routes.GET("", handler.List)
	routes.GET("/:id", handler.Detail)
	routes.POST("/:id/disable", handler.Disable)
	routes.POST("/:id/enable", handler.Enable)
}

func requireAdminAccountNotBlocked(checkers ...middleware.AccessTokenRevocationChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(checkers) == 0 || checkers[0] == nil {
			return
		}
		blocker, ok := checkers[0].(adminAccountBlockChecker)
		if !ok {
			return
		}
		claims, err := middleware.AccessTokenClaims(c)
		if err != nil {
			response.WriteError(c, err)
			c.Abort()
			return
		}
		blocked, err := blocker.IsAccountBlocked(c.Request.Context(), claims.Role, claims.UserID)
		if err != nil {
			response.WriteError(c, apperror.ServiceUnavailable("认证服务暂时不可用", err))
			c.Abort()
			return
		}
		if blocked {
			response.WriteError(c, apperror.New(apperror.CodeForbidden, http.StatusForbidden, "账号已被禁用", nil))
			c.Abort()
			return
		}
	}
}
