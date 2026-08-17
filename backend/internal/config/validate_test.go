package config

import "testing"

func validConfigForTest() Config {
	return Config{
		App: AppConfig{Name: "test", Env: "production"},
		Server: ServerConfig{
			Port: 8080, ReadHeaderTimeout: 1, ReadTimeout: 1, WriteTimeout: 1,
			IdleTimeout: 1, ShutdownTimeout: 1, MaxHeaderBytes: 1024, MaxRequestBodyBytes: 1024,
		},
		Database: DatabaseConfig{Host: "127.0.0.1", Port: 3306, User: "app", Password: "real-db-password", Name: "game", MaxOpenConns: 10, MaxIdleConns: 5},
		Redis:    RedisConfig{Addr: "127.0.0.1:6379"},
		WeChat:   WeChatConfig{AppID: "wx1e7ac815548c561c", AppSecret: "real-app-secret", APIBaseURL: "https://api.weixin.qq.com", Timeout: 1},
		JWT:      JWTConfig{Secret: "01234567890123456789012345678901", Algorithm: "HS256", AccessTTL: 1, RefreshTTL: 2},
		Queue:    QueueConfig{Name: "game", Concurrency: 1, TaskTimeout: 1},
		Game:     GameConfig{DailySeedSecret: "real-daily-seed-secret", MatchmakingWaitSeconds: 12},
		Log:      LogConfig{Level: "info"},
	}
}

func TestValidateProductionRejectsPlaceholderSecrets(t *testing.T) {
	fields := []struct {
		name  string
		apply func(*Config)
	}{
		{name: "database password", apply: func(cfg *Config) { cfg.Database.Password = "replace-with-production-db-password" }},
		{name: "wechat app secret", apply: func(cfg *Config) { cfg.WeChat.AppSecret = "replace-with-wechat-app-secret" }},
		{name: "jwt secret", apply: func(cfg *Config) { cfg.JWT.Secret = "replace-with-at-least-32-random-bytes" }},
		{name: "daily seed secret", apply: func(cfg *Config) { cfg.Game.DailySeedSecret = "replace-with-a-separate-daily-seed-secret" }},
	}
	for _, field := range fields {
		t.Run(field.name, func(t *testing.T) {
			cfg := validConfigForTest()
			field.apply(&cfg)
			if err := Validate(&cfg); err == nil {
				t.Fatalf("Validate() error = nil for placeholder %s", field.name)
			}
		})
	}
}

func TestValidateProductionAcceptsRealSecrets(t *testing.T) {
	if err := Validate(func() *Config {
		cfg := validConfigForTest()
		return &cfg
	}()); err != nil {
		t.Fatalf("Validate() error = %v for real production configuration", err)
	}
}

func TestValidateProductionRejectsMissingRequiredSecrets(t *testing.T) {
	fields := []struct {
		name  string
		apply func(*Config)
	}{
		{name: "database password", apply: func(cfg *Config) { cfg.Database.Password = "" }},
		{name: "wechat app secret", apply: func(cfg *Config) { cfg.WeChat.AppSecret = "" }},
		{name: "jwt secret", apply: func(cfg *Config) { cfg.JWT.Secret = "" }},
		{name: "daily seed secret", apply: func(cfg *Config) { cfg.Game.DailySeedSecret = "" }},
	}
	for _, field := range fields {
		t.Run(field.name, func(t *testing.T) {
			cfg := validConfigForTest()
			field.apply(&cfg)
			if err := Validate(&cfg); err == nil {
				t.Fatalf("Validate() error = nil for missing %s", field.name)
			}
		})
	}
}
