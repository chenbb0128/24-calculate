package moderation_test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"

	"github.com/example/go-service/internal/modules/moderation"
)

func TestModerationMigrationDeclaresRequiredColumns(t *testing.T) {
	data, err := os.ReadFile("../../../database/migrations/00013_create_user_moderation.sql")
	if err != nil {
		t.Fatalf("read moderation migration: %v", err)
	}
	content := strings.ToLower(string(data))
	for _, fragment := range []string{
		"-- +goose up", "-- +goose down", "nickname_moderation_status",
		"avatar_moderation_status", "moderation_updated_at", "user_moderation_events",
	} {
		if !strings.Contains(content, fragment) {
			t.Fatalf("migration is missing %q", fragment)
		}
	}
}

func TestModerationRepositoryPersistsAuditWithoutRawContent(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("GO_SERVICE_TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("GO_SERVICE_TEST_MYSQL_DSN is not configured")
	}
	database, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}

	store := moderation.NewSQLAuditStore(database)
	if err := store.RecordModerationEvent(context.Background(), moderation.AuditEvent{
		UserID: 1, ResourceType: "nickname", ResourceID: "",
		Source: "test", ModerationStatus: moderation.StatusRejected,
		ReasonCode: "87014", ProviderRequestID: "trace-test",
	}); err != nil {
		t.Fatalf("RecordModerationEvent() error = %v", err)
	}
	var status, reason, requestID string
	if err := database.QueryRowContext(context.Background(), `
SELECT moderation_status, reason_code, provider_request_id
FROM user_moderation_events
WHERE user_id = 1 AND source = 'test'
ORDER BY id DESC LIMIT 1`).Scan(&status, &reason, &requestID); err != nil {
		t.Fatal(err)
	}
	if status != string(moderation.StatusRejected) || reason != "87014" || requestID != "trace-test" {
		t.Fatalf("stored moderation event = %q, %q, %q", status, reason, requestID)
	}
}
