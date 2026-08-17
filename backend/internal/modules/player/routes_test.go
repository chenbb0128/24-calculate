package player

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterRoutesNeverExposeLegacyClientCompletions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	group := router.Group("/api/v1")
	RegisterRoutes(group, NewHandler(nil), nil)

	if hasRoute(router, "POST", "/api/v1/player/levels/:level_id/complete") {
		t.Fatal("legacy campaign completion route must never be registered")
	}
	if hasRoute(router, "POST", "/api/v1/player/daily/complete") {
		t.Fatal("legacy daily completion route must never be registered")
	}
	if hasRoute(router, "POST", "/api/v1/player/leaderboards/:mode/submit") {
		t.Fatal("client-controlled leaderboard submission route must not be registered")
	}
	if !hasRoute(router, "POST", "/api/v1/player/campaign/runs/:run_id/submit") {
		t.Fatal("server-validated campaign submit route was not registered")
	}
	if !hasRoute(router, "POST", "/api/v1/player/daily/runs/:run_id/submit") {
		t.Fatal("server-validated daily submit route was not registered")
	}
}

func hasRoute(router *gin.Engine, method, path string) bool {
	for _, route := range router.Routes() {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}
