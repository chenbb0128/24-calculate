package httpapi

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// RegisterAdminStatic mounts the built Vue admin application below /admin.
// The directory is intentionally configured outside RouterOptions so the
// generic API router remains independent from the optional dashboard asset.
func RegisterAdminStatic(router *gin.Engine, dir string) {
	if router == nil || strings.TrimSpace(dir) == "" {
		return
	}

	router.GET("/admin", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/admin/")
	})
	router.Static("/admin", filepath.Clean(dir))
}
