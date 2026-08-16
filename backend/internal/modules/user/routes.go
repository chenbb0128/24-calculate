package user

import (
	"github.com/gin-gonic/gin"

	"github.com/example/go-service/internal/http/middleware"
	jwtplatform "github.com/example/go-service/internal/platform/jwt"
)

func RegisterRoutes(group *gin.RouterGroup, handler *Handler, manager *jwtplatform.Manager) {
	routes := group.Group("/users")
	routes.Use(middleware.RequireAuth(manager))
	routes.GET("/me", handler.GetMe)
	routes.PATCH("/me", handler.UpdateMe)
}
