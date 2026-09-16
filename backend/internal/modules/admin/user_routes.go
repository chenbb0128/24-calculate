package admin

import (
	"github.com/gin-gonic/gin"

	"github.com/example/go-service/internal/http/middleware"
	jwtplatform "github.com/example/go-service/internal/platform/jwt"
)

func RegisterUserRoutes(group *gin.RouterGroup, handler *AdminUserHandler, manager *jwtplatform.Manager, checkers ...middleware.AccessTokenRevocationChecker) {
	routes := group.Group("/admin/users")
	routes.Use(middleware.RequireAdmin(manager, checkers...))
	routes.GET("", handler.List)
	routes.GET("/:id", handler.Detail)
	routes.POST("/:id/disable", handler.Disable)
	routes.POST("/:id/enable", handler.Enable)
}
