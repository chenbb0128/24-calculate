package auth

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/example/go-service/internal/config"
	jwtplatform "github.com/example/go-service/internal/platform/jwt"
	db "github.com/example/go-service/internal/store/sqlc"
)

func TestRegisterSuccess(t *testing.T) {
	users := &fakeUserStore{}
	service := newTestService(users, &fakeTokenStore{})

	result, err := service.Register(context.Background(), RegisterInput{
		Username: "alice",
		Password: "password123",
		Nickname: "Alice",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if result.ID == 0 || result.Username != "alice" {
		t.Fatalf("result = %+v", result)
	}
	if users.created.PasswordHash == "password123" || bcrypt.CompareHashAndPassword([]byte(users.created.PasswordHash), []byte("password123")) != nil {
		t.Fatal("password was not bcrypt hashed")
	}
}

func TestRegisterDuplicateUsername(t *testing.T) {
	users := &fakeUserStore{byUsername: db.User{ID: 1, Username: "alice"}}
	service := newTestService(users, &fakeTokenStore{})

	_, err := service.Register(context.Background(), RegisterInput{Username: "alice", Password: "password123"})
	var appErr interface{ Unwrap() error }
	if err == nil || !errors.As(err, &appErr) || err.Error() == "" {
		t.Fatalf("err = %v, want duplicate username error", err)
	}
}

func TestLoginPasswordErrorUsesGenericMessage(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	users := &fakeUserStore{byUsername: db.User{ID: 1, Username: "alice", PasswordHash: string(hash), Status: 1}}
	service := newTestService(users, &fakeTokenStore{})

	_, err = service.Login(context.Background(), LoginInput{Username: "alice", Password: "wrong-password"}, "127.0.0.1")
	if err == nil || err.Error() != "用户名或密码错误" {
		t.Fatalf("err = %v, want generic credential error", err)
	}
}

func TestLoginDisabledUser(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	users := &fakeUserStore{byUsername: db.User{ID: 1, Username: "alice", PasswordHash: string(hash), Status: 0}}
	service := newTestService(users, &fakeTokenStore{})

	_, err = service.Login(context.Background(), LoginInput{Username: "alice", Password: "password123"}, "127.0.0.1")
	if err == nil || err.Error() != "用户已禁用" {
		t.Fatalf("err = %v, want disabled error", err)
	}
}

func TestDevLoginCreatesStableLocalPlayer(t *testing.T) {
	users := &fakeUserStore{}
	tokens := &fakeTokenStore{}
	service := newTestService(users, tokens)

	result, err := service.DevLogin(context.Background(), DevLoginInput{Slot: 2}, "127.0.0.1")
	if err != nil {
		t.Fatalf("DevLogin() error = %v", err)
	}
	if result.AccessToken == "" || users.created.Username != "dev_player_2" || tokens.savedJTI == "" {
		t.Fatalf("result = %#v, created = %#v, tokens = %#v", result, users.created, tokens)
	}
}

func TestRefreshRotatesRefreshToken(t *testing.T) {
	manager := newTestJWTManager(t)
	refreshToken, claims, err := manager.IssueRefreshToken(1)
	if err != nil {
		t.Fatal(err)
	}
	tokens := &fakeTokenStore{consumedUserID: 1}
	users := &fakeUserStore{byID: db.User{ID: 1, Username: "alice", Status: 1}}
	service := NewService(users, tokens, manager, time.Minute, time.Hour, nil, nil)

	result, err := service.Refresh(context.Background(), RefreshInput{RefreshToken: refreshToken})
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if result.RefreshToken == refreshToken || tokens.consumedJTI != claims.ID || tokens.savedJTI == "" {
		t.Fatalf("rotation state = %+v", tokens)
	}
}

type fakeUserStore struct {
	byID       db.User
	byUsername db.User
	created    db.User
}

func (f *fakeUserStore) GetUserByID(context.Context, uint64) (db.User, error) {
	if f.byID.ID == 0 {
		return db.User{}, sql.ErrNoRows
	}
	return f.byID, nil
}

func (f *fakeUserStore) GetUserByUsername(context.Context, string) (db.User, error) {
	if f.byUsername.ID == 0 {
		return db.User{}, sql.ErrNoRows
	}
	return f.byUsername, nil
}

func (f *fakeUserStore) CreateUserTx(_ context.Context, arg db.CreateUserParams) (uint64, error) {
	f.created = db.User{ID: 1, Username: arg.Username, PasswordHash: arg.PasswordHash, Status: arg.Status}
	return 1, nil
}

type fakeTokenStore struct {
	consumedUserID uint64
	consumedJTI    string
	savedJTI       string
}

func (f *fakeTokenStore) SaveRefreshToken(_ context.Context, jti string, _ uint64, _ time.Duration) error {
	f.savedJTI = jti
	return nil
}

func (f *fakeTokenStore) ConsumeRefreshToken(_ context.Context, jti string) (uint64, error) {
	f.consumedJTI = jti
	return f.consumedUserID, nil
}

func (f *fakeTokenStore) RevokeRefreshToken(context.Context, string) error { return nil }

func (f *fakeTokenStore) AllowLogin(context.Context, string, int64, time.Duration) (bool, error) {
	return true, nil
}

func newTestService(users UserStore, tokens TokenStore) *Service {
	return NewService(users, tokens, newTestJWTManager(nil), time.Minute, time.Hour, nil, nil)
}

func newTestJWTManager(t *testing.T) *jwtplatform.Manager {
	if t != nil {
		t.Helper()
	}
	manager, err := jwtplatform.NewManager(configForTest())
	if err != nil {
		if t != nil {
			t.Fatal(err)
		}
		panic(err)
	}
	return manager
}

func configForTest() config.JWTConfig {
	return config.JWTConfig{
		Secret:     "01234567890123456789012345678901",
		Algorithm:  "HS256",
		Issuer:     "go-service",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
	}
}
