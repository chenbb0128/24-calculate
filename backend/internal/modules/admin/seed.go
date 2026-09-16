package admin

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	storepkg "github.com/example/go-service/internal/store"
	db "github.com/example/go-service/internal/store/sqlc"
)

func SeedAdmin(ctx context.Context, accountStore AdminAccountStore, username, password string) error {
	if len(username) < 3 || len(username) > 64 {
		return fmt.Errorf("admin username must be between 3 and 64 bytes")
	}
	if len(password) < 8 || len(password) > 72 {
		return fmt.Errorf("admin password must be between 8 and 72 bytes")
	}
	if accountStore == nil {
		return fmt.Errorf("admin account store is nil")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}
	now := time.Now().UTC()
	_, err = accountStore.CreateAdminAccount(ctx, db.CreateAdminAccountParams{
		Username:     username,
		PasswordHash: string(hash),
		Role:         "admin",
		Status:       1,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		if storepkg.IsDuplicateEntry(err) {
			return fmt.Errorf("admin username already exists")
		}
		return fmt.Errorf("create admin account: %w", err)
	}
	return nil
}
