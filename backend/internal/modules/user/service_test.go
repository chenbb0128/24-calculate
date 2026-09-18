package user

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
	"time"

	"github.com/example/go-service/internal/apperror"
	"github.com/example/go-service/internal/modules/moderation"
	db "github.com/example/go-service/internal/store/sqlc"
	"github.com/gen2brain/webp"
)

func TestGetProfileReturnsPublicDTO(t *testing.T) {
	store := &fakeStore{user: db.User{
		ID:                       7,
		Username:                 "alice",
		PasswordHash:             "must-not-leak",
		Nickname:                 "Alice",
		NicknameModerationStatus: string(moderation.StatusApproved),
		AvatarModerationStatus:   string(moderation.StatusApproved),
		Status:                   StatusActive,
		CreatedAt:                time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:                time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
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

func TestUpdateProfileRejectsNicknameWithStableBusinessCode(t *testing.T) {
	store := &fakeStore{user: db.User{ID: 7, Username: "alice", Nickname: "Old", Avatar: "old", Status: StatusActive}}
	service := NewService(store)
	nickname := "New"

	_, err := service.UpdateProfile(context.Background(), 7, UpdateProfileInput{Nickname: &nickname})
	if err == nil {
		t.Fatal("UpdateProfile() error = nil, want nickname edit disabled")
	}
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.BusinessCode != "NICKNAME_EDIT_DISABLED" || appErr.HTTPStatus != 400 {
		t.Fatalf("error = %v, app error = %+v", err, appErr)
	}
	if store.updated.ID != 0 {
		t.Fatalf("disabled nickname changed profile: %+v", store.updated)
	}
}

func TestGetProfileHidesUnreviewedProfileFromPublicDTO(t *testing.T) {
	service := NewService(&fakeStore{user: db.User{
		ID: 7, Username: "alice", Nickname: "违规昵称", Avatar: "https://calc-api.pdurl.cn/avatars/7/bad.webp", Status: StatusActive,
		NicknameModerationStatus: string(moderation.StatusRejected), AvatarModerationStatus: string(moderation.StatusPending),
	}})

	profile, err := service.GetProfile(context.Background(), 7)
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if profile.Nickname != DefaultNickname || profile.Avatar != DefaultAvatar {
		t.Fatalf("unsafe profile = %+v", profile)
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
	for _, value := range []string{"sun", "star", "rocket", "target", "rainbow", "spark", "https://cdn.example.com/avatars/7/new.webp"} {
		if _, err := NormalizeAvatar(value); err != nil {
			t.Fatalf("NormalizeAvatar(%q) error = %v", value, err)
		}
	}
	for _, value := range []string{"https://thirdwx.qlogo.cn/example.png", "http://example.com/avatars/7/avatar.webp", "/avatars/7/avatar.webp", "https://cdn.example.com/avatars/7/avatar.webp?cache=1", "javascript:alert(1)"} {
		if _, err := NormalizeAvatar(value); err == nil {
			t.Fatalf("NormalizeAvatar(%q) succeeded, want error", value)
		}
	}
}

func TestNormalizeWeChatAvatarOnlyAcceptsWeChatHTTPSHosts(t *testing.T) {
	for _, value := range []string{"https://thirdwx.qlogo.cn/mmopen/example/132", "https://qlogo.cn/example"} {
		if _, err := NormalizeWeChatAvatar(value); err != nil {
			t.Fatalf("NormalizeWeChatAvatar(%q) error = %v", value, err)
		}
	}
	for _, value := range []string{"http://thirdwx.qlogo.cn/example", "https://evil.example/avatar.png", "https://thirdwx.qlogo.cn.evil.example/avatar.png"} {
		if _, err := NormalizeWeChatAvatar(value); err == nil {
			t.Fatalf("NormalizeWeChatAvatar(%q) succeeded, want error", value)
		}
	}
}

func TestUpdateProfileAcceptsOnlyOwnedHTTPSBackendAvatar(t *testing.T) {
	store := &fakeStore{user: db.User{ID: 7, Username: "alice", Nickname: "玩家", Avatar: DefaultAvatar, Status: StatusActive}}
	service := NewService(store)
	service.SetAvatarPublicBaseURL("https://calc-api.pdurl.cn")

	for _, value := range []string{
		"/avatars/7/avatar.webp",
		"https://evil.example/avatars/7/avatar.webp",
		"https://calc-api.pdurl.cn/avatars/8/avatar.webp",
		"https://calc-api.pdurl.cn/avatars/7/avatar.webp?x=1",
	} {
		store.updated = db.UpdateUserProfileParams{}
		if _, err := service.UpdateProfile(context.Background(), 7, UpdateProfileInput{Avatar: &value}); err == nil {
			t.Fatalf("UpdateProfile(%q) succeeded, want error", value)
		}
		if store.updated.ID != 0 {
			t.Fatalf("invalid avatar %q changed profile: %+v", value, store.updated)
		}
	}

	value := "https://calc-api.pdurl.cn/avatars/7/avatar.webp"
	if _, err := service.UpdateProfile(context.Background(), 7, UpdateProfileInput{Avatar: &value}); err != nil {
		t.Fatalf("UpdateProfile(owned avatar) error = %v", err)
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

func TestUploadAvatarRejectsEmptyAndOversizedFilesWithExpectedStatuses(t *testing.T) {
	store := &fakeStore{user: db.User{ID: 7, Username: "alice", Nickname: "玩家", Avatar: DefaultAvatar, Status: StatusActive}}
	storage := &fakeAvatarStorage{avatar: StoredAvatar{Key: "avatars/7/new.webp", URL: "https://cdn.example.com/avatars/7/new.webp"}}
	service := NewServiceWithAvatarStorage(store, storage, 2<<20, 4096, time.Minute)

	if _, err := service.UploadAvatar(context.Background(), 7, nil, 4096); !hasHTTPStatus(err, 400) {
		t.Fatalf("empty upload error = %v, want HTTP 400", err)
	}
	if _, err := service.UploadAvatar(context.Background(), 7, make([]byte, (2<<20)+1), 4096); !hasHTTPStatus(err, 413) {
		t.Fatalf("oversized upload error = %v, want HTTP 413", err)
	}
	if store.updated.ID != 0 || len(storage.saved) != 0 {
		t.Fatalf("rejected uploads changed state: updated = %+v, saved = %d", store.updated, len(storage.saved))
	}
}

func TestUploadAvatarSupportsJPEGPngAndWebP(t *testing.T) {
	for _, test := range []struct {
		name string
		data []byte
	}{
		{name: "jpeg", data: testJPEG(t, 320, 240)},
		{name: "png", data: testPNG(t, 320, 240)},
		{name: "webp", data: testWEBP(t, 320, 240)},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeStore{user: db.User{ID: 7, Username: "alice", Nickname: "玩家", Avatar: DefaultAvatar, Status: StatusActive}}
			storage := &fakeAvatarStorage{avatar: StoredAvatar{Key: "avatars/7/new.webp", URL: "https://cdn.example.com/avatars/7/new.webp"}}
			service := NewServiceWithAvatarStorage(store, storage, 2<<20, 4096, time.Minute)
			result, err := service.UploadAvatar(context.Background(), 7, test.data, 4096)
			if err != nil {
				t.Fatalf("UploadAvatar() error = %v", err)
			}
			if result.Width != 256 || result.Height != 256 || result.Format != "webp" {
				t.Fatalf("result = %+v", result)
			}
		})
	}
}

func TestUploadAvatarFailureDoesNotOverwriteOldProfile(t *testing.T) {
	store := &fakeStore{user: db.User{ID: 7, Username: "alice", Nickname: "玩家", Avatar: "https://cdn.example.com/avatars/7/old.webp", Status: StatusActive}, updateErr: errors.New("database unavailable")}
	storage := &fakeAvatarStorage{avatar: StoredAvatar{Key: "avatars/7/new.webp", URL: "https://cdn.example.com/avatars/7/new.webp"}}
	service := NewServiceWithAvatarStorage(store, storage, 2<<20, 4096, time.Minute)

	if _, err := service.UploadAvatar(context.Background(), 7, testPNG(t, 300, 300), 4096); err == nil {
		t.Fatal("UploadAvatar() error = nil, want profile update failure")
	}
	if store.user.Avatar != "https://cdn.example.com/avatars/7/old.webp" || len(storage.deleted) != 1 {
		t.Fatalf("old profile was overwritten or new file not cleaned: user = %+v, deleted = %v", store.user, storage.deleted)
	}
}

func TestUploadAvatarUsesConfiguredRateLimit(t *testing.T) {
	store := &fakeStore{user: db.User{ID: 7, Username: "alice", Nickname: "玩家", Avatar: DefaultAvatar, Status: StatusActive}}
	storage := &fakeAvatarStorage{avatar: StoredAvatar{Key: "avatars/7/new.webp", URL: "https://cdn.example.com/avatars/7/new.webp"}}
	limiter := &fakeAvatarRateLimiter{allowed: true}
	service := NewServiceWithAvatarStorage(store, storage, 2<<20, 4096, 30*time.Second)
	service.SetAvatarRateLimiter(limiter)

	if _, err := service.UploadAvatar(context.Background(), 7, testPNG(t, 300, 300), 4096); err != nil {
		t.Fatalf("first UploadAvatar() error = %v", err)
	}
	limiter.allowed = false
	if _, err := service.UploadAvatar(context.Background(), 7, testPNG(t, 300, 300), 4096); !hasHTTPStatus(err, 429) {
		t.Fatalf("second UploadAvatar() error = %v, want HTTP 429", err)
	}
	if limiter.userID != 7 || limiter.limit != 1 || limiter.window != 30*time.Second {
		t.Fatalf("rate limit arguments = %+v", limiter)
	}
}

func TestUploadAvatarRejectedByModeratorLeavesOldAvatarAndDoesNotSave(t *testing.T) {
	store := &fakeStore{user: db.User{ID: 7, Username: "alice", Nickname: "玩家", Avatar: DefaultAvatar, Status: StatusActive}}
	storage := &fakeAvatarStorage{avatar: StoredAvatar{Key: "avatars/7/new.webp", URL: "https://cdn.example.com/avatars/7/new.webp"}}
	service := NewServiceWithAvatarStorage(store, storage, 2<<20, 4096, time.Minute)
	service.SetContentModerator(moderation.NewService(fakeModerationProvider{image: moderation.ProviderResult{Status: moderation.StatusRejected, ReasonCode: "87014"}}, nil))

	_, err := service.UploadAvatar(context.Background(), 7, testPNG(t, 300, 300), 4096)
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.BusinessCode != "AVATAR_REJECTED" {
		t.Fatalf("error = %v, app error = %+v", err, appErr)
	}
	if store.user.Avatar != DefaultAvatar || store.updated.ID != 0 || len(storage.saved) != 0 {
		t.Fatalf("rejected upload changed state: user = %+v, updated = %+v, saved = %d", store.user, store.updated, len(storage.saved))
	}
}

func TestUploadAvatarPendingLeavesOldAvatarAndReturnsNoURL(t *testing.T) {
	store := &fakeStore{user: db.User{ID: 7, Username: "alice", Nickname: "玩家", Avatar: DefaultAvatar, Status: StatusActive}}
	storage := &fakeAvatarStorage{avatar: StoredAvatar{Key: "avatars/7/new.webp", URL: "https://cdn.example.com/avatars/7/new.webp"}}
	service := NewServiceWithAvatarStorage(store, storage, 2<<20, 4096, time.Minute)
	service.SetContentModerator(moderation.NewService(fakeModerationProvider{image: moderation.ProviderResult{Status: moderation.StatusPending}}, nil))

	result, err := service.UploadAvatar(context.Background(), 7, testPNG(t, 300, 300), 4096)
	if err != nil || result.ModerationStatus != string(moderation.StatusPending) || result.AvatarURL != "" {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if store.user.Avatar != DefaultAvatar || store.updated.ID != 0 || len(storage.saved) != 0 {
		t.Fatalf("pending upload changed state: user = %+v, updated = %+v, saved = %d", store.user, store.updated, len(storage.saved))
	}
}

func TestUploadAvatarProviderUnavailableDoesNotPublish(t *testing.T) {
	store := &fakeStore{user: db.User{ID: 7, Username: "alice", Nickname: "玩家", Avatar: DefaultAvatar, Status: StatusActive}}
	storage := &fakeAvatarStorage{avatar: StoredAvatar{Key: "avatars/7/new.webp", URL: "https://cdn.example.com/avatars/7/new.webp"}}
	service := NewServiceWithAvatarStorage(store, storage, 2<<20, 4096, time.Minute)
	service.SetContentModerator(moderation.NewService(fakeModerationProvider{err: errors.New("provider down")}, nil))

	_, err := service.UploadAvatar(context.Background(), 7, testPNG(t, 300, 300), 4096)
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.BusinessCode != "MODERATION_PROVIDER_UNAVAILABLE" || appErr.HTTPStatus != 503 {
		t.Fatalf("error = %v, app error = %+v", err, appErr)
	}
	if store.updated.ID != 0 || len(storage.saved) != 0 {
		t.Fatalf("unavailable moderation published upload: updated = %+v, saved = %d", store.updated, len(storage.saved))
	}
}

func TestSafePublicProfileHidesNonApprovedValues(t *testing.T) {
	if got := SafePublicNickname("违规昵称", string(moderation.StatusRejected)); got != DefaultNickname {
		t.Fatalf("SafePublicNickname() = %q", got)
	}
	if got := SafePublicAvatar("https://calc-api.pdurl.cn/avatars/7/bad.webp", string(moderation.StatusPending)); got != DefaultAvatar {
		t.Fatalf("SafePublicAvatar() = %q", got)
	}
	if got := SafePublicNickname("玩家", string(moderation.StatusApproved)); got != "玩家" {
		t.Fatalf("approved nickname = %q", got)
	}
	wechatAvatar := "https://thirdwx.qlogo.cn/mmopen/example/132"
	if got := SafePublicAvatar(wechatAvatar, string(moderation.StatusApproved)); got != wechatAvatar {
		t.Fatalf("approved WeChat avatar = %q", got)
	}
}

func TestSafePublicAvatarNeverReturnsWeChatProviderURL(t *testing.T) {
	if got := SafePublicAvatar("https://thirdwx.qlogo.cn/mmopen/example/132", string(moderation.StatusApproved)); got != DefaultAvatar {
		t.Fatalf("SafePublicAvatar() = %q, want default avatar", got)
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

func testJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 100, A: 255})
		}
	}
	var data bytes.Buffer
	if err := jpeg.Encode(&data, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("jpeg.Encode() error = %v", err)
	}
	return data.Bytes()
}

func testWEBP(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 100, A: 255})
		}
	}
	var data bytes.Buffer
	if err := webp.Encode(&data, img, webp.Options{Quality: 80, Method: 4}); err != nil {
		t.Fatalf("webp.Encode() error = %v", err)
	}
	return data.Bytes()
}

func hasHTTPStatus(err error, status int) bool {
	var appErr *apperror.AppError
	return errors.As(err, &appErr) && appErr.HTTPStatus == status
}

func TestGetProfileNotFound(t *testing.T) {
	service := NewService(&fakeStore{})
	_, err := service.GetProfile(context.Background(), 7)
	if err == nil || err.Error() != "用户不存在" {
		t.Fatalf("err = %v, want user not found", err)
	}
}

type fakeStore struct {
	user      db.User
	updated   db.UpdateUserProfileParams
	updateErr error
}

type fakeAvatarStorage struct {
	avatar  StoredAvatar
	saved   []byte
	err     error
	deleted []string
}

type fakeModerationProvider struct {
	text  moderation.ProviderResult
	image moderation.ProviderResult
	err   error
}

func (f fakeModerationProvider) CheckText(context.Context, moderation.TextCheckRequest) (moderation.ProviderResult, error) {
	return f.text, f.err
}

func (f fakeModerationProvider) CheckImage(context.Context, moderation.ImageCheckRequest) (moderation.ProviderResult, error) {
	return f.image, f.err
}

func (f *fakeAvatarStorage) Save(_ context.Context, _ uint64, data []byte) (StoredAvatar, error) {
	if f.err != nil {
		return StoredAvatar{}, f.err
	}
	f.saved = append([]byte(nil), data...)
	return f.avatar, nil
}

func (f *fakeAvatarStorage) Delete(_ context.Context, value string) error {
	f.deleted = append(f.deleted, value)
	return nil
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
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updated = arg
	f.user.Nickname = arg.Nickname
	f.user.Avatar = arg.Avatar
	return nil
}

type fakeAvatarRateLimiter struct {
	allowed bool
	userID  uint64
	limit   int64
	window  time.Duration
}

func (f *fakeAvatarRateLimiter) AllowAvatarUpload(_ context.Context, userID uint64, limit int64, window time.Duration) (bool, error) {
	f.userID, f.limit, f.window = userID, limit, window
	return f.allowed, nil
}
