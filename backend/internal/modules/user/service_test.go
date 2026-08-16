package user

import (
	"context"
	"database/sql"
	"testing"
	"time"

	db "github.com/example/go-service/internal/store/sqlc"
)

func TestGetProfileReturnsPublicDTO(t *testing.T) {
	store := &fakeStore{user: db.User{
		ID:           7,
		Username:     "alice",
		PasswordHash: "must-not-leak",
		Nickname:     "Alice",
		Status:       StatusActive,
		CreatedAt:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:    time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	}}
	service := NewService(store)

	profile, err := service.GetProfile(context.Background(), 7)
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if profile.ID != 7 || profile.Username != "alice" || profile.CreatedAt != "2026-01-01T00:00:00Z" {
		t.Fatalf("profile = %+v", profile)
	}
}

func TestUpdateProfile(t *testing.T) {
	store := &fakeStore{user: db.User{ID: 7, Username: "alice", Nickname: "Old", Avatar: "old", Status: StatusActive}}
	service := NewService(store)
	nickname := "New"
	avatar := "new-avatar"

	profile, err := service.UpdateProfile(context.Background(), 7, UpdateProfileInput{Nickname: &nickname, Avatar: &avatar})
	if err != nil {
		t.Fatalf("UpdateProfile() error = %v", err)
	}
	if profile.Nickname != nickname || profile.Avatar != avatar {
		t.Fatalf("profile = %+v", profile)
	}
	if store.updated.Nickname != nickname || store.updated.Avatar != avatar {
		t.Fatalf("updated params = %+v", store.updated)
	}
}

func TestGetProfileNotFound(t *testing.T) {
	service := NewService(&fakeStore{})
	_, err := service.GetProfile(context.Background(), 7)
	if err == nil || err.Error() != "用户不存在" {
		t.Fatalf("err = %v, want user not found", err)
	}
}

type fakeStore struct {
	user    db.User
	updated db.UpdateUserProfileParams
}

func (f *fakeStore) GetUserByID(context.Context, uint64) (db.User, error) {
	if f.user.ID == 0 {
		return db.User{}, sql.ErrNoRows
	}
	return f.user, nil
}

func (f *fakeStore) GetUserByUsername(context.Context, string) (db.User, error) {
	return db.User{}, sql.ErrNoRows
}

func (f *fakeStore) CreateUser(context.Context, db.CreateUserParams) (uint64, error) {
	return 1, nil
}

func (f *fakeStore) UpdateUserProfile(_ context.Context, arg db.UpdateUserProfileParams) error {
	f.updated = arg
	return nil
}
