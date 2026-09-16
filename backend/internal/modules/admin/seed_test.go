package admin

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	db "github.com/example/go-service/internal/store/sqlc"
)

type fakeAdminAccountStore struct {
	created db.CreateAdminAccountParams
	err     error
}

func (s *fakeAdminAccountStore) CreateAdminAccount(_ context.Context, arg db.CreateAdminAccountParams) (sql.Result, error) {
	s.created = arg
	return nil, s.err
}

func TestSeedAdminCreatesBcryptHash(t *testing.T) {
	store := &fakeAdminAccountStore{}
	password := "correct horse battery staple"

	if err := SeedAdmin(context.Background(), store, "admin", password); err != nil {
		t.Fatalf("SeedAdmin() error = %v", err)
	}
	if store.created.Username != "admin" {
		t.Fatalf("username = %q, want admin", store.created.Username)
	}
	if store.created.Role != "admin" || store.created.Status != 1 {
		t.Fatalf("account defaults = role %q status %d, want admin/1", store.created.Role, store.created.Status)
	}
	if store.created.PasswordHash == password {
		t.Fatal("stored password is plaintext")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(store.created.PasswordHash), []byte(password)); err != nil {
		t.Fatalf("stored password is not a bcrypt hash of the password: %v", err)
	}
	if store.created.CreatedAt.Location() != time.UTC || store.created.UpdatedAt.Location() != time.UTC {
		t.Fatalf("timestamps must use UTC: created=%v updated=%v", store.created.CreatedAt.Location(), store.created.UpdatedAt.Location())
	}
}

func TestSeedAdminRejectsEmptyCredentials(t *testing.T) {
	for _, test := range []struct {
		name     string
		username string
		password string
	}{
		{name: "empty username", username: "", password: "password123"},
		{name: "empty password", username: "admin", password: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeAdminAccountStore{}
			if err := SeedAdmin(context.Background(), store, test.username, test.password); err == nil {
				t.Fatal("SeedAdmin() error = nil, want validation error")
			}
			if store.created.Username != "" || store.created.PasswordHash != "" {
				t.Fatal("store was called for invalid credentials")
			}
		})
	}
}

func TestSeedAdminRejectsUsernameOutsideBounds(t *testing.T) {
	for _, username := range []string{strings.Repeat("a", 2), strings.Repeat("a", 65)} {
		store := &fakeAdminAccountStore{}
		if err := SeedAdmin(context.Background(), store, username, "password123"); err == nil {
			t.Fatalf("SeedAdmin(%d-byte username) error = nil", len(username))
		}
	}
}

func TestSeedAdminRejectsPasswordOutsideByteBounds(t *testing.T) {
	for _, password := range []string{strings.Repeat("a", 7), strings.Repeat("a", 73)} {
		store := &fakeAdminAccountStore{}
		if err := SeedAdmin(context.Background(), store, "admin", password); err == nil {
			t.Fatalf("SeedAdmin(%d-byte password) error = nil", len(password))
		}
	}
}

func TestSeedAdminRefusesDuplicateUsername(t *testing.T) {
	password := "duplicate-secret"
	store := &fakeAdminAccountStore{err: errors.New("duplicate entry for username")}

	err := SeedAdmin(context.Background(), store, "admin", password)
	if err == nil {
		t.Fatal("SeedAdmin() error = nil, want duplicate error")
	}
	if strings.Contains(err.Error(), password) {
		t.Fatal("duplicate error contains the plain password")
	}
}

func TestSeedAdminDoesNotExposePlainPasswordInStoreError(t *testing.T) {
	password := "secret-password"
	store := &fakeAdminAccountStore{err: errors.New("database failed")}

	err := SeedAdmin(context.Background(), store, "admin", password)
	if err == nil {
		t.Fatal("SeedAdmin() error = nil, want store error")
	}
	if strings.Contains(err.Error(), password) {
		t.Fatal("returned error contains the plain password")
	}
}
