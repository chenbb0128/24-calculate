package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/example/go-service/internal/apperror"
	"github.com/example/go-service/internal/modules/moderation"
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
	store               Store
	avatarStorage       AvatarStorage
	avatarRateLimiter   AvatarRateLimiter
	avatarMaxBytes      int64
	avatarMaxDimension  int
	uploadCooldown      time.Duration
	uploadMu            sync.Mutex
	lastAvatarUploads   map[uint64]time.Time
	avatarPublicBaseURL string
	logger              *slog.Logger
	moderator           *moderation.Service
	wechatProfileClient WeChatProfileClient
	wechatProfileGuard  WeChatProfileGuard
	wechatAvatarFetcher WeChatAvatarFetcher
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
		logger:             slog.Default(),
	}
}

func (s *Service) SetAvatarRateLimiter(limiter AvatarRateLimiter) {
	if s != nil {
		s.avatarRateLimiter = limiter
	}
}

func (s *Service) SetAvatarPublicBaseURL(value string) {
	if s != nil {
		s.avatarPublicBaseURL = strings.TrimRight(strings.TrimSpace(value), "/")
	}
}

func (s *Service) SetLogger(logger *slog.Logger) {
	if s != nil && logger != nil {
		s.logger = logger
	}
}

func (s *Service) SetContentModerator(moderator *moderation.Service) {
	if s != nil {
		s.moderator = moderator
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
	if input.Nickname != nil {
		return ProfileResponse{}, apperror.NewBusiness("NICKNAME_EDIT_DISABLED", http.StatusBadRequest, "昵称暂时无法修改", nil)
	}
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
	nicknameStatus := user.NicknameModerationStatus
	avatarStatus := user.AvatarModerationStatus
	if input.Avatar != nil {
		var err error
		avatar, err = s.normalizeProfileAvatar(*input.Avatar, id)
		if err != nil {
			return ProfileResponse{}, invalidProfile(err.Error())
		}
		avatarStatus = string(moderation.StatusApproved)
	}
	// Old accounts may contain empty fields from before profile defaults were
	// introduced. Normalize them on the next profile read/update as well.
	if strings.TrimSpace(nickname) == "" {
		nickname = DefaultNickname
	}
	if strings.TrimSpace(avatar) == "" {
		avatar = DefaultAvatar
	}
	if nicknameStatus == "" {
		nicknameStatus = string(moderation.StatusApproved)
	}
	if avatarStatus == "" {
		avatarStatus = string(moderation.StatusApproved)
	}

	now := time.Now().UTC()
	if err := s.store.UpdateUserProfile(ctx, db.UpdateUserProfileParams{
		Nickname:                 nickname,
		Avatar:                   avatar,
		NicknameModerationStatus: nicknameStatus,
		AvatarModerationStatus:   avatarStatus,
		ModerationUpdatedAt:      &now,
		UpdatedAt:                now,
		ID:                       id,
	}); err != nil {
		return ProfileResponse{}, err
	}

	user.Nickname = nickname
	user.Avatar = avatar
	user.NicknameModerationStatus = nicknameStatus
	user.AvatarModerationStatus = avatarStatus
	user.ModerationUpdatedAt = &now
	user.UpdatedAt = now
	return toProfileResponse(user), nil
}

func (s *Service) UploadAvatar(ctx context.Context, id uint64, data []byte, maxDimension int) (AvatarUploadResponse, error) {
	detectedFormat := detectAvatarFormat(data)
	logUpload := func(success bool, reason, avatarURL string) {
		if s != nil && s.logger != nil {
			s.logger.InfoContext(ctx, "avatar upload", "user_id", id, "bytes", len(data), "format", detectedFormat, "saved", success, "avatar_url", avatarURL, "reason", reason)
		}
	}
	if s.avatarStorage == nil {
		logUpload(false, "storage_not_configured", "")
		return AvatarUploadResponse{}, apperror.ServiceUnavailable("头像上传暂未配置", nil)
	}
	if len(data) == 0 {
		logUpload(false, "empty_file", "")
		return AvatarUploadResponse{}, invalidProfile("头像文件不能为空")
	}
	if int64(len(data)) > s.avatarMaxBytes {
		logUpload(false, "file_too_large_or_empty", "")
		return AvatarUploadResponse{}, AvatarTooLarge(nil)
	}
	if maxDimension <= 0 {
		maxDimension = s.avatarMaxDimension
	}
	user, err := s.store.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logUpload(false, "user_not_found", "")
			return AvatarUploadResponse{}, NotFound(err)
		}
		logUpload(false, "load_user_failed", "")
		return AvatarUploadResponse{}, err
	}
	if user.Status == StatusDisabled {
		logUpload(false, "user_disabled", "")
		return AvatarUploadResponse{}, Disabled(nil)
	}
	if s.avatarRateLimiter != nil {
		allowed, limitErr := s.avatarRateLimiter.AllowAvatarUpload(ctx, id, 1, s.uploadCooldown)
		if limitErr != nil {
			logUpload(false, "rate_limiter_failed", "")
			return AvatarUploadResponse{}, limitErr
		}
		if !allowed {
			logUpload(false, "rate_limited", "")
			return AvatarUploadResponse{}, apperror.New(10006, 429, "头像上传过于频繁，请稍后再试", nil)
		}
	} else if !s.allowAvatarUpload(id, time.Now().UTC()) {
		logUpload(false, "rate_limited", "")
		return AvatarUploadResponse{}, apperror.New(10006, 429, "头像上传过于频繁，请稍后再试", nil)
	}
	encoded, width, height, err := processAvatarImage(data, maxDimension)
	if err != nil {
		logUpload(false, "invalid_image", "")
		return AvatarUploadResponse{}, invalidProfile(err.Error())
	}
	if s.moderator != nil {
		decision, moderationErr := s.moderator.ModerateImage(ctx, id, "avatar_upload", encoded)
		if moderationErr != nil || decision.Status == moderation.StatusUnavailable {
			logUpload(false, "moderation_provider_unavailable", "")
			return AvatarUploadResponse{}, apperror.NewBusiness("MODERATION_PROVIDER_UNAVAILABLE", http.StatusServiceUnavailable, "内容审核服务暂不可用", moderationErr)
		}
		if decision.Status == moderation.StatusRejected {
			logUpload(false, "moderation_rejected", "")
			return AvatarUploadResponse{}, apperror.NewBusiness("AVATAR_REJECTED", http.StatusBadRequest, "头像未通过内容审核，请更换后再试", nil)
		}
		if decision.Status == moderation.StatusPending {
			logUpload(false, "moderation_pending", "")
			return AvatarUploadResponse{ModerationStatus: string(moderation.StatusPending), Profile: toProfileResponse(user)}, nil
		}
	}
	stored, err := s.avatarStorage.Save(ctx, id, encoded)
	if err != nil {
		logUpload(false, "storage_save_failed", "")
		return AvatarUploadResponse{}, apperror.ServiceUnavailable("头像保存失败", err)
	}
	nickname := strings.TrimSpace(user.Nickname)
	if nickname == "" {
		nickname = DefaultNickname
	}
	nicknameStatus := strings.TrimSpace(user.NicknameModerationStatus)
	if nicknameStatus == "" {
		nicknameStatus = string(moderation.StatusApproved)
	}
	oldAvatar := strings.TrimSpace(user.Avatar)
	now := time.Now().UTC()
	if err := s.store.UpdateUserProfile(ctx, db.UpdateUserProfileParams{
		Nickname:                 nickname,
		Avatar:                   stored.URL,
		NicknameModerationStatus: nicknameStatus,
		AvatarModerationStatus:   string(moderation.StatusApproved),
		ModerationUpdatedAt:      &now,
		UpdatedAt:                now,
		ID:                       id,
	}); err != nil {
		_ = s.avatarStorage.Delete(context.Background(), stored.Key)
		logUpload(false, "profile_update_failed", "")
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
	user.NicknameModerationStatus = nicknameStatus
	user.AvatarModerationStatus = string(moderation.StatusApproved)
	user.ModerationUpdatedAt = &now
	user.UpdatedAt = now
	logUpload(true, "", stored.URL)
	return AvatarUploadResponse{AvatarURL: stored.URL, AvatarKey: stored.Key, Width: width, Height: height, Format: "webp", ModerationStatus: string(moderation.StatusApproved), Profile: toProfileResponse(user)}, nil
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
	return ProfileResponse{
		ID:                       user.ID,
		Username:                 user.Username,
		Nickname:                 SafePublicNickname(user.Nickname, user.NicknameModerationStatus),
		Avatar:                   SafePublicAvatar(user.Avatar, user.AvatarModerationStatus),
		Status:                   user.Status,
		CreatedAt:                user.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:                user.UpdatedAt.UTC().Format(time.RFC3339),
		NicknameModerationStatus: user.NicknameModerationStatus,
		AvatarModerationStatus:   user.AvatarModerationStatus,
	}
}

func SafePublicNickname(value, status string) string {
	if !moderation.IsPubliclyApproved(moderation.Status(strings.TrimSpace(status))) {
		return DefaultNickname
	}
	nickname, err := NormalizeNickname(value)
	if err != nil {
		return DefaultNickname
	}
	return nickname
}

func SafePublicAvatar(value, status string) string {
	if !moderation.IsPubliclyApproved(moderation.Status(strings.TrimSpace(status))) {
		return DefaultAvatar
	}
	avatar, err := NormalizeAvatar(value)
	if err == nil {
		return avatar
	}
	// Third-party provider URLs are never public profile values. The sync
	// service downloads and stores them under our own avatar path first.
	return DefaultAvatar
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

// NormalizeAvatar accepts one of the built-in avatar identifiers or a
// syntactically valid HTTPS backend-avatar URL. Callers that have an
// authenticated user must use Service.UpdateProfile, which additionally
// checks the configured host and the user's ownership of the path.
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
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || !isBackendAvatarPath(parsed.Path) || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("avatar 必须是预设头像或 HTTPS 图片地址")
	}
	return value, nil
}

// NormalizeWeChatAvatar is used only for the server response received during
// WeChat privacy authorization. It accepts WeChat's HTTPS avatar hosts, while
// the authenticated profile endpoint accepts only our own generated avatars.
func NormalizeWeChatAvatar(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return DefaultAvatar, nil
	}
	if len([]rune(value)) > MaxAvatarRunes {
		return "", fmt.Errorf("avatar 长度不能超过 %d 个字符", MaxAvatarRunes)
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || !isWeChatAvatarHost(parsed.Hostname()) {
		return "", fmt.Errorf("avatar 必须是微信安全头像地址或预设头像")
	}
	return value, nil
}

func (s *Service) normalizeProfileAvatar(value string, userID uint64) (string, error) {
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
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || !isBackendAvatarPath(parsed.Path) {
		return "", fmt.Errorf("avatar 必须是后端生成的 HTTPS 头像地址或预设头像")
	}
	base, baseErr := url.Parse(s.avatarPublicBaseURL)
	if baseErr != nil || base.Scheme != "https" || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" || !strings.EqualFold(parsed.Scheme, base.Scheme) || !strings.EqualFold(parsed.Host, base.Host) {
		return "", fmt.Errorf("avatar 必须是后端生成的 HTTPS 头像地址或预设头像")
	}
	prefix := "/avatars/" + strconv.FormatUint(userID, 10) + "/"
	if !strings.HasPrefix(parsed.Path, prefix) {
		return "", fmt.Errorf("avatar 不属于当前用户")
	}
	return value, nil
}

func isBackendAvatarPath(value string) bool {
	cleaned := path.Clean("/" + strings.TrimSpace(value))
	return strings.HasPrefix(cleaned, "/avatars/") && !strings.Contains(value, "..") && strings.HasSuffix(cleaned, ".webp")
}

func isWeChatAvatarHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "qlogo.cn" || strings.HasSuffix(host, ".qlogo.cn")
}

func IsAllowedAvatar(value string) bool {
	_, ok := allowedAvatars[value]
	return ok
}
