package user

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/example/go-service/internal/modules/moderation"
	wechatplatform "github.com/example/go-service/internal/platform/wechat"
	db "github.com/example/go-service/internal/store/sqlc"
)

func TestSyncAuthorizedWeChatProfileRejectedNicknameKeepsOldValue(t *testing.T) {
	store := &fakeStore{user: db.User{
		ID: 7, Nickname: "原昵称", Avatar: DefaultAvatar, Status: StatusActive,
		NicknameModerationStatus: string(moderation.StatusApproved),
		AvatarModerationStatus:   string(moderation.StatusApproved),
	}}
	service := NewService(store)
	service.SetContentModerator(moderation.NewService(fakeModerationProvider{
		text: moderation.ProviderResult{Status: moderation.StatusRejected, ReasonCode: "87014"},
	}, nil))

	result, err := service.SyncAuthorizedWeChatProfile(
		context.Background(),
		7,
		"openid-7",
		WeChatProfileInput{Nickname: "违规昵称"},
		"wechat_profile_sync",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.NicknameUpdated || result.Profile.Nickname != "原昵称" || result.SyncStatus != string(moderation.StatusRejected) {
		t.Fatalf("result = %+v", result)
	}
	if store.updated.Nickname != "" {
		t.Fatalf("rejected nickname changed stored profile: %+v", store.updated)
	}
}

func TestSyncAuthorizedWeChatProfileApprovedNicknameUpdatesProfile(t *testing.T) {
	store := &fakeStore{user: db.User{
		ID: 7, Nickname: DefaultNickname, Avatar: DefaultAvatar, Status: StatusActive,
		NicknameModerationStatus: string(moderation.StatusApproved),
		AvatarModerationStatus:   string(moderation.StatusApproved),
	}}
	service := newTestWeChatProfileService(store, moderation.ProviderResult{Status: moderation.StatusApproved}, moderation.ProviderResult{Status: moderation.StatusApproved})

	result, err := service.SyncAuthorizedWeChatProfile(context.Background(), 7, "openid-7", WeChatProfileInput{Nickname: "新昵称"}, "wechat_profile_sync")
	if err != nil {
		t.Fatal(err)
	}
	if result.SyncStatus != string(moderation.StatusApproved) || !result.NicknameUpdated || result.Profile.Nickname != "新昵称" {
		t.Fatalf("result = %+v", result)
	}
	if store.updated.Nickname != "新昵称" || store.updated.NicknameModerationStatus != string(moderation.StatusApproved) {
		t.Fatalf("stored update = %+v", store.updated)
	}
}

func TestSyncAuthorizedWeChatProfilePendingKeepsOldValue(t *testing.T) {
	store := &fakeStore{user: db.User{
		ID: 7, Nickname: "原昵称", Avatar: DefaultAvatar, Status: StatusActive,
		NicknameModerationStatus: string(moderation.StatusApproved),
		AvatarModerationStatus:   string(moderation.StatusApproved),
	}}
	service := newTestWeChatProfileService(store, moderation.ProviderResult{Status: moderation.StatusPending}, moderation.ProviderResult{Status: moderation.StatusApproved})

	result, err := service.SyncAuthorizedWeChatProfile(context.Background(), 7, "openid-7", WeChatProfileInput{Nickname: "待审核昵称"}, "wechat_profile_sync")
	if err != nil {
		t.Fatal(err)
	}
	if result.SyncStatus != string(moderation.StatusPending) || result.NicknameUpdated || result.Profile.Nickname != "原昵称" {
		t.Fatalf("result = %+v", result)
	}
	if store.updated.Nickname != "" {
		t.Fatalf("pending nickname changed stored profile: %+v", store.updated)
	}
}

func TestSyncAuthorizedWeChatProfileUnavailableKeepsOldValue(t *testing.T) {
	store := &fakeStore{user: db.User{
		ID: 7, Nickname: "原昵称", Avatar: DefaultAvatar, Status: StatusActive,
		NicknameModerationStatus: string(moderation.StatusApproved),
		AvatarModerationStatus:   string(moderation.StatusApproved),
	}}
	service := NewService(store)

	result, err := service.SyncAuthorizedWeChatProfile(context.Background(), 7, "openid-7", WeChatProfileInput{Nickname: "新昵称"}, "wechat_profile_sync")
	if err != nil {
		t.Fatal(err)
	}
	if result.SyncStatus != string(moderation.StatusUnavailable) || result.NicknameUpdated || result.Profile.Nickname != "原昵称" {
		t.Fatalf("result = %+v", result)
	}
	if store.updated.Nickname != "" {
		t.Fatalf("unavailable nickname changed stored profile: %+v", store.updated)
	}
}

func TestSyncAuthorizedWeChatProfileApprovedAvatarUsesOwnStorage(t *testing.T) {
	store := &fakeStore{user: db.User{
		ID: 7, Nickname: "原昵称", Avatar: DefaultAvatar, Status: StatusActive,
		NicknameModerationStatus: string(moderation.StatusApproved),
		AvatarModerationStatus:   string(moderation.StatusApproved),
	}}
	storage := &fakeAvatarStorage{avatar: StoredAvatar{
		Key: "avatars/7/new.webp", URL: "https://calc-api.pdurl.cn/avatars/7/new.webp",
		Width: 256, Height: 256, Format: "webp",
	}}
	service := newTestWeChatProfileService(store, moderation.ProviderResult{Status: moderation.StatusApproved}, moderation.ProviderResult{Status: moderation.StatusApproved})
	service.SetAvatarPublicBaseURL("https://calc-api.pdurl.cn")
	service.avatarStorage = storage
	service.SetWeChatAvatarFetcher(fakeWeChatAvatarFetcher{data: testPNG(t, 40, 40)})

	result, err := service.SyncAuthorizedWeChatProfile(context.Background(), 7, "openid-7", WeChatProfileInput{Avatar: "https://thirdwx.qlogo.cn/mmopen/example/132"}, "wechat_profile_sync")
	if err != nil {
		t.Fatal(err)
	}
	if result.SyncStatus != string(moderation.StatusApproved) || !result.AvatarUpdated || result.Profile.Avatar != storage.avatar.URL {
		t.Fatalf("result = %+v", result)
	}
	if store.updated.Avatar != storage.avatar.URL || len(storage.saved) == 0 {
		t.Fatalf("avatar was not stored: update=%+v saved=%d", store.updated, len(storage.saved))
	}
}

func TestSyncAuthorizedWeChatProfileDoesNotDeletePresetAvatarOnProfileFailure(t *testing.T) {
	store := &fakeStore{
		user: db.User{
			ID: 7, Nickname: DefaultNickname, Avatar: DefaultAvatar, Status: StatusActive,
			NicknameModerationStatus: string(moderation.StatusApproved),
			AvatarModerationStatus:   string(moderation.StatusApproved),
		},
		updateErr: errors.New("database unavailable"),
	}
	storage := &fakeAvatarStorage{}
	service := newTestWeChatProfileService(store, moderation.ProviderResult{Status: moderation.StatusApproved}, moderation.ProviderResult{Status: moderation.StatusApproved})
	service.avatarStorage = storage

	_, err := service.SyncAuthorizedWeChatProfile(
		context.Background(),
		7,
		"openid-7",
		WeChatProfileInput{Nickname: "新昵称", Avatar: DefaultAvatar},
		"wechat_profile_sync",
	)
	if err == nil {
		t.Fatal("SyncAuthorizedWeChatProfile() error = nil, want profile update failure")
	}
	if len(storage.deleted) != 0 {
		t.Fatalf("preset avatar was treated as stored file: deleted = %v", storage.deleted)
	}
}

func TestSyncAuthorizedWeChatProfileRejectedAvatarKeepsOldValue(t *testing.T) {
	store := &fakeStore{user: db.User{
		ID: 7, Nickname: "原昵称", Avatar: "https://calc-api.pdurl.cn/avatars/7/old.webp", Status: StatusActive,
		NicknameModerationStatus: string(moderation.StatusApproved),
		AvatarModerationStatus:   string(moderation.StatusApproved),
	}}
	storage := &fakeAvatarStorage{avatar: StoredAvatar{Key: "avatars/7/new.webp", URL: "https://calc-api.pdurl.cn/avatars/7/new.webp"}}
	service := newTestWeChatProfileService(store, moderation.ProviderResult{Status: moderation.StatusApproved}, moderation.ProviderResult{Status: moderation.StatusRejected})
	service.SetAvatarPublicBaseURL("https://calc-api.pdurl.cn")
	service.avatarStorage = storage
	service.SetWeChatAvatarFetcher(fakeWeChatAvatarFetcher{data: testPNG(t, 40, 40)})

	result, err := service.SyncAuthorizedWeChatProfile(
		context.Background(),
		7,
		"openid-7",
		WeChatProfileInput{Avatar: "https://thirdwx.qlogo.cn/mmopen/example/132"},
		"wechat_profile_sync",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.SyncStatus != string(moderation.StatusRejected) || result.AvatarUpdated || result.Profile.Avatar != store.user.Avatar {
		t.Fatalf("result = %+v, old avatar = %q", result, store.user.Avatar)
	}
	if store.updated.ID != 0 || len(storage.saved) != 0 {
		t.Fatalf("rejected avatar changed state: update = %+v, saved = %d", store.updated, len(storage.saved))
	}
}

func TestSyncAuthorizedWeChatProfileCannotUsePresetAvatarAsWeChatProfile(t *testing.T) {
	store := &fakeStore{user: db.User{
		ID: 7, Nickname: "原昵称", Avatar: DefaultAvatar, Status: StatusActive,
		NicknameModerationStatus: string(moderation.StatusApproved),
		AvatarModerationStatus:   string(moderation.StatusApproved),
	}}
	storage := &fakeAvatarStorage{avatar: StoredAvatar{Key: "avatars/7/new.webp", URL: "https://calc-api.pdurl.cn/avatars/7/new.webp"}}
	service := newTestWeChatProfileService(store, moderation.ProviderResult{Status: moderation.StatusApproved}, moderation.ProviderResult{Status: moderation.StatusApproved})
	service.avatarStorage = storage

	result, err := service.SyncAuthorizedWeChatProfile(
		context.Background(),
		7,
		"openid-7",
		WeChatProfileInput{Avatar: DefaultAvatar},
		"wechat_profile_sync",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.SyncStatus != string(moderation.StatusRejected) || result.AvatarUpdated || result.Profile.Avatar != DefaultAvatar {
		t.Fatalf("preset avatar result = %+v", result)
	}
	if store.updated.ID != 0 || len(storage.saved) != 0 {
		t.Fatalf("preset avatar changed state: update = %+v, saved = %d", store.updated, len(storage.saved))
	}
}

func TestSyncAuthorizedWeChatProfileRequiresPublicHTTPSAvatarURL(t *testing.T) {
	store := &fakeStore{user: db.User{
		ID: 7, Nickname: "原昵称", Avatar: DefaultAvatar, Status: StatusActive,
		NicknameModerationStatus: string(moderation.StatusApproved),
		AvatarModerationStatus:   string(moderation.StatusApproved),
	}}
	storage := &fakeAvatarStorage{avatar: StoredAvatar{Key: "avatars/7/new.webp", URL: "/avatars/7/new.webp"}}
	service := newTestWeChatProfileService(store, moderation.ProviderResult{Status: moderation.StatusApproved}, moderation.ProviderResult{Status: moderation.StatusApproved})
	service.avatarStorage = storage
	service.SetWeChatAvatarFetcher(fakeWeChatAvatarFetcher{data: testPNG(t, 40, 40)})

	result, err := service.SyncAuthorizedWeChatProfile(
		context.Background(),
		7,
		"openid-7",
		WeChatProfileInput{Avatar: "https://thirdwx.qlogo.cn/mmopen/example/132"},
		"wechat_profile_sync",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.SyncStatus != string(moderation.StatusUnavailable) || result.AvatarUpdated || result.Profile.Avatar != DefaultAvatar {
		t.Fatalf("missing public avatar URL result = %+v", result)
	}
	if store.updated.ID != 0 || len(storage.deleted) != 1 {
		t.Fatalf("invalid stored avatar changed state: update = %+v, deleted = %d", store.updated, len(storage.deleted))
	}
}

func TestSyncWeChatProfileRejectsIdentityMismatch(t *testing.T) {
	store := &fakeSyncIdentityStore{
		fakeStore: &fakeStore{user: db.User{ID: 7, Nickname: DefaultNickname, Avatar: DefaultAvatar, Status: StatusActive}},
		identity:  db.User{ID: 8, Status: StatusActive},
	}
	service := NewService(store)
	service.SetWeChatProfileClient(fakeWeChatProfileClient{result: wechatplatform.LoginResult{OpenID: "openid-8"}})
	service.SetWeChatProfileGuard(&fakeWeChatProfileGuard{})

	_, err := service.SyncWeChatProfile(context.Background(), 7, WeChatProfileSyncInput{Code: "code-1", Nickname: "新昵称"})
	if !hasHTTPStatus(err, 403) {
		t.Fatalf("error = %v, want 403", err)
	}
	if store.updated.Nickname != "" {
		t.Fatalf("mismatched identity changed profile: %+v", store.updated)
	}
}

func TestSyncWeChatProfileRejectsReplayedCode(t *testing.T) {
	store := &fakeSyncIdentityStore{
		fakeStore: &fakeStore{user: db.User{ID: 7, Nickname: DefaultNickname, Avatar: DefaultAvatar, Status: StatusActive}},
		identity:  db.User{ID: 7, Status: StatusActive},
	}
	guard := &fakeWeChatProfileGuard{claimed: false, allowed: true}
	service := NewService(store)
	service.SetWeChatProfileClient(fakeWeChatProfileClient{result: wechatplatform.LoginResult{OpenID: "openid-7"}})
	service.SetWeChatProfileGuard(guard)

	_, err := service.SyncWeChatProfile(context.Background(), 7, WeChatProfileSyncInput{Code: "code-1", Nickname: "新昵称"})
	if !hasHTTPStatus(err, 409) {
		t.Fatalf("error = %v, want 409", err)
	}
	if store.updated.Nickname != "" || guard.claimCalls != 1 {
		t.Fatalf("replayed code changed state: update=%+v claims=%d", store.updated, guard.claimCalls)
	}
}

func TestSyncWeChatProfileRejectsRateLimitedRequest(t *testing.T) {
	store := &fakeSyncIdentityStore{
		fakeStore: &fakeStore{user: db.User{ID: 7, Nickname: DefaultNickname, Avatar: DefaultAvatar, Status: StatusActive}},
		identity:  db.User{ID: 7, Status: StatusActive},
	}
	guard := &fakeWeChatProfileGuard{claimed: true, allowed: false}
	service := NewService(store)
	service.SetWeChatProfileClient(fakeWeChatProfileClient{result: wechatplatform.LoginResult{OpenID: "openid-7"}})
	service.SetWeChatProfileGuard(guard)

	_, err := service.SyncWeChatProfile(context.Background(), 7, WeChatProfileSyncInput{Code: "code-1", Nickname: "新昵称"})
	if !hasHTTPStatus(err, 429) {
		t.Fatalf("error = %v, want 429", err)
	}
	if store.updated.ID != 0 || guard.claimCalls != 1 {
		t.Fatalf("rate-limited request changed state: update = %+v, claims = %d", store.updated, guard.claimCalls)
	}
}

func TestSyncWeChatProfileMapsInvalidWeChatCode(t *testing.T) {
	store := &fakeSyncIdentityStore{fakeStore: &fakeStore{user: db.User{ID: 7, Status: StatusActive}}}
	guard := &fakeWeChatProfileGuard{claimed: true, allowed: true}
	service := NewService(store)
	service.SetWeChatProfileClient(fakeWeChatProfileClient{err: wechatplatform.ErrInvalidCode})
	service.SetWeChatProfileGuard(guard)

	_, err := service.SyncWeChatProfile(context.Background(), 7, WeChatProfileSyncInput{Code: "expired-code", Nickname: "新昵称"})
	if !hasHTTPStatus(err, 401) {
		t.Fatalf("error = %v, want 401", err)
	}
	if guard.claimCalls != 0 {
		t.Fatalf("invalid code was claimed: %d", guard.claimCalls)
	}
}

func newTestWeChatProfileService(store Store, text, image moderation.ProviderResult) *Service {
	service := NewService(store)
	service.SetContentModerator(moderation.NewService(fakeModerationProvider{text: text, image: image}, nil))
	return service
}

type fakeSyncIdentityStore struct {
	*fakeStore
	identity    db.User
	identityErr error
}

func (f *fakeSyncIdentityStore) GetUserByProviderSubject(context.Context, string, string) (db.User, error) {
	if f.identityErr != nil {
		return db.User{}, f.identityErr
	}
	if f.identity.ID == 0 {
		return db.User{}, sql.ErrNoRows
	}
	return f.identity, nil
}

type fakeWeChatProfileClient struct {
	result wechatplatform.LoginResult
	err    error
}

func (f fakeWeChatProfileClient) ExchangeCode(context.Context, string) (wechatplatform.LoginResult, error) {
	return f.result, f.err
}

type fakeWeChatProfileGuard struct {
	claimed    bool
	allowed    bool
	claimCalls int
}

func (f *fakeWeChatProfileGuard) ClaimWeChatProfileCode(context.Context, string, time.Duration) (bool, error) {
	f.claimCalls++
	return f.claimed, nil
}

func (f *fakeWeChatProfileGuard) AllowWeChatProfileSync(context.Context, uint64, int64, time.Duration) (bool, error) {
	return f.allowed, nil
}

type fakeWeChatAvatarFetcher struct {
	data []byte
	err  error
}

func (f fakeWeChatAvatarFetcher) Fetch(context.Context, string, int64) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]byte(nil), f.data...), nil
}
