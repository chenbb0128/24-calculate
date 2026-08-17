package player

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterRoutesLegacyCompletionsAreOptional(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		allowLegacy   bool
		wantLegacyAPI bool
	}{
		{name: "production", allowLegacy: false, wantLegacyAPI: false},
		{name: "development compatibility", allowLegacy: true, wantLegacyAPI: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			group := router.Group("/api/v1")
			RegisterRoutes(group, NewHandler(nil), nil, tt.allowLegacy)

			if got := hasRoute(router, "POST", "/api/v1/player/levels/:level_id/complete"); got != tt.wantLegacyAPI {
				t.Fatalf("legacy campaign completion route registered = %v, want %v", got, tt.wantLegacyAPI)
			}
			if got := hasRoute(router, "POST", "/api/v1/player/daily/complete"); got != tt.wantLegacyAPI {
				t.Fatalf("legacy daily completion route registered = %v, want %v", got, tt.wantLegacyAPI)
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
		})
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
