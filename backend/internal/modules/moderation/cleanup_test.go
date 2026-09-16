package moderation

import (
	"context"
	"testing"

	db "github.com/example/go-service/internal/store/sqlc"
)

func TestCleanupDryRunDoesNotUpdateOrCreateRewards(t *testing.T) {
	store := &cleanupStore{users: []db.User{{
		ID: 7, Nickname: "待整改昵称", Avatar: "sun", Status: 1,
		NicknameModerationStatus: string(StatusUnreviewed), AvatarModerationStatus: string(StatusApproved),
	}}}
	service := NewService(cleanupProvider{result: ProviderResult{Status: StatusRejected, ReasonCode: "87014"}}, nil)

	report, err := service.Cleanup(context.Background(), store, true, 50)
	if err != nil {
		t.Fatalf("Cleanup(dry-run) error = %v", err)
	}
	if report.Scanned != 1 || report.Updated != 1 || len(store.updates) != 0 {
		t.Fatalf("dry-run report = %+v, updates = %#v", report, store.updates)
	}
}

func TestCleanupIsIdempotentForAlreadyApprovedProfile(t *testing.T) {
	store := &cleanupStore{users: []db.User{{
		ID: 7, Nickname: "安全昵称", Avatar: "sun", Status: 1,
		NicknameModerationStatus: string(StatusApproved), AvatarModerationStatus: string(StatusApproved),
	}}}
	service := NewService(cleanupProvider{result: ProviderResult{Status: StatusApproved}}, nil)

	first, err := service.Cleanup(context.Background(), store, false, 50)
	if err != nil {
		t.Fatalf("first Cleanup() error = %v", err)
	}
	second, err := service.Cleanup(context.Background(), store, false, 50)
	if err != nil {
		t.Fatalf("second Cleanup() error = %v", err)
	}
	if first.Updated != 0 || second.Updated != 0 || len(store.updates) != 0 {
		t.Fatalf("approved profile was updated: first=%+v second=%+v updates=%#v", first, second, store.updates)
	}
}

func TestCleanupReportsProviderFailuresAndHiddenAvatars(t *testing.T) {
	store := &cleanupStore{users: []db.User{{
		ID: 8, Username: "legacy-8", Nickname: "待审核昵称", Avatar: "https://old.example/avatar.webp", Status: 1,
		NicknameModerationStatus: string(StatusUnreviewed), AvatarModerationStatus: string(StatusUnreviewed),
	}}}
	service := NewService(cleanupProvider{err: context.DeadlineExceeded}, nil)

	report, err := service.Cleanup(context.Background(), store, false, 50)
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if report.Failed != 1 || report.AvatarsHidden != 1 {
		t.Fatalf("cleanup report = %+v, want one failure and one hidden avatar", report)
	}
	if len(store.updates) != 1 || store.updates[0].Avatar != cleanupDefaultAvatar {
		t.Fatalf("cleanup updates = %#v, want the unsafe avatar replaced", store.updates)
	}
}

type cleanupStore struct {
	users   []db.User
	updates []db.UpdateUserModerationParams
}

func (f *cleanupStore) ListUsersForModeration(_ context.Context, afterID uint64, limit int) ([]db.User, error) {
	result := make([]db.User, 0, limit)
	for _, item := range f.users {
		if item.ID > afterID && len(result) < limit {
			result = append(result, item)
		}
	}
	return result, nil
}

func (f *cleanupStore) UpdateUserModeration(_ context.Context, arg db.UpdateUserModerationParams) error {
	f.updates = append(f.updates, arg)
	for index := range f.users {
		if f.users[index].ID == arg.ID {
			f.users[index].Nickname = arg.Nickname
			f.users[index].Avatar = arg.Avatar
			f.users[index].NicknameModerationStatus = arg.NicknameModerationStatus
			f.users[index].AvatarModerationStatus = arg.AvatarModerationStatus
			f.users[index].ModerationUpdatedAt = arg.ModerationUpdatedAt
			f.users[index].UpdatedAt = arg.UpdatedAt
		}
	}
	return nil
}

type cleanupProvider struct {
	result ProviderResult
	err    error
}

func (f cleanupProvider) CheckText(context.Context, TextCheckRequest) (ProviderResult, error) {
	return f.result, f.err
}

func (f cleanupProvider) CheckImage(context.Context, ImageCheckRequest) (ProviderResult, error) {
	return ProviderResult{Status: StatusApproved}, nil
}
