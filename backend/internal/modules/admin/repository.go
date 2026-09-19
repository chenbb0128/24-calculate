package admin

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	db "github.com/example/go-service/internal/store/sqlc"
)

type AdminAccountStore interface {
	CreateAdminAccount(context.Context, db.CreateAdminAccountParams) (sql.Result, error)
	UpdateAdminPassword(context.Context, db.UpdateAdminPasswordParams) (int64, error)
}

type AdminRepository struct {
	queries *db.Queries
}

func NewRepository(queries *db.Queries) *AdminRepository {
	return &AdminRepository{queries: queries}
}

func (r *AdminRepository) CreateAdminAccount(ctx context.Context, arg db.CreateAdminAccountParams) (sql.Result, error) {
	if r == nil || r.queries == nil {
		return nil, fmt.Errorf("admin repository is not initialized")
	}
	return r.queries.CreateAdminAccount(ctx, arg)
}

func (r *AdminRepository) UpdateAdminPassword(ctx context.Context, arg db.UpdateAdminPasswordParams) (int64, error) {
	if r == nil || r.queries == nil {
		return 0, fmt.Errorf("admin repository is not initialized")
	}
	return r.queries.UpdateAdminPassword(ctx, arg)
}

func (r *AdminRepository) GetAdminAccountByID(ctx context.Context, id uint64) (db.AdminAccount, error) {
	if r == nil || r.queries == nil {
		return db.AdminAccount{}, fmt.Errorf("admin repository is not initialized")
	}
	return r.queries.GetAdminAccountByID(ctx, id)
}

func (r *AdminRepository) GetAdminAccountByUsername(ctx context.Context, username string) (db.AdminAccount, error) {
	if r == nil || r.queries == nil {
		return db.AdminAccount{}, fmt.Errorf("admin repository is not initialized")
	}
	return r.queries.GetAdminAccountByUsername(ctx, username)
}

func (r *AdminRepository) TouchAdminLastLogin(ctx context.Context, arg db.TouchAdminLastLoginParams) error {
	if r == nil || r.queries == nil {
		return fmt.Errorf("admin repository is not initialized")
	}
	return r.queries.TouchAdminLastLogin(ctx, arg)
}

func (r *AdminRepository) ListAdminUsers(ctx context.Context, input ListAdminUsersInput) ([]AdminUserRecord, error) {
	if r == nil || r.queries == nil {
		return nil, fmt.Errorf("admin repository is not initialized")
	}
	offset := int64(input.Page-1) * int64(input.PageSize)
	rows, err := r.queries.ListAdminUsers(ctx, db.ListAdminUsersParams{
		Query: input.Query, Status: input.Status, Platform: input.Platform,
		Limit: int32(input.PageSize), Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}
	items := make([]AdminUserRecord, 0, len(rows))
	for _, row := range rows {
		items = append(items, adminUserRecordFromListRow(row))
	}
	return items, nil
}

func (r *AdminRepository) CountAdminUsers(ctx context.Context, input ListAdminUsersInput) (int64, error) {
	if r == nil || r.queries == nil {
		return 0, fmt.Errorf("admin repository is not initialized")
	}
	return r.queries.CountAdminUsers(ctx, db.CountAdminUsersParams{
		Query: input.Query, Status: input.Status, Platform: input.Platform,
	})
}

func (r *AdminRepository) GetAdminUserStats(ctx context.Context) (AdminUserStats, error) {
	if r == nil || r.queries == nil {
		return AdminUserStats{}, fmt.Errorf("admin repository is not initialized")
	}
	stats, err := r.queries.GetAdminUserStats(ctx)
	if err != nil {
		return AdminUserStats{}, err
	}
	return AdminUserStats{Total: stats.Total, NewToday: stats.NewToday, Active: stats.Active, Disabled: stats.Disabled}, nil
}

func (r *AdminRepository) GetAdminUser(ctx context.Context, id uint64) (AdminUserRecord, error) {
	if r == nil || r.queries == nil {
		return AdminUserRecord{}, fmt.Errorf("admin repository is not initialized")
	}
	row, err := r.queries.GetAdminUser(ctx, id)
	if err != nil {
		return AdminUserRecord{}, err
	}
	return adminUserRecordFromDetailRow(row), nil
}

func (r *AdminRepository) SetAdminUserStatus(ctx context.Context, id uint64, status uint8, updatedAt time.Time) (AdminUserRecord, error) {
	if r == nil || r.queries == nil {
		return AdminUserRecord{}, fmt.Errorf("admin repository is not initialized")
	}
	updated, err := r.queries.UpdateAdminUserStatus(ctx, db.UpdateAdminUserStatusParams{ID: id, Status: status, UpdatedAt: updatedAt})
	if err != nil {
		return AdminUserRecord{}, err
	}
	if updated == 0 {
		return AdminUserRecord{}, sql.ErrNoRows
	}
	return r.GetAdminUser(ctx, id)
}

func adminUserRecordFromListRow(row db.ListAdminUsersRow) AdminUserRecord {
	return adminUserRecord(row.ID, row.Username, row.Nickname, row.Avatar, row.Platform, row.Status,
		row.NicknameModerationStatus, row.AvatarModerationStatus, row.ModerationUpdatedAt, row.CreatedAt, row.UpdatedAt)
}

func adminUserRecordFromDetailRow(row db.GetAdminUserRow) AdminUserRecord {
	return adminUserRecord(row.ID, row.Username, row.Nickname, row.Avatar, row.Platform, row.Status,
		row.NicknameModerationStatus, row.AvatarModerationStatus, row.ModerationUpdatedAt, row.CreatedAt, row.UpdatedAt)
}

func adminUserRecord(id uint64, username, nickname, avatar, platform string, status uint8, nicknameStatus, avatarStatus string, moderationUpdatedAt sql.NullTime, createdAt, updatedAt time.Time) AdminUserRecord {
	result := AdminUserRecord{
		ID: id, Username: username, Nickname: nickname, Avatar: avatar, Platform: platform, Status: status,
		NicknameModerationStatus: nicknameStatus, AvatarModerationStatus: avatarStatus, CreatedAt: createdAt, UpdatedAt: updatedAt,
	}
	if moderationUpdatedAt.Valid {
		value := moderationUpdatedAt.Time
		result.ModerationUpdatedAt = &value
	}
	return result
}
