package moderation

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	db "github.com/example/go-service/internal/store/sqlc"
)

const (
	cleanupDefaultNickname  = "算术玩家"
	cleanupDefaultAvatar    = "sun"
	cleanupMaxNicknameRunes = 12
)

type CleanupReport struct {
	Scanned       int `json:"scanned"`
	Updated       int `json:"updated"`
	Approved      int `json:"approved"`
	Rejected      int `json:"rejected"`
	Unreviewed    int `json:"unreviewed"`
	Failed        int `json:"failed"`
	AvatarsHidden int `json:"avatars_hidden"`
	CacheChanges  int `json:"cache_changes"`
}

// Cleanup reviews legacy profile values without touching player progress,
// rewards, rankings or caches. A dry run reports the same decisions while
// leaving the database unchanged.
func (s *Service) Cleanup(ctx context.Context, store UserStore, dryRun bool, pageSize int) (CleanupReport, error) {
	if s == nil || s.provider == nil {
		return CleanupReport{}, fmt.Errorf("moderation provider is unavailable")
	}
	if store == nil {
		return CleanupReport{}, fmt.Errorf("moderation user store is unavailable")
	}
	if pageSize <= 0 {
		pageSize = 100
	}
	if pageSize > 500 {
		return CleanupReport{}, fmt.Errorf("moderation cleanup page size is invalid")
	}

	var report CleanupReport
	var afterID uint64
	for {
		users, err := store.ListUsersForModeration(ctx, afterID, pageSize)
		if err != nil {
			return report, err
		}
		if len(users) == 0 {
			return report, nil
		}
		for _, account := range users {
			report.Scanned++
			if account.ID > afterID {
				afterID = account.ID
			}
			decision := s.reviewLegacyUser(ctx, account, dryRun)
			if decision.updated {
				report.Updated++
			}
			if decision.failed {
				report.Failed++
			}
			if decision.avatarHidden {
				report.AvatarsHidden++
			}
			switch decision.nicknameStatus {
			case StatusApproved:
				report.Approved++
			case StatusRejected:
				report.Rejected++
			default:
				report.Unreviewed++
			}
			if !dryRun && decision.updated {
				now := time.Now().UTC()
				if err := store.UpdateUserModeration(ctx, db.UpdateUserModerationParams{
					Nickname:                 decision.nickname,
					Avatar:                   decision.avatar,
					NicknameModerationStatus: string(decision.nicknameStatus),
					AvatarModerationStatus:   string(decision.avatarStatus),
					ModerationUpdatedAt:      &now,
					UpdatedAt:                now,
					ID:                       account.ID,
				}); err != nil {
					return report, err
				}
			}
		}
		if len(users) < pageSize {
			return report, nil
		}
	}
}

type cleanupDecision struct {
	nickname, avatar              string
	nicknameStatus, avatarStatus  Status
	updated, failed, avatarHidden bool
}

func (s *Service) reviewLegacyUser(ctx context.Context, account db.User, dryRun bool) cleanupDecision {
	nickname := strings.TrimSpace(account.Nickname)
	if nickname == "" {
		nickname = cleanupDefaultNickname
	}
	nicknameStatus := Status(strings.TrimSpace(account.NicknameModerationStatus))
	failed := false
	avatar := strings.TrimSpace(account.Avatar)
	originalAvatar := avatar
	if avatar == "" {
		avatar = cleanupDefaultAvatar
	}
	avatarStatus := Status(strings.TrimSpace(account.AvatarModerationStatus))

	if nicknameStatus != StatusApproved || !validCleanupNickname(nickname) {
		candidate, normalizeErr := NormalizeText(nickname)
		if normalizeErr != nil || !validCleanupNickname(candidate) {
			nickname = cleanupDefaultNickname
			nicknameStatus = StatusRejected
		} else {
			decision, err := s.moderateText(ctx, account.ID, account.Username, "moderation_cleanup", candidate, !dryRun)
			switch {
			case err != nil || decision.Status == StatusUnavailable:
				nickname = cleanupDefaultNickname
				nicknameStatus = StatusUnreviewed
				failed = true
			case decision.Status == StatusApproved:
				nickname = candidate
				nicknameStatus = StatusApproved
			case decision.Status == StatusRejected:
				nickname = cleanupDefaultNickname
				nicknameStatus = StatusRejected
			default:
				nickname = cleanupDefaultNickname
				nicknameStatus = decision.Status
			}
		}
	}

	// Presets are server-owned and need no upstream image call. Historical
	// external/unreviewed avatars are hidden and replaced with the safe preset;
	// this command intentionally never fetches arbitrary URLs or creates an
	// SSRF path just to inspect legacy data.
	if avatarStatus == StatusApproved {
		if avatar == "" {
			avatar = cleanupDefaultAvatar
		}
	} else if isCleanupAvatarPreset(avatar) {
		avatar = strings.TrimSpace(avatar)
		avatarStatus = StatusApproved
	} else {
		avatar = cleanupDefaultAvatar
		if avatarStatus != StatusRejected {
			avatarStatus = StatusUnreviewed
		}
	}
	avatarHidden := originalAvatar != "" && avatar == cleanupDefaultAvatar && originalAvatar != cleanupDefaultAvatar

	return cleanupDecision{
		nickname: nickname, avatar: avatar,
		nicknameStatus: nicknameStatus, avatarStatus: avatarStatus,
		updated: nickname != account.Nickname || avatar != account.Avatar || string(nicknameStatus) != account.NicknameModerationStatus || string(avatarStatus) != account.AvatarModerationStatus,
		failed:  failed, avatarHidden: avatarHidden,
	}
}

func validCleanupNickname(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len([]rune(value)) > cleanupMaxNicknameRunes {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || strings.ContainsRune("<>\\{}", r) {
			return false
		}
	}
	return true
}

func isCleanupAvatarPreset(value string) bool {
	switch strings.TrimSpace(value) {
	case "sun", "star", "rocket", "target", "rainbow", "spark":
		return true
	default:
		return false
	}
}
