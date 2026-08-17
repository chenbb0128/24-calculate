package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	goRedis "github.com/redis/go-redis/v9"

	"github.com/example/go-service/internal/config"
)

var ErrRefreshTokenNotFound = errors.New("refresh token not found")

type Client struct {
	*goRedis.Client
}

func New(cfg config.RedisConfig) *Client {
	return &Client{Client: goRedis.NewClient(&goRedis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})}
}

func (c *Client) Check(ctx context.Context) error {
	if c == nil || c.Client == nil {
		return errors.New("redis client is nil")
	}
	return c.Ping(ctx).Err()
}

func (c *Client) SaveRefreshToken(ctx context.Context, jti string, userID uint64, ttl time.Duration) error {
	if ttl <= 0 {
		return fmt.Errorf("refresh token TTL must be greater than zero")
	}
	return c.Set(ctx, RefreshTokenKey(jti), strconv.FormatUint(userID, 10), ttl).Err()
}

func (c *Client) ConsumeRefreshToken(ctx context.Context, jti string) (uint64, error) {
	value, err := consumeScript.Run(ctx, c.Client, []string{RefreshTokenKey(jti)}).Result()
	if err != nil {
		if errors.Is(err, goRedis.Nil) {
			return 0, ErrRefreshTokenNotFound
		}
		return 0, err
	}
	text, ok := value.(string)
	if !ok || text == "" {
		return 0, ErrRefreshTokenNotFound
	}
	userID, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse refresh token user id: %w", err)
	}
	return userID, nil
}

func (c *Client) RevokeRefreshToken(ctx context.Context, jti string) error {
	return c.Del(ctx, RefreshTokenKey(jti)).Err()
}

func (c *Client) AllowLogin(ctx context.Context, ip string, limit int64, window time.Duration) (bool, error) {
	if limit <= 0 || window <= 0 {
		return false, fmt.Errorf("login rate limit configuration is invalid")
	}
	key := LoginRateKey(ip)
	count, err := c.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if count == 1 {
		if err := c.Expire(ctx, key, window).Err(); err != nil {
			return false, err
		}
	}
	return count <= limit, nil
}

func (c *Client) AllowAvatarUpload(ctx context.Context, userID uint64, limit int64, window time.Duration) (bool, error) {
	if c == nil || c.Client == nil || userID == 0 || limit <= 0 || window <= 0 {
		return false, fmt.Errorf("avatar upload rate limit configuration is invalid")
	}
	key := AvatarUploadRateKey(userID)
	count, err := c.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if count == 1 {
		if err := c.Expire(ctx, key, window).Err(); err != nil {
			return false, err
		}
	}
	return count <= limit, nil
}

// AcquireDistributedLock uses a short-lived token so a late release cannot
// delete a lock acquired by another request after the original lock expired.
// The scope is hashed by DistributedLockKey and never appears verbatim in a
// Redis key.
func (c *Client) AcquireDistributedLock(ctx context.Context, scope string, ttl time.Duration) (string, bool, error) {
	if c == nil || c.Client == nil {
		return "", false, errors.New("redis client is nil")
	}
	if strings.TrimSpace(scope) == "" || ttl <= 0 {
		return "", false, errors.New("distributed lock scope and TTL are required")
	}
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", false, fmt.Errorf("generate distributed lock token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
	ok, err := c.SetNX(ctx, DistributedLockKey(scope), token, ttl).Result()
	if err != nil {
		return "", false, err
	}
	return token, ok, nil
}

func (c *Client) ReleaseDistributedLock(ctx context.Context, scope, token string) error {
	if c == nil || c.Client == nil {
		return errors.New("redis client is nil")
	}
	if strings.TrimSpace(scope) == "" || strings.TrimSpace(token) == "" {
		return errors.New("distributed lock scope and token are required")
	}
	return releaseLockScript.Run(ctx, c.Client, []string{DistributedLockKey(scope)}, token).Err()
}

type Readiness struct {
	Client  *Client
	Timeout time.Duration
}

func (r Readiness) Check(ctx context.Context) error {
	if r.Client == nil {
		return errors.New("redis client is nil")
	}
	checkCtx := ctx
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		checkCtx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}
	return r.Client.Check(checkCtx)
}

var consumeScript = goRedis.NewScript(`
local value = redis.call('GET', KEYS[1])
if value then
    redis.call('DEL', KEYS[1])
end
return value
`)

var releaseLockScript = goRedis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
    return redis.call('DEL', KEYS[1])
end
return 0
`)
