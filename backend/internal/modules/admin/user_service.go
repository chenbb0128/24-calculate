package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/example/go-service/internal/apperror"
	"github.com/example/go-service/internal/modules/user"
	jwtplatform "github.com/example/go-service/internal/platform/jwt"
)

type AdminUserStore interface {
	ListAdminUsers(context.Context, ListAdminUsersInput) ([]AdminUserRecord, error)
	GetAdminUserStats(context.Context) (AdminUserStats, error)
	GetAdminUser(context.Context, uint64) (AdminUserRecord, error)
	SetAdminUserStatus(context.Context, uint64, uint8, time.Time) (AdminUserRecord, error)
}

type AdminAccountBlocker interface {
	BlockAccount(context.Context, string, uint64, time.Duration) error
	UnblockAccount(context.Context, string, uint64) error
}

type AdminUserService struct {
	store     AdminUserStore
	blocker   AdminAccountBlocker
	accessTTL time.Duration
}

func NewAdminUserService(store AdminUserStore, blocker AdminAccountBlocker, accessTTL time.Duration) *AdminUserService {
	return &AdminUserService{store: store, blocker: blocker, accessTTL: accessTTL}
}

func (s *AdminUserService) List(ctx context.Context, input ListAdminUsersInput) (AdminUserListResponse, error) {
	input = normalizeAdminUserListInput(input)
	accounts, err := s.store.ListAdminUsers(ctx, input)
	if err != nil {
		return AdminUserListResponse{}, err
	}
	stats, err := s.store.GetAdminUserStats(ctx)
	if err != nil {
		return AdminUserListResponse{}, err
	}
	items := make([]AdminUserListItem, 0, len(accounts))
	for _, account := range accounts {
		items = append(items, toAdminUserListItem(account))
	}
	return AdminUserListResponse{Items: items, Page: input.Page, PageSize: input.PageSize, Total: stats.Total, Stats: stats}, nil
}

func (s *AdminUserService) Detail(ctx context.Context, id uint64) (AdminUserDetailResponse, error) {
	account, err := s.get(ctx, id)
	if err != nil {
		return AdminUserDetailResponse{}, err
	}
	return toAdminUserDetail(account), nil
}

func (s *AdminUserService) Disable(ctx context.Context, id uint64) (AdminUserStatusResponse, error) {
	return s.setStatus(ctx, id, user.StatusDisabled)
}

func (s *AdminUserService) Enable(ctx context.Context, id uint64) (AdminUserStatusResponse, error) {
	return s.setStatus(ctx, id, user.StatusActive)
}

func (s *AdminUserService) setStatus(ctx context.Context, id uint64, status uint8) (AdminUserStatusResponse, error) {
	if id == 0 {
		return AdminUserStatusResponse{}, apperror.BadRequest("用户 ID 无效", nil)
	}
	if s.store == nil {
		return AdminUserStatusResponse{}, fmt.Errorf("admin user store is not initialized")
	}
	if s.blocker == nil || s.accessTTL <= 0 {
		return AdminUserStatusResponse{}, fmt.Errorf("admin user account blocker is not initialized")
	}
	if _, err := s.get(ctx, id); err != nil {
		return AdminUserStatusResponse{}, err
	}
	now := time.Now().UTC()
	account, err := s.store.SetAdminUserStatus(ctx, id, status, now)
	if errors.Is(err, sql.ErrNoRows) {
		return AdminUserStatusResponse{}, apperror.NotFound("用户不存在", err)
	}
	if err != nil {
		return AdminUserStatusResponse{}, err
	}
	if status == user.StatusDisabled {
		if err := s.blocker.BlockAccount(ctx, jwtplatform.RoleUser, id, s.accessTTL); err != nil {
			return AdminUserStatusResponse{}, err
		}
	} else if err := s.blocker.UnblockAccount(ctx, jwtplatform.RoleUser, id); err != nil {
		return AdminUserStatusResponse{}, err
	}
	return AdminUserStatusResponse{ID: account.ID, Status: account.Status, UpdatedAt: formatAdminUserTime(account.UpdatedAt)}, nil
}

func (s *AdminUserService) get(ctx context.Context, id uint64) (AdminUserRecord, error) {
	if id == 0 {
		return AdminUserRecord{}, apperror.BadRequest("用户 ID 无效", nil)
	}
	if s.store == nil {
		return AdminUserRecord{}, fmt.Errorf("admin user store is not initialized")
	}
	account, err := s.store.GetAdminUser(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return AdminUserRecord{}, apperror.NotFound("用户不存在", err)
	}
	return account, err
}

func normalizeAdminUserListInput(input ListAdminUsersInput) ListAdminUsersInput {
	input.Query = strings.TrimSpace(input.Query)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	if input.Status != "active" && input.Status != "disabled" {
		input.Status = "all"
	}
	input.Platform = strings.ToLower(strings.TrimSpace(input.Platform))
	if input.Platform != "wechat" && input.Platform != "taptap" && input.Platform != "password" {
		input.Platform = "all"
	}
	if input.Page < 1 {
		input.Page = 1
	}
	if input.PageSize < 1 {
		input.PageSize = defaultAdminUsersPageSize
	}
	if input.PageSize > maxAdminUsersPageSize {
		input.PageSize = maxAdminUsersPageSize
	}
	return input
}

func toAdminUserListItem(account AdminUserRecord) AdminUserListItem {
	return AdminUserListItem{
		ID: account.ID, Username: account.Username,
		Nickname: user.SafePublicNickname(account.Nickname, account.NicknameModerationStatus),
		Avatar:   user.SafePublicAvatar(account.Avatar, account.AvatarModerationStatus),
		Platform: account.Platform, Status: account.Status,
		NicknameModerationStatus: account.NicknameModerationStatus, AvatarModerationStatus: account.AvatarModerationStatus,
		CreatedAt: formatAdminUserTime(account.CreatedAt), UpdatedAt: formatAdminUserTime(account.UpdatedAt),
	}
}

func toAdminUserDetail(account AdminUserRecord) AdminUserDetailResponse {
	item := toAdminUserListItem(account)
	result := AdminUserDetailResponse{
		ID: item.ID, Username: item.Username, Nickname: item.Nickname, Avatar: item.Avatar, Platform: item.Platform, Status: item.Status,
		NicknameModerationStatus: item.NicknameModerationStatus, AvatarModerationStatus: item.AvatarModerationStatus,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
	if account.ModerationUpdatedAt != nil {
		value := formatAdminUserTime(*account.ModerationUpdatedAt)
		result.ModerationUpdatedAt = &value
	}
	return result
}

func formatAdminUserTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
