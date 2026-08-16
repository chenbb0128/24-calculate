package user

import (
	"context"

	db "github.com/example/go-service/internal/store/sqlc"
)

type Store interface {
	GetUserByID(ctx context.Context, id uint64) (db.User, error)
	GetUserByUsername(ctx context.Context, username string) (db.User, error)
	CreateUser(ctx context.Context, arg db.CreateUserParams) (uint64, error)
	UpdateUserProfile(ctx context.Context, arg db.UpdateUserProfileParams) error
}
