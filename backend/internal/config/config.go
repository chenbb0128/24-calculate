package config

import (
	"net"
	"strconv"
	"time"
)

type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	WeChat   WeChatConfig   `mapstructure:"wechat"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Queue    QueueConfig    `mapstructure:"queue"`
	Game     GameConfig     `mapstructure:"game"`
	Avatar   AvatarConfig   `mapstructure:"avatar"`
	Log      LogConfig      `mapstructure:"log"`
}

type AppConfig struct {
	Name string `mapstructure:"name"`
	Env  string `mapstructure:"env"`
}

type ServerConfig struct {
	Host                string        `mapstructure:"host"`
	Port                int           `mapstructure:"port"`
	ReadHeaderTimeout   time.Duration `mapstructure:"read_header_timeout"`
	ReadTimeout         time.Duration `mapstructure:"read_timeout"`
	WriteTimeout        time.Duration `mapstructure:"write_timeout"`
	IdleTimeout         time.Duration `mapstructure:"idle_timeout"`
	ShutdownTimeout     time.Duration `mapstructure:"shutdown_timeout"`
	MaxHeaderBytes      int           `mapstructure:"max_header_bytes"`
	MaxRequestBodyBytes int64         `mapstructure:"max_request_body_bytes"`
	TrustedProxies      []string      `mapstructure:"trusted_proxies"`
	CORSAllowedOrigins  []string      `mapstructure:"cors_allowed_origins"`
}

func (c ServerConfig) Address() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Name            string        `mapstructure:"name"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
	ConnectTimeout  time.Duration `mapstructure:"connect_timeout"`
}

type RedisConfig struct {
	Addr         string        `mapstructure:"addr"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type WeChatConfig struct {
	AppID      string        `mapstructure:"app_id"`
	AppSecret  string        `mapstructure:"app_secret"`
	APIBaseURL string        `mapstructure:"api_base_url"`
	Timeout    time.Duration `mapstructure:"timeout"`
}

type JWTConfig struct {
	Secret     string        `mapstructure:"secret"`
	Algorithm  string        `mapstructure:"algorithm"`
	Issuer     string        `mapstructure:"issuer"`
	AccessTTL  time.Duration `mapstructure:"access_ttl"`
	RefreshTTL time.Duration `mapstructure:"refresh_ttl"`
}

type QueueConfig struct {
	Name        string        `mapstructure:"name"`
	Concurrency int           `mapstructure:"concurrency"`
	TaskTimeout time.Duration `mapstructure:"task_timeout"`
	MaxRetry    int           `mapstructure:"max_retry"`
}

type GameConfig struct {
	DailySeedSecret        string `mapstructure:"daily_seed_secret"`
	CampaignContentVersion string `mapstructure:"campaign_content_version"`
	CampaignContentSecret  string `mapstructure:"campaign_content_secret"`
	MatchmakingWaitSeconds int    `mapstructure:"matchmaking_wait_seconds"`
	RankSeasonID           string `mapstructure:"rank_season_id"`
}

type AvatarConfig struct {
	StorageDir            string `mapstructure:"storage_dir"`
	PublicBaseURL         string `mapstructure:"public_base_url"`
	MaxBytes              int64  `mapstructure:"max_bytes"`
	MaxDimension          int    `mapstructure:"max_dimension"`
	UploadCooldownSeconds int    `mapstructure:"upload_cooldown_seconds"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}
