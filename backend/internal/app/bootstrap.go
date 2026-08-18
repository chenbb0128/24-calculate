package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/example/go-service/internal/config"
	httpapi "github.com/example/go-service/internal/http"
	"github.com/example/go-service/internal/modules/auth"
	"github.com/example/go-service/internal/modules/player"
	"github.com/example/go-service/internal/modules/user"
	jwtplatform "github.com/example/go-service/internal/platform/jwt"
	appLogger "github.com/example/go-service/internal/platform/logger"
	queueplatform "github.com/example/go-service/internal/platform/queue"
	redisplatform "github.com/example/go-service/internal/platform/redis"
	wechatplatform "github.com/example/go-service/internal/platform/wechat"
	"github.com/example/go-service/internal/store"
	db "github.com/example/go-service/internal/store/sqlc"
)

// BootstrapAPI is the composition root for the API process. External
// dependencies are added here in later stages, keeping cmd/api small.
type Runtime struct {
	Server *http.Server
	Logger *slog.Logger
	MySQL  *sql.DB
	Redis  *redisplatform.Client
	Queue  *queueplatform.Client
	Player *player.Service
}

func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	var closeErrs []error
	if r.Queue != nil {
		if err := r.Queue.Close(); err != nil {
			closeErrs = append(closeErrs, err)
		}
	}
	if r.Redis != nil {
		if err := r.Redis.Close(); err != nil {
			closeErrs = append(closeErrs, err)
		}
	}
	if r.MySQL != nil {
		if err := r.MySQL.Close(); err != nil {
			closeErrs = append(closeErrs, err)
		}
	}
	return errors.Join(closeErrs...)
}

func BootstrapAPI(cfg *config.Config) (*Runtime, error) {
	logger := appLogger.New(cfg.Log)
	database, err := store.OpenMySQL(cfg.Database)
	if err != nil {
		return nil, err
	}
	redisClient := redisplatform.New(cfg.Redis)
	redisCtx, cancel := context.WithTimeout(context.Background(), cfg.Redis.DialTimeout)
	defer cancel()
	if err := redisClient.Check(redisCtx); err != nil {
		_ = database.Close()
		_ = redisClient.Close()
		return nil, fmt.Errorf("check redis: %w", err)
	}
	manager, err := jwtplatform.NewManager(cfg.JWT)
	if err != nil {
		_ = database.Close()
		_ = redisClient.Close()
		return nil, err
	}
	queueClient := queueplatform.NewClient(cfg.Queue, cfg.Redis)

	queries := db.New(database)
	txManager := store.NewTxManager(database)
	userRepository := user.NewRepository(queries, txManager)
	authService := auth.NewServiceWithWeChat(userRepository, redisClient, manager, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL, queueClient, logger, wechatplatform.NewClient(cfg.WeChat))
	authHandler := auth.NewHandler(authService)
	avatarStorage := user.NewFileAvatarStorage(cfg.Avatar.StorageDir, cfg.Avatar.PublicBaseURL)
	userService := user.NewServiceWithAvatarStorage(userRepository, avatarStorage, cfg.Avatar.MaxBytes, cfg.Avatar.MaxDimension, time.Duration(cfg.Avatar.UploadCooldownSeconds)*time.Second)
	userService.SetAvatarRateLimiter(redisClient)
	userHandler := user.NewHandler(userService)
	playerRepository := player.NewRepository(queries, txManager)
	friendRoomRepository := player.NewFriendRoomRepository(redisClient, database)
	playerService := player.NewServiceWithRoomsAndEndless(userService, playerRepository, friendRoomRepository, friendRoomRepository)
	playerService.SetFriendHistoryStore(player.NewSQLFriendMatchHistoryRepository(database))
	rankRepository := player.NewSQLRankRepository(database)
	playerService.SetRankStore(rankRepository)
	if err := playerService.SetRankSeasonID(cfg.Game.RankSeasonID); err != nil {
		_ = database.Close()
		_ = queueClient.Close()
		_ = redisClient.Close()
		return nil, err
	}
	seedSecret := cfg.Game.DailySeedSecret
	if strings.TrimSpace(seedSecret) == "" {
		seedSecret = cfg.JWT.Secret
	}
	playerService.SetDailySeedSecret(seedSecret)
	playerService.SetMatchmakingWait(time.Duration(cfg.Game.MatchmakingWaitSeconds) * time.Second)
	playerHandler := player.NewHandler(playerService)

	router, err := httpapi.NewRouter(cfg, logger, httpapi.RouterOptions{
		AvatarStorageDir: cfg.Avatar.StorageDir,
		Readiness: dependencyReadiness{
			MySQL: store.MySQLReadiness{DB: database, Timeout: 2 * time.Second},
			Redis: redisplatform.Readiness{Client: redisClient, Timeout: 2 * time.Second},
		},
		APIRoutes: func(group *gin.RouterGroup) {
			auth.RegisterRoutes(group, authHandler, !strings.EqualFold(strings.TrimSpace(cfg.App.Env), "production"))
			user.RegisterRoutes(group, userHandler, manager, redisClient)
			player.RegisterRoutes(group, playerHandler, manager, redisClient)
		},
	})
	if err != nil {
		_ = database.Close()
		_ = queueClient.Close()
		_ = redisClient.Close()
		return nil, err
	}

	server := &http.Server{
		Addr:              cfg.Server.Address(),
		Handler:           router,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
		MaxHeaderBytes:    cfg.Server.MaxHeaderBytes,
	}

	return &Runtime{Server: server, Logger: logger, MySQL: database, Redis: redisClient, Queue: queueClient, Player: playerService}, nil
}

type dependencyReadiness struct {
	MySQL store.MySQLReadiness
	Redis redisplatform.Readiness
}

func (r dependencyReadiness) Check(ctx context.Context) error {
	if err := r.MySQL.Check(ctx); err != nil {
		return fmt.Errorf("mysql readiness: %w", err)
	}
	if err := r.Redis.Check(ctx); err != nil {
		return fmt.Errorf("redis readiness: %w", err)
	}
	return nil
}
