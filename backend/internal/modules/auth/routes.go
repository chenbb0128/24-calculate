package auth

import "github.com/gin-gonic/gin"

func RegisterRoutes(group *gin.RouterGroup, handler *Handler, allowDevAuth bool) {
	routes := group.Group("/auth")
	routes.POST("/register", handler.Register)
	routes.POST("/login", handler.Login)
	routes.POST("/wechat-login", handler.WeChatLogin)
	if allowDevAuth {
		routes.POST("/dev-login", handler.DevLogin)
	}
	routes.POST("/refresh", handler.Refresh)
	routes.POST("/logout", handler.Logout)
}
