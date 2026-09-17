package redis

import (
	"context"
	"os"
	"testing"
	"time"

	goRedis "github.com/redis/go-redis/v9"
)

func TestClaimWeChatProfileCodeIsAtomic(t *testing.T) {
	if os.Getenv("GO_SERVICE_RUN_REDIS_TESTS") != "1" {
		t.Skip("set GO_SERVICE_RUN_REDIS_TESTS=1 to run Redis integration tests")
	}
	raw := goRedis.NewClient(&goRedis.Options{Addr: "127.0.0.1:6379"})
	t.Cleanup(func() { _ = raw.Close() })
	if err := raw.Ping(context.Background()).Err(); err != nil {
		t.Skipf("local Redis is unavailable: %v", err)
	}
	client := &Client{Client: raw}
	ctx := context.Background()
	hash := "profile-code-test-" + time.Now().UTC().Format("20060102150405.000000000")
	first, err := client.ClaimWeChatProfileCode(ctx, hash, time.Minute)
	if err != nil || !first {
		t.Fatalf("first claim = %v, %v", first, err)
	}
	second, err := client.ClaimWeChatProfileCode(ctx, hash, time.Minute)
	if err != nil || second {
		t.Fatalf("second claim = %v, %v; want false", second, err)
	}
	_ = client.Del(ctx, WeChatProfileCodeKey(hash)).Err()
}

func TestAllowWeChatProfileSyncHonorsLimit(t *testing.T) {
	if os.Getenv("GO_SERVICE_RUN_REDIS_TESTS") != "1" {
		t.Skip("set GO_SERVICE_RUN_REDIS_TESTS=1 to run Redis integration tests")
	}
	raw := goRedis.NewClient(&goRedis.Options{Addr: "127.0.0.1:6379"})
	t.Cleanup(func() { _ = raw.Close() })
	if err := raw.Ping(context.Background()).Err(); err != nil {
		t.Skipf("local Redis is unavailable: %v", err)
	}
	client := &Client{Client: raw}
	ctx := context.Background()
	userID := uint64(time.Now().UTC().UnixNano())
	first, err := client.AllowWeChatProfileSync(ctx, userID, 1, time.Minute)
	if err != nil || !first {
		t.Fatalf("first allow = %v, %v", first, err)
	}
	second, err := client.AllowWeChatProfileSync(ctx, userID, 1, time.Minute)
	if err != nil || second {
		t.Fatalf("second allow = %v, %v; want false", second, err)
	}
	_ = client.Del(ctx, WeChatProfileSyncRateKey(userID)).Err()
}
