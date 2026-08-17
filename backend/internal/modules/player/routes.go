package player

import (
	"github.com/gin-gonic/gin"

	"github.com/example/go-service/internal/http/middleware"
	jwtplatform "github.com/example/go-service/internal/platform/jwt"
)

// RegisterRoutes registers player endpoints. The legacy completion endpoints
// accept client-provided scores and are only suitable for local development
// and compatibility testing; production must keep them disabled.
func RegisterRoutes(group *gin.RouterGroup, handler *Handler, manager *jwtplatform.Manager, allowLegacyCompletions bool) {
	routes := group.Group("/player")
	routes.Use(middleware.RequireAuth(manager))
	routes.GET("/bootstrap", handler.Bootstrap)
	routes.GET("/leaderboards/:mode", handler.Leaderboard)
	routes.POST("/endless/runs", handler.StartEndlessRun)
	routes.GET("/endless/runs/:run_id", handler.ResumeEndlessRun)
	routes.POST("/endless/runs/:run_id/submit", handler.SubmitEndlessRun)
	routes.POST("/campaign/runs", handler.StartCampaignRun)
	routes.GET("/campaign/runs/:run_id", handler.ResumeCampaignRun)
	routes.POST("/campaign/runs/:run_id/submit", handler.SubmitCampaignRun)
	routes.POST("/daily/runs", handler.StartDailyRun)
	routes.GET("/daily/runs/:run_id", handler.ResumeDailyRun)
	routes.POST("/daily/runs/:run_id/submit", handler.SubmitDailyRun)
	routes.POST("/matchmaking/join", handler.JoinMatchmaking)
	routes.GET("/matchmaking/status", handler.GetMatchmakingStatus)
	routes.POST("/matchmaking/cancel", handler.CancelMatchmaking)
	routes.POST("/friend/rooms", handler.CreateFriendRoom)
	routes.POST("/friend/rooms/:room_code/join", handler.JoinFriendRoom)
	routes.GET("/friend/rooms/:room_code", handler.GetFriendRoom)
	routes.DELETE("/friend/rooms/:room_code", handler.LeaveFriendRoom)
	routes.POST("/friend/rooms/:room_code/ready", handler.ReadyFriendRoom)
	routes.POST("/friend/rooms/:room_code/start", handler.StartFriendRoom)
	routes.POST("/friend/rooms/:room_code/match/progress", handler.UpdateFriendMatchProgress)
	routes.GET("/friend/rooms/:room_code/match/progress", handler.GetFriendMatchProgress)
	routes.POST("/friend/rooms/:room_code/match/submit", handler.SubmitFriendMatch)
	if allowLegacyCompletions {
		routes.POST("/levels/:level_id/complete", handler.CompleteLevel)
		routes.POST("/daily/complete", handler.CompleteDaily)
	}
	routes.POST("/skins/:skin_id/purchase", handler.PurchaseSkin)
	routes.POST("/skins/:skin_id/equip", handler.EquipSkin)
	routes.POST("/cosmetics/:cosmetic_id/purchase", handler.PurchaseCosmetic)
	routes.POST("/cosmetics/:cosmetic_id/equip", handler.EquipCosmetic)
	routes.PATCH("/preferences", handler.UpdatePreferences)
}
