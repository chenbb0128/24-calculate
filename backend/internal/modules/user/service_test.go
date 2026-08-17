package user

import (
	"bytes"
	"context"
	"database/sql"
	"image"
	"image/color"
	"image/png"
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
	avatar := "star"

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

func TestGetProfileAppliesDefaultsForLegacyEmptyFields(t *testing.T) {
	service := NewService(&fakeStore{user: db.User{ID: 7, Username: "alice", Status: StatusActive}})

	profile, err := service.GetProfile(context.Background(), 7)
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if profile.Nickname != DefaultNickname || profile.Avatar != DefaultAvatar {
		t.Fatalf("profile defaults = %+v", profile)
	}
}

func TestNormalizeNickname(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "default", input: "", want: DefaultNickname},
		{name: "trim", input: "  玩家  ", want: "玩家"},
		{name: "too long", input: "一二三四五六七八九十一二三", wantErr: true},
		{name: "script", input: "<script>alert(1)</script>", wantErr: true},
		{name: "control", input: "玩家\u0000", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeNickname(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NormalizeNickname() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("NormalizeNickname() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeAvatar(t *testing.T) {
	if got, err := NormalizeAvatar(""); err != nil || got != DefaultAvatar {
		t.Fatalf("empty avatar = %q, err = %v", got, err)
	}
	for _, value := range []string{"sun", "star", "rocket", "target", "rainbow", "spark", "https://thirdwx.qlogo.cn/example.png"} {
		if _, err := NormalizeAvatar(value); err != nil {
			t.Fatalf("NormalizeAvatar(%q) error = %v", value, err)
		}
	}
	for _, value := range []string{"http://example.com/avatar.png", "avatar.png", "javascript:alert(1)"} {
		if _, err := NormalizeAvatar(value); err == nil {
			t.Fatalf("NormalizeAvatar(%q) succeeded, want error", value)
		}
	}
}

func TestUploadAvatarProcessesImageAndUpdatesProfile(t *testing.T) {
	store := &fakeStore{user: db.User{ID: 7, Username: "alice", Nickname: "玩家", Avatar: DefaultAvatar, Status: StatusActive}}
	storage := &fakeAvatarStorage{avatar: StoredAvatar{Key: "avatars/7/new.webp", URL: "https://cdn.example.com/avatars/7/new.webp", Width: 256, Height: 256, Format: "webp"}}
	service := NewServiceWithAvatarStorage(store, storage, 2<<20, 4096, time.Minute)

	input := testPNG(t, 400, 200)
	result, err := service.UploadAvatar(context.Background(), 7, input, 4096)
	if err != nil {
		t.Fatalf("UploadAvatar() error = %v", err)
	}
	if result.AvatarURL != storage.avatar.URL || result.Width != 256 || result.Height != 256 || result.Format != "webp" {
		t.Fatalf("upload result = %+v", result)
	}
	if store.updated.Avatar != storage.avatar.URL || len(storage.saved) == 0 {
		t.Fatalf("stored profile = %+v, saved bytes = %d", store.updated, len(storage.saved))
	}
}

func TestUploadAvatarRejectsInvalidImageWithoutUpdatingProfile(t *testing.T) {
	store := &fakeStore{user: db.User{ID: 7, Username: "alice", Nickname: "玩家", Avatar: DefaultAvatar, Status: StatusActive}}
	storage := &fakeAvatarStorage{avatar: StoredAvatar{Key: "avatars/7/new.webp", URL: "https://cdn.example.com/avatars/7/new.webp", Width: 256, Height: 256, Format: "webp"}}
	service := NewServiceWithAvatarStorage(store, storage, 2<<20, 4096, time.Minute)

	if _, err := service.UploadAvatar(context.Background(), 7, []byte("GIF89a not allowed"), 4096); err == nil {
		t.Fatal("UploadAvatar() error = nil for invalid image")
	}
	if store.updated.ID != 0 || len(storage.saved) != 0 {
		t.Fatalf("invalid upload changed state: updated = %+v, saved = %d", store.updated, len(storage.saved))
	}
}

func testPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 100, A: 255})
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return data.Bytes()
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

type fakeAvatarStorage struct {
	avatar StoredAvatar
	saved  []byte
}

func (f *fakeAvatarStorage) Save(_ context.Context, _ uint64, data []byte) (StoredAvatar, error) {
	f.saved = append([]byte(nil), data...)
	return f.avatar, nil
}

func (f *fakeAvatarStorage) Delete(context.Context, string) error { return nil }

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
