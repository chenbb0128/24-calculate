package admin

import "github.com/gin-gonic/gin"

func RegisterAuthRoutes(group *gin.RouterGroup, handler *AdminAuthHandler) {
	routes := group.Group("/admin/auth")
	routes.POST("/login", handler.Login)
	routes.POST("/refresh", handler.Refresh)
	routes.POST("/logout", handler.Logout)
}
