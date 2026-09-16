package admin

import (
	"context"
	"database/sql"
	"fmt"

	db "github.com/example/go-service/internal/store/sqlc"
)

type AdminAccountStore interface {
	CreateAdminAccount(context.Context, db.CreateAdminAccountParams) (sql.Result, error)
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
