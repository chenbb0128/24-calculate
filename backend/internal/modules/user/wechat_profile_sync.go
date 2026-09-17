package user

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/example/go-service/internal/modules/moderation"
	wechatplatform "github.com/example/go-service/internal/platform/wechat"
	db "github.com/example/go-service/internal/store/sqlc"
)

const (
	wechatProfileCodeTTL    = 10 * time.Minute
	wechatProfileSyncLimit  = 3
	wechatProfileSyncWindow = time.Minute
)

type WeChatProfileInput struct {
	Nickname string
	Avatar   string
}

type WeChatProfileSyncInput struct {
	Code     string `json:"code"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

type WeChatProfileResult struct {
	Profile         ProfileResponse `json:"user"`
	SyncStatus      string          `json:"sync_status"`
	NicknameUpdated bool            `json:"nickname_updated"`
	AvatarUpdated   bool            `json:"avatar_updated"`
}

type WeChatIdentityStore interface {
	GetUserByProviderSubject(context.Context, string, string) (db.User, error)
}

type WeChatProfileClient interface {
	ExchangeCode(context.Context, string) (wechatplatform.LoginResult, error)
}

type WeChatProfileGuard interface {
	ClaimWeChatProfileCode(context.Context, string, time.Duration) (bool, error)
	AllowWeChatProfileSync(context.Context, uint64, int64, time.Duration) (bool, error)
}

type WeChatAvatarFetcher interface {
	Fetch(context.Context, string, int64) ([]byte, error)
}

type WeChatProfileSynchronizer interface {
	SyncAuthorizedWeChatProfile(context.Context, uint64, string, WeChatProfileInput, string) (WeChatProfileResult, error)
}

func (s *Service) SetWeChatProfileClient(client WeChatProfileClient) {
	if s != nil {
		s.wechatProfileClient = client
	}
}

func (s *Service) SetWeChatProfileGuard(guard WeChatProfileGuard) {
	if s != nil {
		s.wechatProfileGuard = guard
	}
}

func (s *Service) SetWeChatAvatarFetcher(fetcher WeChatAvatarFetcher) {
	if s != nil {
		s.wechatAvatarFetcher = fetcher
	}
}

func (s *Service) SyncAuthorizedWeChatProfile(ctx context.Context, userID uint64, subject string, input WeChatProfileInput, source string) (WeChatProfileResult, error) {
	if s == nil || userID == 0 {
		return WeChatProfileResult{}, WeChatProfileUnavailable(errors.New("user service is not initialized"))
	}
	account, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return WeChatProfileResult{}, NotFound(err)
		}
		return WeChatProfileResult{}, err
	}
	if account.Status == StatusDisabled {
		return WeChatProfileResult{}, Disabled(nil)
	}

	nickname := strings.TrimSpace(account.Nickname)
	if nickname == "" {
		nickname = DefaultNickname
	}
	avatar := strings.TrimSpace(account.Avatar)
	if avatar == "" {
		avatar = DefaultAvatar
	}
	nicknameStatus := strings.TrimSpace(account.NicknameModerationStatus)
	avatarStatus := strings.TrimSpace(account.AvatarModerationStatus)
	resultStatus := moderation.StatusApproved
	nicknameUpdated := false
	avatarUpdated := false
	newStoredAvatarKey := ""
	oldAvatar := strings.TrimSpace(account.Avatar)

	if strings.TrimSpace(input.Nickname) != "" {
		candidate, candidateStatus := s.moderateNickname(ctx, userID, subject, input.Nickname, source)
		resultStatus = stricterProfileStatus(resultStatus, candidateStatus)
		if candidateStatus == moderation.StatusApproved {
			nicknameUpdated = candidate != nickname
			nickname = candidate
			nicknameStatus = string(moderation.StatusApproved)
		}
	}

	if strings.TrimSpace(input.Avatar) != "" {
		stored, candidateStatus, candidateErr := s.prepareWeChatAvatar(ctx, userID, input.Avatar, source)
		resultStatus = stricterProfileStatus(resultStatus, candidateStatus)
		if candidateErr == nil && candidateStatus == moderation.StatusApproved {
			avatar = stored.URL
			avatarStatus = string(moderation.StatusApproved)
			avatarUpdated = avatar != oldAvatar
			if !IsAllowedAvatar(stored.URL) {
				newStoredAvatarKey = stored.Key
			}
		}
	}

	if !nicknameUpdated && !avatarUpdated {
		account.Nickname = nickname
		account.Avatar = avatar
		account.NicknameModerationStatus = nicknameStatus
		account.AvatarModerationStatus = avatarStatus
		return WeChatProfileResult{
			Profile:    toProfileResponse(account),
			SyncStatus: string(resultStatus),
		}, nil
	}

	now := time.Now().UTC()
	if err := s.store.UpdateUserProfile(ctx, db.UpdateUserProfileParams{
		Nickname:                 nickname,
		Avatar:                   avatar,
		NicknameModerationStatus: nicknameStatus,
		AvatarModerationStatus:   avatarStatus,
		ModerationUpdatedAt:      &now,
		UpdatedAt:                now,
		ID:                       userID,
	}); err != nil {
		if newStoredAvatarKey != "" && s.avatarStorage != nil {
			_ = s.avatarStorage.Delete(context.Background(), newStoredAvatarKey)
		}
		return WeChatProfileResult{}, err
	}
	if newStoredAvatarKey != "" && oldAvatar != "" && oldAvatar != DefaultAvatar && oldAvatar != avatar && s.avatarStorage != nil {
		go func(value string) {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = s.avatarStorage.Delete(cleanupCtx, value)
		}(oldAvatar)
	}
	account.Nickname = nickname
	account.Avatar = avatar
	account.NicknameModerationStatus = nicknameStatus
	account.AvatarModerationStatus = avatarStatus
	account.ModerationUpdatedAt = &now
	account.UpdatedAt = now
	return WeChatProfileResult{
		Profile:         toProfileResponse(account),
		SyncStatus:      string(resultStatus),
		NicknameUpdated: nicknameUpdated,
		AvatarUpdated:   avatarUpdated,
	}, nil
}

func (s *Service) SyncWeChatProfile(ctx context.Context, userID uint64, input WeChatProfileSyncInput) (WeChatProfileResult, error) {
	code := strings.TrimSpace(input.Code)
	if code == "" || len(code) > 512 {
		return WeChatProfileResult{}, InvalidWeChatProfileCode(nil)
	}
	if strings.TrimSpace(input.Nickname) == "" && strings.TrimSpace(input.Avatar) == "" {
		return WeChatProfileResult{}, invalidProfile("至少提供昵称或头像")
	}
	if s == nil || s.wechatProfileClient == nil || s.wechatProfileGuard == nil {
		return WeChatProfileResult{}, WeChatProfileUnavailable(errors.New("WeChat profile dependencies are unavailable"))
	}
	loginResult, err := s.wechatProfileClient.ExchangeCode(ctx, code)
	if err != nil {
		if errors.Is(err, wechatplatform.ErrInvalidCode) {
			return WeChatProfileResult{}, InvalidWeChatProfileCode(err)
		}
		return WeChatProfileResult{}, WeChatProfileUnavailable(err)
	}
	subject := strings.TrimSpace(loginResult.OpenID)
	if subject == "" {
		return WeChatProfileResult{}, InvalidWeChatProfileCode(nil)
	}
	identityStore, ok := s.store.(WeChatIdentityStore)
	if !ok {
		return WeChatProfileResult{}, WeChatProfileUnavailable(errors.New("user store does not support identities"))
	}
	account, err := identityStore.GetUserByProviderSubject(ctx, "wechat", subject)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return WeChatProfileResult{}, WeChatProfileIdentityMismatch(err)
		}
		return WeChatProfileResult{}, err
	}
	if account.ID == 0 || account.ID != userID {
		return WeChatProfileResult{}, WeChatProfileIdentityMismatch(nil)
	}
	digest := sha256.Sum256([]byte(code))
	claimed, err := s.wechatProfileGuard.ClaimWeChatProfileCode(ctx, hex.EncodeToString(digest[:]), wechatProfileCodeTTL)
	if err != nil {
		return WeChatProfileResult{}, WeChatProfileUnavailable(err)
	}
	if !claimed {
		return WeChatProfileResult{}, WeChatProfileCodeUsed(nil)
	}
	allowed, err := s.wechatProfileGuard.AllowWeChatProfileSync(ctx, userID, wechatProfileSyncLimit, wechatProfileSyncWindow)
	if err != nil {
		return WeChatProfileResult{}, WeChatProfileUnavailable(err)
	}
	if !allowed {
		return WeChatProfileResult{}, WeChatProfileRateLimited(nil)
	}
	return s.SyncAuthorizedWeChatProfile(ctx, userID, subject, WeChatProfileInput{
		Nickname: input.Nickname,
		Avatar:   input.Avatar,
	}, "wechat_profile_sync")
}

func (s *Service) moderateNickname(ctx context.Context, userID uint64, subject, value, source string) (string, moderation.Status) {
	candidate, err := NormalizeNickname(value)
	if err != nil {
		return DefaultNickname, moderation.StatusRejected
	}
	if s.moderator == nil {
		return DefaultNickname, moderation.StatusUnavailable
	}
	decision, err := s.moderator.ModerateText(ctx, userID, subject, source, candidate)
	if err != nil {
		return DefaultNickname, moderation.StatusUnavailable
	}
	switch decision.Status {
	case moderation.StatusApproved:
		return candidate, moderation.StatusApproved
	case moderation.StatusPending:
		return DefaultNickname, moderation.StatusPending
	case moderation.StatusRejected:
		return DefaultNickname, moderation.StatusRejected
	default:
		return DefaultNickname, moderation.StatusUnavailable
	}
}

func (s *Service) prepareWeChatAvatar(ctx context.Context, userID uint64, rawURL, source string) (StoredAvatar, moderation.Status, error) {
	normalized, err := NormalizeWeChatAvatar(rawURL)
	if err != nil {
		return StoredAvatar{}, moderation.StatusRejected, err
	}
	if IsAllowedAvatar(normalized) {
		return StoredAvatar{URL: normalized, Key: normalized}, moderation.StatusApproved, nil
	}
	if s.wechatAvatarFetcher == nil || s.avatarStorage == nil {
		return StoredAvatar{}, moderation.StatusUnavailable, errors.New("WeChat avatar dependencies are unavailable")
	}
	raw, err := s.wechatAvatarFetcher.Fetch(ctx, normalized, s.avatarMaxBytes)
	if err != nil {
		return StoredAvatar{}, moderation.StatusUnavailable, err
	}
	encoded, _, _, err := processAvatarImage(raw, s.avatarMaxDimension)
	if err != nil {
		return StoredAvatar{}, moderation.StatusRejected, err
	}
	if s.moderator == nil {
		return StoredAvatar{}, moderation.StatusUnavailable, errors.New("content moderator is unavailable")
	}
	decision, err := s.moderator.ModerateImage(ctx, userID, source, encoded)
	if err != nil {
		return StoredAvatar{}, moderation.StatusUnavailable, err
	}
	if decision.Status != moderation.StatusApproved {
		return StoredAvatar{}, decision.Status, errors.New("WeChat avatar moderation rejected")
	}
	stored, err := s.avatarStorage.Save(ctx, userID, encoded)
	if err != nil {
		return StoredAvatar{}, moderation.StatusUnavailable, err
	}
	// A successful storage write is not enough: the published value must be a
	// same-user URL on the configured HTTPS avatar origin. This also prevents a
	// development instance with an empty base URL from publishing a relative
	// path that the client cannot safely use as a public avatar.
	if normalizedStoredURL, validationErr := s.normalizeProfileAvatar(stored.URL, userID); validationErr != nil {
		if stored.Key != "" {
			_ = s.avatarStorage.Delete(context.Background(), stored.Key)
		}
		return StoredAvatar{}, moderation.StatusUnavailable, errors.New("avatar public URL is not configured")
	} else {
		stored.URL = normalizedStoredURL
	}
	return stored, moderation.StatusApproved, nil
}

func stricterProfileStatus(current, candidate moderation.Status) moderation.Status {
	rank := func(status moderation.Status) int {
		switch status {
		case moderation.StatusUnavailable:
			return 4
		case moderation.StatusRejected:
			return 3
		case moderation.StatusPending:
			return 2
		case moderation.StatusApproved:
			return 1
		default:
			return 4
		}
	}
	if rank(candidate) > rank(current) {
		return candidate
	}
	return current
}
