package admin

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/example/go-service/internal/apperror"
	jwtplatform "github.com/example/go-service/internal/platform/jwt"
)

func TestAdminUserListNormalizesFiltersAndReturnsSafeStats(t *testing.T) {
	createdAt := time.Date(2026, 9, 16, 8, 0, 0, 0, time.UTC)
	store := &fakeAdminUserStore{
		users: []AdminUserRecord{{
			ID: 9, Username: "alice", Nickname: "rejected name", Avatar: "https://example.com/rejected.webp",
			Platform: "wechat", Status: 1, NicknameModerationStatus: "rejected", AvatarModerationStatus: "pending",
			CreatedAt: createdAt, UpdatedAt: createdAt,
		}},
		stats: AdminUserStats{Total: 12, NewToday: 3, Active: 10, Disabled: 2},
	}
	service := NewAdminUserService(store, &fakeAccountBlocker{}, time.Minute)

	result, err := service.List(context.Background(), ListAdminUsersInput{
		Query: "  alice  ", Status: " DISABLED ", Platform: " TAPTAP ", Page: 0, PageSize: 1000,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if store.listInput.Query != "alice" || store.listInput.Status != "disabled" || store.listInput.Platform != "taptap" || store.listInput.Page != 1 || store.listInput.PageSize != 100 {
		t.Fatalf("normalized list input = %+v", store.listInput)
	}
	if result.Page != 1 || result.PageSize != 100 || result.Total != 12 || result.Stats != store.stats {
		t.Fatalf("list metadata = %+v", result)
	}
	item := result.Items[0]
	if item.Nickname == "rejected name" || item.Avatar == "https://example.com/rejected.webp" || item.Nickname != "算术玩家" || item.Avatar != "sun" {
		t.Fatalf("unsafe profile material in list item: %+v", item)
	}
}

func TestAdminUserListUsesPlatformPrecedenceFromStore(t *testing.T) {
	store := &fakeAdminUserStore{
		users: []AdminUserRecord{
			{ID: 1, Platform: "wechat"},
			{ID: 2, Platform: "taptap"},
			{ID: 3, Platform: "password"},
		},
	}
	service := NewAdminUserService(store, &fakeAccountBlocker{}, time.Minute)

	result, err := service.List(context.Background(), ListAdminUsersInput{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	for index, platform := range []string{"wechat", "taptap", "password"} {
		if result.Items[index].Platform != platform {
			t.Fatalf("item %d platform = %q, want %q", index, result.Items[index].Platform, platform)
		}
	}
}

func TestAdminUserDetailExcludesUnsafeProfileMaterial(t *testing.T) {
	updatedAt := time.Date(2026, 9, 16, 9, 0, 0, 0, time.UTC)
	store := &fakeAdminUserStore{users: []AdminUserRecord{{
		ID: 8, Username: "alice", Nickname: "original rejected nickname", Avatar: "https://example.com/rejected.webp",
		Platform: "password", Status: 1, NicknameModerationStatus: "rejected", AvatarModerationStatus: "pending",
		ModerationUpdatedAt: &updatedAt, CreatedAt: updatedAt, UpdatedAt: updatedAt,
	}}}
	service := NewAdminUserService(store, &fakeAccountBlocker{}, time.Minute)

	result, err := service.Detail(context.Background(), 8)
	if err != nil {
		t.Fatalf("Detail() error = %v", err)
	}
	if result.Nickname != "算术玩家" || result.Avatar != "sun" || result.NicknameModerationStatus != "rejected" || result.AvatarModerationStatus != "pending" || result.ModerationUpdatedAt == nil {
		t.Fatalf("detail = %+v", result)
	}
}

func TestAdminUserStatusIsIdempotentAndDoesNotMutateProfile(t *testing.T) {
	createdAt := time.Date(2026, 9, 16, 8, 0, 0, 0, time.UTC)
	store := &fakeAdminUserStore{users: []AdminUserRecord{{
		ID: 7, Username: "alice", Nickname: "safe", Avatar: "sun", Platform: "wechat", Status: 1,
		NicknameModerationStatus: "approved", AvatarModerationStatus: "approved", CreatedAt: createdAt, UpdatedAt: createdAt,
	}}}
	blocker := &fakeAccountBlocker{}
	service := NewAdminUserService(store, blocker, 2*time.Minute)

	for range 2 {
		result, err := service.Disable(context.Background(), 7)
		if err != nil || result.Status != 0 {
			t.Fatalf("Disable() = %+v, %v", result, err)
		}
	}
	for range 2 {
		result, err := service.Enable(context.Background(), 7)
		if err != nil || result.Status != 1 {
			t.Fatalf("Enable() = %+v, %v", result, err)
		}
	}
	account := store.users[0]
	if account.Nickname != "safe" || account.Avatar != "sun" || account.NicknameModerationStatus != "approved" || account.AvatarModerationStatus != "approved" {
		t.Fatalf("status changes mutated profile: %+v", account)
	}
	if blocker.blockCalls != 2 || blocker.unblockCalls != 2 || blocker.role != jwtplatform.RoleUser || blocker.id != 7 || blocker.ttl != 2*time.Minute {
		t.Fatalf("blocker calls = %+v", blocker)
	}
}

func TestAdminUserStatusMapsMissingUserToNotFound(t *testing.T) {
	service := NewAdminUserService(&fakeAdminUserStore{}, &fakeAccountBlocker{}, time.Minute)

	if _, err := service.Disable(context.Background(), 99); err == nil || !isNotFound(err) {
		t.Fatalf("Disable() error = %v, want not found", err)
	}
}

func isNotFound(err error) bool {
	var appErr *apperror.AppError
	return errors.As(err, &appErr) && appErr.HTTPStatus == 404
}

type fakeAdminUserStore struct {
	users     []AdminUserRecord
	stats     AdminUserStats
	listInput ListAdminUsersInput
}

func (s *fakeAdminUserStore) ListAdminUsers(_ context.Context, input ListAdminUsersInput) ([]AdminUserRecord, error) {
	s.listInput = input
	return append([]AdminUserRecord(nil), s.users...), nil
}

func (s *fakeAdminUserStore) GetAdminUserStats(context.Context) (AdminUserStats, error) {
	return s.stats, nil
}

func (s *fakeAdminUserStore) GetAdminUser(_ context.Context, id uint64) (AdminUserRecord, error) {
	for _, account := range s.users {
		if account.ID == id {
			return account, nil
		}
	}
	return AdminUserRecord{}, sql.ErrNoRows
}

func (s *fakeAdminUserStore) SetAdminUserStatus(_ context.Context, id uint64, status uint8, updatedAt time.Time) (AdminUserRecord, error) {
	for index := range s.users {
		if s.users[index].ID == id {
			s.users[index].Status = status
			s.users[index].UpdatedAt = updatedAt
			return s.users[index], nil
		}
	}
	return AdminUserRecord{}, sql.ErrNoRows
}

type fakeAccountBlocker struct {
	blockCalls   int
	unblockCalls int
	role         string
	id           uint64
	ttl          time.Duration
}

func (b *fakeAccountBlocker) BlockAccount(_ context.Context, role string, id uint64, ttl time.Duration) error {
	b.blockCalls++
	b.role, b.id, b.ttl = role, id, ttl
	return nil
}

func (b *fakeAccountBlocker) UnblockAccount(_ context.Context, role string, id uint64) error {
	b.unblockCalls++
	b.role, b.id = role, id
	return nil
}
