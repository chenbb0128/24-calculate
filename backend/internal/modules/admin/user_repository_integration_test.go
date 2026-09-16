//go:build integration

package admin

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	db "github.com/example/go-service/internal/store/sqlc"
)

func TestAdminUserRepositoryPrefersWeChatWhenUserHasMultipleIdentities(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("GO_SERVICE_TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("GO_SERVICE_TEST_MYSQL_DSN is not configured")
	}
	database, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	if err := database.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	username := fmt.Sprintf("integration_admin_platform_%d", time.Now().UnixNano())
	result, err := database.ExecContext(ctx, `
INSERT INTO users (
    username, password_hash, nickname, avatar, status,
    nickname_moderation_status, avatar_moderation_status,
    created_at, updated_at
) VALUES (?, SHA2(UUID(), 256), '', '', 1, 'approved', 'approved', NOW(3), NOW(3))`, username)
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	defer database.ExecContext(ctx, "DELETE FROM users WHERE id = ?", id)

	for _, provider := range []string{"password", "taptap", "wechat"} {
		if _, err := database.ExecContext(ctx, `
INSERT INTO user_identities (user_id, provider, provider_subject, created_at, updated_at)
VALUES (?, ?, ?, NOW(3), NOW(3))`, id, provider, fmt.Sprintf("%s-%d", provider, id)); err != nil {
			t.Fatal(err)
		}
	}

	repository := NewRepository(db.New(database))
	account, err := repository.GetAdminUser(ctx, uint64(id))
	if err != nil {
		t.Fatal(err)
	}
	if account.Platform != "wechat" {
		t.Fatalf("platform = %q, want wechat when wechat+taptap+password identities exist", account.Platform)
	}
}
