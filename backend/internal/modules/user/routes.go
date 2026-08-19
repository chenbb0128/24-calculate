package user

import (
	"github.com/gin-gonic/gin"

	"github.com/example/go-service/internal/http/middleware"
	jwtplatform "github.com/example/go-service/internal/platform/jwt"
)

func RegisterRoutes(group *gin.RouterGroup, handler *Handler, manager *jwtplatform.Manager, revocation ...middleware.AccessTokenRevocationChecker) {
	routes := group.Group("/users")
	routes.Use(middleware.RequireAuth(manager, revocation...))
	routes.GET("/me", handler.GetMe)
	routes.PATCH("/me", handler.UpdateMe)
	routes.POST("/me/avatar", handler.UploadAvatar)
}
