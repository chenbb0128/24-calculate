package user

import (
	"context"
	"fmt"

	"github.com/example/go-service/internal/store"
	db "github.com/example/go-service/internal/store/sqlc"
)

type Repository struct {
	queries *db.Queries
	tx      *store.TxManager
}

func NewRepository(queries *db.Queries, tx *store.TxManager) *Repository {
	return &Repository{queries: queries, tx: tx}
}

func (r *Repository) GetUserByID(ctx context.Context, id uint64) (db.User, error) {
	if r == nil || r.queries == nil {
		return db.User{}, fmt.Errorf("user repository is not initialized")
	}
	return r.queries.GetUserByID(ctx, id)
}

func (r *Repository) GetUserByUsername(ctx context.Context, username string) (db.User, error) {
	if r == nil || r.queries == nil {
		return db.User{}, fmt.Errorf("user repository is not initialized")
	}
	return r.queries.GetUserByUsername(ctx, username)
}

func (r *Repository) GetUserByProviderSubject(ctx context.Context, provider, subject string) (db.User, error) {
	if r == nil || r.queries == nil {
		return db.User{}, fmt.Errorf("user repository is not initialized")
	}
	return r.queries.GetUserByProviderSubject(ctx, db.GetUserByProviderSubjectParams{
		Provider:        provider,
		ProviderSubject: subject,
	})
}

func (r *Repository) CreateUser(ctx context.Context, arg db.CreateUserParams) (uint64, error) {
	if r == nil || r.queries == nil {
		return 0, fmt.Errorf("user repository is not initialized")
	}
	result, err := r.queries.CreateUser(ctx, arg)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get created user id: %w", err)
	}
	return uint64(id), nil
}

func (r *Repository) CreateUserTx(ctx context.Context, arg db.CreateUserParams) (uint64, error) {
	if r == nil || r.tx == nil {
		return 0, fmt.Errorf("user repository transaction manager is not initialized")
	}
	var id uint64
	if err := r.tx.Exec(ctx, func(queries *db.Queries) error {
		result, err := queries.CreateUser(ctx, arg)
		if err != nil {
			return err
		}
		lastID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("get created user id: %w", err)
		}
		id = uint64(lastID)
		return nil
	}); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *Repository) CreateUserWithIdentityTx(ctx context.Context, userArg db.CreateUserParams, identityArg db.CreateUserIdentityParams) (uint64, error) {
	if r == nil || r.tx == nil {
		return 0, fmt.Errorf("user repository transaction manager is not initialized")
	}
	var id uint64
	if err := r.tx.Exec(ctx, func(queries *db.Queries) error {
		result, err := queries.CreateUser(ctx, userArg)
		if err != nil {
			return err
		}
		lastID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("get created user id: %w", err)
		}
		id = uint64(lastID)
		identityArg.UserID = id
		return queries.CreateUserIdentity(ctx, identityArg)
	}); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *Repository) UpdateUserProfile(ctx context.Context, arg db.UpdateUserProfileParams) error {
	if r == nil || r.queries == nil {
		return fmt.Errorf("user repository is not initialized")
	}
	return r.queries.UpdateUserProfile(ctx, arg)
}
