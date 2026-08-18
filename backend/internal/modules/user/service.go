package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/example/go-service/internal/apperror"
	db "github.com/example/go-service/internal/store/sqlc"
)

const (
	StatusDisabled = 0
	StatusActive   = 1

	DefaultNickname  = "算术玩家"
	DefaultAvatar    = "sun"
	MaxNicknameRunes = 12
	MaxAvatarRunes   = 500
)

var allowedAvatars = map[string]struct{}{
	"sun": {}, "star": {}, "rocket": {}, "target": {}, "rainbow": {}, "spark": {},
}

type AvatarRateLimiter interface {
	AllowAvatarUpload(context.Context, uint64, int64, time.Duration) (bool, error)
}

type Service struct {
	store              Store
	avatarStorage      AvatarStorage
	avatarRateLimiter  AvatarRateLimiter
	avatarMaxBytes     int64
	avatarMaxDimension int
	uploadCooldown     time.Duration
	uploadMu           sync.Mutex
	lastAvatarUploads  map[uint64]time.Time
}

func NewService(store Store) *Service {
	return NewServiceWithAvatarStorage(store, nil, 2<<20, 4096, 30*time.Second)
}

func NewServiceWithAvatarStorage(store Store, avatarStorage AvatarStorage, maxBytes int64, maxDimension int, uploadCooldown time.Duration) *Service {
	if maxBytes <= 0 {
		maxBytes = 2 << 20
	}
	if uploadCooldown <= 0 {
		uploadCooldown = 30 * time.Second
	}
	if maxDimension <= 0 {
		maxDimension = 4096
	}
	return &Service{
		store:              store,
		avatarStorage:      avatarStorage,
		avatarMaxBytes:     maxBytes,
		avatarMaxDimension: maxDimension,
		uploadCooldown:     uploadCooldown,
		lastAvatarUploads:  make(map[uint64]time.Time),
	}
}

func (s *Service) SetAvatarRateLimiter(limiter AvatarRateLimiter) {
	if s != nil {
		s.avatarRateLimiter = limiter
	}
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
		var err error
		nickname, err = NormalizeNickname(*input.Nickname)
		if err != nil {
			return ProfileResponse{}, invalidProfile(err.Error())
		}
	}
	if input.Avatar != nil {
		var err error
		avatar, err = NormalizeAvatar(*input.Avatar)
		if err != nil {
			return ProfileResponse{}, invalidProfile(err.Error())
		}
	}
	// Old accounts may contain empty fields from before profile defaults were
	// introduced. Normalize them on the next profile read/update as well.
	if strings.TrimSpace(nickname) == "" {
		nickname = DefaultNickname
	}
	if strings.TrimSpace(avatar) == "" {
		avatar = DefaultAvatar
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

func (s *Service) UploadAvatar(ctx context.Context, id uint64, data []byte, maxDimension int) (AvatarUploadResponse, error) {
	if s.avatarStorage == nil {
		return AvatarUploadResponse{}, apperror.ServiceUnavailable("头像上传暂未配置", nil)
	}
	if len(data) == 0 || int64(len(data)) > s.avatarMaxBytes {
		return AvatarUploadResponse{}, invalidProfile(fmt.Sprintf("头像文件不能超过 %d MB", s.avatarMaxBytes/(1<<20)))
	}
	if maxDimension <= 0 {
		maxDimension = s.avatarMaxDimension
	}
	user, err := s.store.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AvatarUploadResponse{}, NotFound(err)
		}
		return AvatarUploadResponse{}, err
	}
	if user.Status == StatusDisabled {
		return AvatarUploadResponse{}, Disabled(nil)
	}
	if s.avatarRateLimiter != nil {
		allowed, limitErr := s.avatarRateLimiter.AllowAvatarUpload(ctx, id, 2, time.Hour)
		if limitErr != nil {
			return AvatarUploadResponse{}, limitErr
		}
		if !allowed {
			return AvatarUploadResponse{}, apperror.New(10006, 429, "头像上传过于频繁，请稍后再试", nil)
		}
	} else if !s.allowAvatarUpload(id, time.Now().UTC()) {
		return AvatarUploadResponse{}, apperror.New(10006, 429, "头像上传过于频繁，请稍后再试", nil)
	}
	encoded, width, height, err := processAvatarImage(data, maxDimension)
	if err != nil {
		return AvatarUploadResponse{}, invalidProfile(err.Error())
	}
	stored, err := s.avatarStorage.Save(ctx, id, encoded)
	if err != nil {
		return AvatarUploadResponse{}, apperror.ServiceUnavailable("头像保存失败", err)
	}
	nickname := strings.TrimSpace(user.Nickname)
	if nickname == "" {
		nickname = DefaultNickname
	}
	oldAvatar := strings.TrimSpace(user.Avatar)
	now := time.Now().UTC()
	if err := s.store.UpdateUserProfile(ctx, db.UpdateUserProfileParams{
		Nickname:  nickname,
		Avatar:    stored.URL,
		UpdatedAt: now,
		ID:        id,
	}); err != nil {
		_ = s.avatarStorage.Delete(context.Background(), stored.Key)
		return AvatarUploadResponse{}, err
	}
	if oldAvatar != "" && oldAvatar != DefaultAvatar && oldAvatar != stored.URL {
		go func(value string) {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := s.avatarStorage.Delete(cleanupCtx, value); err != nil {
				// Cleanup is deliberately best effort; the new profile is already valid.
			}
		}(oldAvatar)
	}
	user.Avatar = stored.URL
	user.UpdatedAt = now
	return AvatarUploadResponse{AvatarURL: stored.URL, AvatarKey: stored.Key, Width: width, Height: height, Format: "webp", Profile: toProfileResponse(user)}, nil
}

func (s *Service) allowAvatarUpload(id uint64, now time.Time) bool {
	s.uploadMu.Lock()
	defer s.uploadMu.Unlock()
	last, exists := s.lastAvatarUploads[id]
	if exists && now.Sub(last) < s.uploadCooldown {
		return false
	}
	s.lastAvatarUploads[id] = now
	return true
}

func toProfileResponse(user db.User) ProfileResponse {
	nickname := strings.TrimSpace(user.Nickname)
	if nickname == "" {
		nickname = DefaultNickname
	}
	avatar := strings.TrimSpace(user.Avatar)
	if avatar == "" {
		avatar = DefaultAvatar
	}
	return ProfileResponse{
		ID:        user.ID,
		Username:  user.Username,
		Nickname:  nickname,
		Avatar:    avatar,
		Status:    user.Status,
		CreatedAt: user.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: user.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func invalidProfile(message string) error {
	return apperror.BadRequest(message, nil)
}

// NormalizeNickname validates a user-supplied nickname and applies the
// default when the client explicitly sends an empty value. Keeping this in
// the user module makes WeChat login and the authenticated profile endpoint
// enforce exactly the same rules.
func NormalizeNickname(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return DefaultNickname, nil
	}
	if len([]rune(value)) < 1 || len([]rune(value)) > MaxNicknameRunes {
		return "", fmt.Errorf("nickname 长度必须为 1 到 %d 个字符", MaxNicknameRunes)
	}
	lower := strings.ToLower(value)
	for _, marker := range []string{"<script", "</script", "javascript:", "data:text", "vbscript:"} {
		if strings.Contains(lower, marker) {
			return "", fmt.Errorf("nickname 包含不允许的内容")
		}
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || strings.ContainsRune("<>\\{}", r) {
			return "", fmt.Errorf("nickname 包含不允许的字符")
		}
	}
	return value, nil
}

// NormalizeAvatar accepts one of the built-in avatar identifiers or an HTTPS
// image URL returned by WeChat/object storage. Empty input resets to the
// built-in default avatar.
func NormalizeAvatar(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return DefaultAvatar, nil
	}
	if len([]rune(value)) > MaxAvatarRunes {
		return "", fmt.Errorf("avatar 长度不能超过 %d 个字符", MaxAvatarRunes)
	}
	if _, ok := allowedAvatars[value]; ok {
		return value, nil
	}
	if strings.HasPrefix(value, "/avatars/") && !strings.Contains(value, "..") && strings.HasSuffix(value, ".webp") {
		return value, nil
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return "", fmt.Errorf("avatar 必须是预设头像或 HTTPS 图片地址")
	}
	return value, nil
}

func IsAllowedAvatar(value string) bool {
	_, ok := allowedAvatars[value]
	return ok
}
