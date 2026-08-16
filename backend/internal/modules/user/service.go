package user

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/example/go-service/internal/apperror"
	db "github.com/example/go-service/internal/store/sqlc"
)

const (
	StatusDisabled = 0
	StatusActive   = 1
)

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) GetProfile(ctx context.Context, id uint64) (ProfileResponse, error) {
	user, err := s.store.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProfileResponse{}, NotFound(err)
		}
		return ProfileResponse{}, err
	}
	if user.Status == StatusDisabled {
		return ProfileResponse{}, Disabled(nil)
	}
	return toProfileResponse(user), nil
}

func (s *Service) UpdateProfile(ctx context.Context, id uint64, input UpdateProfileInput) (ProfileResponse, error) {
	user, err := s.store.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProfileResponse{}, NotFound(err)
		}
		return ProfileResponse{}, err
	}
	if user.Status == StatusDisabled {
		return ProfileResponse{}, Disabled(nil)
	}

	nickname := user.Nickname
	avatar := user.Avatar
	if input.Nickname != nil {
		nickname = strings.TrimSpace(*input.Nickname)
		if len([]rune(nickname)) > 100 {
			return ProfileResponse{}, invalidProfile("nickname 长度不能超过 100 个字符")
		}
	}
	if input.Avatar != nil {
		avatar = strings.TrimSpace(*input.Avatar)
		if len([]rune(avatar)) > 500 {
			return ProfileResponse{}, invalidProfile("avatar 长度不能超过 500 个字符")
		}
	}

	now := time.Now().UTC()
	if err := s.store.UpdateUserProfile(ctx, db.UpdateUserProfileParams{
		Nickname:  nickname,
		Avatar:    avatar,
		UpdatedAt: now,
		ID:        id,
	}); err != nil {
		return ProfileResponse{}, err
	}

	user.Nickname = nickname
	user.Avatar = avatar
	user.UpdatedAt = now
	return toProfileResponse(user), nil
}

func toProfileResponse(user db.User) ProfileResponse {
	return ProfileResponse{
		ID:        user.ID,
		Username:  user.Username,
		Nickname:  user.Nickname,
		Avatar:    user.Avatar,
		Status:    user.Status,
		CreatedAt: user.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: user.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func invalidProfile(message string) error {
	return apperror.BadRequest(message, nil)
}
