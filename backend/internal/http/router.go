package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/example/go-service/internal/apperror"
	"github.com/example/go-service/internal/config"
	"github.com/example/go-service/internal/http/middleware"
	"github.com/example/go-service/internal/http/response"
)

type ReadinessChecker interface {
	Check(context.Context) error
}

type ReadinessCheckerFunc func(context.Context) error

func (f ReadinessCheckerFunc) Check(ctx context.Context) error {
	return f(ctx)
}

type RouterOptions struct {
	Readiness        ReadinessChecker
	APIRoutes        func(*gin.RouterGroup)
	AvatarStorageDir string
	Metrics          *Metrics
}

func NewRouter(cfg *config.Config, logger *slog.Logger, options RouterOptions) (*gin.Engine, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if options.Readiness == nil {
		options.Readiness = ReadinessCheckerFunc(func(context.Context) error {
			return errors.New("readiness checker is not configured")
		})
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.RedirectTrailingSlash = false
	router.HandleMethodNotAllowed = true
	if err := router.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		return nil, errors.New("configure trusted proxies: " + err.Error())
	}

	metrics := options.Metrics
	if metrics == nil {
		metrics = NewMetrics()
	}
	router.Use(
		middleware.RequestID(),
		middleware.AccessLog(logger),
		middleware.Recovery(logger),
		middleware.Security(cfg),
		middleware.CORS(cfg),
		metrics.Middleware(),
	)

	router.GET("/health", func(c *gin.Context) {
		response.Success(c, http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/ready", func(c *gin.Context) {
		if err := options.Readiness.Check(c.Request.Context()); err != nil {
			logger.ErrorContext(c.Request.Context(), "readiness check failed", "error", err)
			response.WriteError(c, apperror.ServiceUnavailable("服务尚未就绪", err))
			return
		}
		response.Success(c, http.StatusOK, gin.H{"status": "ready"})
	})
	router.GET("/metrics", metrics.Handler)

	api := router.Group("/api/v1")
	if options.APIRoutes != nil {
		options.APIRoutes(api)
	}
	if !strings.EqualFold(cfg.App.Env, "production") {
		// net/http redirects requests for a file named index.html to the
		// containing directory. Serve the directory URL explicitly so that
		// /swagger/index.html and /swagger/ both resolve to the UI.
		router.GET("/swagger/", func(c *gin.Context) {
			c.File("docs/swagger.html")
		})
		router.StaticFile("/swagger/index.html", "docs/swagger.html")
		router.StaticFile("/swagger/openapi.yaml", "docs/openapi.yaml")
	}
	if strings.TrimSpace(options.AvatarStorageDir) != "" {
		router.Static("/avatars", filepath.Join(options.AvatarStorageDir, "avatars"))
	}

	router.NoRoute(func(c *gin.Context) {
		response.WriteError(c, apperror.NotFound("请求地址不存在", nil))
	})
	router.NoMethod(func(c *gin.Context) {
		response.WriteError(c, apperror.New(10005, http.StatusMethodNotAllowed, "请求方法不允许", nil))
	})

	return router, nil
}
