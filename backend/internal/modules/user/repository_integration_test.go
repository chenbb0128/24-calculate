//go:build integration

package user

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/example/go-service/internal/store"
	db "github.com/example/go-service/internal/store/sqlc"
)

func TestRepositoryWithMySQL(t *testing.T) {
	dsn := os.Getenv("GO_SERVICE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("GO_SERVICE_TEST_MYSQL_DSN is not set")
	}

	database, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}

	queries := db.New(database)
	repository := NewRepository(queries, store.NewTxManager(database))
	username := fmt.Sprintf("integration_%d", time.Now().UnixNano())
	now := time.Now().UTC()
	id, err := repository.CreateUserTx(context.Background(), db.CreateUserParams{
		Username:     username,
		PasswordHash: "integration-hash",
		Status:       StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.ExecContext(context.Background(), "DELETE FROM users WHERE id = ?", id)

	created, err := repository.GetUserByID(context.Background(), id)
	if err != nil || created.Username != username {
		t.Fatalf("GetUserByID() = %+v, err = %v", created, err)
	}
	byName, err := repository.GetUserByUsername(context.Background(), username)
	if err != nil || byName.ID != id {
		t.Fatalf("GetUserByUsername() = %+v, err = %v", byName, err)
	}

	if err := repository.UpdateUserProfile(context.Background(), db.UpdateUserProfileParams{
		Nickname:  "integration",
		Avatar:    "avatar",
		UpdatedAt: time.Now().UTC(),
		ID:        id,
	}); err != nil {
		t.Fatal(err)
	}

	_, err = repository.CreateUserTx(context.Background(), db.CreateUserParams{
		Username:     username,
		PasswordHash: "integration-hash",
		Status:       StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if !store.IsDuplicateEntry(err) {
		t.Fatalf("duplicate insert error = %v, want MySQL duplicate entry", err)
	}
}
