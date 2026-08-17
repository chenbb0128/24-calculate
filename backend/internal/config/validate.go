package config

import (
	"fmt"
	"strings"
)

func Validate(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if strings.TrimSpace(cfg.App.Name) == "" {
		return fmt.Errorf("app.name must not be empty")
	}

	switch strings.ToLower(strings.TrimSpace(cfg.App.Env)) {
	case "development", "test", "staging", "production":
	default:
		return fmt.Errorf("app.env must be one of development, test, staging, production")
	}

	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535")
	}
	if cfg.Server.ReadHeaderTimeout <= 0 || cfg.Server.ReadTimeout <= 0 || cfg.Server.WriteTimeout <= 0 || cfg.Server.IdleTimeout <= 0 {
		return fmt.Errorf("server timeouts must be greater than zero")
	}
	if cfg.Server.ShutdownTimeout <= 0 {
		return fmt.Errorf("server.shutdown_timeout must be greater than zero")
	}
	if cfg.Server.MaxHeaderBytes <= 0 {
		return fmt.Errorf("server.max_header_bytes must be greater than zero")
	}
	if cfg.Server.MaxRequestBodyBytes <= 0 {
		return fmt.Errorf("server.max_request_body_bytes must be greater than zero")
	}

	if strings.TrimSpace(cfg.Database.Host) == "" {
		return fmt.Errorf("database.host must not be empty")
	}
	if cfg.Database.Port < 1 || cfg.Database.Port > 65535 {
		return fmt.Errorf("database.port must be between 1 and 65535")
	}
	if strings.TrimSpace(cfg.Database.User) == "" || strings.TrimSpace(cfg.Database.Name) == "" {
		return fmt.Errorf("database.user and database.name must not be empty")
	}

	if strings.TrimSpace(cfg.Redis.Addr) == "" {
		return fmt.Errorf("redis.addr must not be empty")
	}
	if strings.TrimSpace(cfg.WeChat.APIBaseURL) == "" || cfg.WeChat.Timeout <= 0 {
		return fmt.Errorf("wechat api configuration is invalid")
	}
	if strings.EqualFold(cfg.App.Env, "production") && (strings.TrimSpace(cfg.WeChat.AppID) == "" || strings.TrimSpace(cfg.WeChat.AppSecret) == "") {
		return fmt.Errorf("wechat.app_id and wechat.app_secret must be provided in production")
	}

	if len([]byte(cfg.JWT.Secret)) < 32 {
		return fmt.Errorf("jwt.secret must be at least 32 bytes and must be provided through the environment")
	}
	if cfg.JWT.Algorithm != "HS256" {
		return fmt.Errorf("jwt.algorithm must be HS256")
	}
	if cfg.JWT.AccessTTL <= 0 || cfg.JWT.RefreshTTL <= 0 || cfg.JWT.RefreshTTL <= cfg.JWT.AccessTTL {
		return fmt.Errorf("jwt token TTL values are invalid")
	}

	if cfg.Database.MaxOpenConns <= 0 || cfg.Database.MaxIdleConns < 0 {
		return fmt.Errorf("database connection pool values are invalid")
	}
	if strings.TrimSpace(cfg.Queue.Name) == "" || cfg.Queue.Concurrency <= 0 {
		return fmt.Errorf("queue.name and queue.concurrency are invalid")
	}
	if cfg.Queue.TaskTimeout <= 0 || cfg.Queue.MaxRetry < 0 {
		return fmt.Errorf("queue task settings are invalid")
	}
	if cfg.Game.MatchmakingWaitSeconds < 1 || cfg.Game.MatchmakingWaitSeconds > 60 {
		return fmt.Errorf("game.matchmaking_wait_seconds must be between 1 and 60")
	}
	if cfg.Log.Level == "" {
		return fmt.Errorf("log.level must not be empty")
	}
	if strings.EqualFold(cfg.App.Env, "production") {
		productionSecrets := map[string]string{
			"database.password":      cfg.Database.Password,
			"wechat.app_secret":      cfg.WeChat.AppSecret,
			"jwt.secret":             cfg.JWT.Secret,
			"game.daily_seed_secret": cfg.Game.DailySeedSecret,
		}
		for name, value := range productionSecrets {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("%s must be provided in production", name)
			}
			if isPlaceholderSecret(value) {
				return fmt.Errorf("%s must contain a real production secret", name)
			}
		}
		for _, origin := range cfg.Server.CORSAllowedOrigins {
			if strings.TrimSpace(origin) == "*" {
				return fmt.Errorf("wildcard CORS is not allowed in production")
			}
		}
	}

	return nil
}

func isPlaceholderSecret(value string) bool {
	s := strings.ToLower(strings.TrimSpace(value))
	if s == "" {
		return false
	}
	for _, marker := range []string{
		"replace-with-", "replace_with_", "change-me", "change_me", "changeme",
		"your-secret", "your_secret", "your-password", "your_password",
		"your-app-secret", "your_app_secret", "<secret>", "<password>",
	} {
		if strings.Contains(s, marker) {
			return true
		}
	}
	return false
}
