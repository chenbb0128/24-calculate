package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

const envPrefix = "GO_SERVICE"

// Load reads defaults, an optional YAML file, and environment overrides in
// that order, then validates the resulting strongly typed configuration.
func Load() (*Config, error) {
	v := viper.New()
	v.SetEnvPrefix(envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	setDefaults(v)
	bindEnvironment(v)

	if configFile := os.Getenv(envPrefix + "_CONFIG_FILE"); configFile != "" {
		v.SetConfigFile(configFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
		v.AddConfigPath(".")
	}

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg, viper.DecodeHook(mapstructure.StringToTimeDurationHookFunc())); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if err := Validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	defaults := map[string]any{
		"app.name": "go-service",
		"app.env":  "development",

		"server.host":                   "0.0.0.0",
		"server.port":                   8080,
		"server.read_header_timeout":    "5s",
		"server.read_timeout":           "15s",
		"server.write_timeout":          "15s",
		"server.idle_timeout":           "60s",
		"server.shutdown_timeout":       "10s",
		"server.max_header_bytes":       1 << 20,
		"server.max_request_body_bytes": 2 << 20,
		"server.trusted_proxies":        []string{},
		"server.cors_allowed_origins":   []string{},

		"database.host":               "127.0.0.1",
		"database.port":               3306,
		"database.user":               "root",
		"database.name":               "go_service",
		"database.max_open_conns":     25,
		"database.max_idle_conns":     10,
		"database.conn_max_lifetime":  "30m",
		"database.conn_max_idle_time": "5m",
		"database.connect_timeout":    "5s",

		"redis.addr":          "127.0.0.1:6379",
		"redis.db":            0,
		"redis.dial_timeout":  "5s",
		"redis.read_timeout":  "3s",
		"redis.write_timeout": "3s",

		"wechat.api_base_url": "https://api.weixin.qq.com",
		// AppID is public configuration; AppSecret remains environment-only.
		"wechat.app_id":  "wx1e7ac815548c561c",
		"wechat.timeout": "5s",

		"jwt.algorithm":   "HS256",
		"jwt.issuer":      "go-service",
		"jwt.access_ttl":  "15m",
		"jwt.refresh_ttl": "168h",

		"queue.name":         "default",
		"queue.concurrency":  10,
		"queue.task_timeout": "30s",
		"queue.max_retry":    5,

		"game.matchmaking_wait_seconds": 12,

		"log.level":  "info",
		"log.format": "json",
	}

	for key, value := range defaults {
		v.SetDefault(key, value)
	}
}

func bindEnvironment(v *viper.Viper) {
	keys := []string{
		"app.name", "app.env",
		"server.host", "server.port", "server.read_header_timeout", "server.read_timeout",
		"server.write_timeout", "server.idle_timeout", "server.shutdown_timeout",
		"server.max_header_bytes", "server.max_request_body_bytes", "server.trusted_proxies",
		"server.cors_allowed_origins",
		"database.host", "database.port", "database.user", "database.password", "database.name",
		"database.max_open_conns", "database.max_idle_conns", "database.conn_max_lifetime",
		"database.conn_max_idle_time", "database.connect_timeout",
		"redis.addr", "redis.password", "redis.db", "redis.dial_timeout", "redis.read_timeout",
		"redis.write_timeout",
		"wechat.app_id", "wechat.app_secret", "wechat.api_base_url", "wechat.timeout",
		"jwt.secret", "jwt.algorithm", "jwt.issuer", "jwt.access_ttl", "jwt.refresh_ttl",
		"queue.name", "queue.concurrency", "queue.task_timeout", "queue.max_retry",
		"game.daily_seed_secret", "game.matchmaking_wait_seconds",
		"log.level", "log.format",
	}

	for _, key := range keys {
		_ = v.BindEnv(key)
	}
}
