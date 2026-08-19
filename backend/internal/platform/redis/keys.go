package redis

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const keyPrefix = "twenty-four-calculate"

func RefreshTokenKey(jti string) string {
	return fmt.Sprintf("%s:auth:refresh:%s", keyPrefix, jti)
}

func AccessTokenRevokedKey(jti string) string {
	return fmt.Sprintf("%s:auth:access-revoked:%s", keyPrefix, jti)
}

func LoginRateKey(ip string) string {
	digest := sha256.Sum256([]byte(ip))
	return fmt.Sprintf("%s:rate:login:%s", keyPrefix, hex.EncodeToString(digest[:]))
}

func WelcomeTaskKey(userID uint64) string {
	return fmt.Sprintf("%s:task:user-welcome:%d", keyPrefix, userID)
}

func FriendRoomKey(roomCode string) string {
	return fmt.Sprintf("%s:friend:room:%s", keyPrefix, roomCode)
}

func FriendRoomPlayersKey(roomCode string) string {
	return fmt.Sprintf("%s:friend:room:%s:players", keyPrefix, roomCode)
}

func FriendRoomStateKey(roomCode string) string {
	return fmt.Sprintf("%s:friend:room:%s:state", keyPrefix, roomCode)
}

func FriendRoomReadyKey(roomCode string) string {
	return fmt.Sprintf("%s:friend:room:%s:ready", keyPrefix, roomCode)
}

func FriendRoomLastSeenKey(roomCode string) string {
	return fmt.Sprintf("%s:friend:room:%s:last-seen", keyPrefix, roomCode)
}

func FriendRoomMatchProgressKey(roomCode string) string {
	return fmt.Sprintf("%s:friend:room:%s:progress", keyPrefix, roomCode)
}

// The round-scoped keys keep a rematch from seeing progress or submissions
// written by a previous round. An empty round id deliberately falls back to
// the legacy key so old rooms can finish without a data migration.
func FriendRoomMatchProgressRoundKey(roomCode, roundID string) string {
	if roundID == "" {
		return FriendRoomMatchProgressKey(roomCode)
	}
	return fmt.Sprintf("%s:friend:room:%s:round:%s:progress", keyPrefix, roomCode, roundID)
}

func FriendRoomMatchProgressStateKey(roomCode string) string {
	return fmt.Sprintf("%s:friend:room:%s:progress-state", keyPrefix, roomCode)
}

func FriendRoomMatchProgressStateRoundKey(roomCode, roundID string) string {
	if roundID == "" {
		return FriendRoomMatchProgressStateKey(roomCode)
	}
	return fmt.Sprintf("%s:friend:room:%s:round:%s:progress-state", keyPrefix, roomCode, roundID)
}

func FriendRoomMatchProgressEventsKey(roomCode string) string {
	return fmt.Sprintf("%s:friend:room:%s:progress-events", keyPrefix, roomCode)
}

func FriendRoomMatchProgressEventsRoundKey(roomCode, roundID string) string {
	if roundID == "" {
		return FriendRoomMatchProgressEventsKey(roomCode)
	}
	return fmt.Sprintf("%s:friend:room:%s:round:%s:progress-events", keyPrefix, roomCode, roundID)
}

func FriendRoomMatchResultsKey(roomCode string) string {
	return fmt.Sprintf("%s:friend:room:%s:results", keyPrefix, roomCode)
}

func FriendRoomMatchResultsRoundKey(roomCode, roundID string) string {
	if roundID == "" {
		return FriendRoomMatchResultsKey(roomCode)
	}
	return fmt.Sprintf("%s:friend:room:%s:round:%s:results", keyPrefix, roomCode, roundID)
}

func FriendRoomRematchKey(roomCode, idempotencyKey string) string {
	digest := sha256.Sum256([]byte(idempotencyKey))
	return fmt.Sprintf("%s:friend:room:%s:rematch:%s", keyPrefix, roomCode, hex.EncodeToString(digest[:]))
}

func FriendBotRoomsKey() string {
	return fmt.Sprintf("%s:friend:bot-rooms", keyPrefix)
}

func FriendRoomRateKey(action string, userID uint64) string {
	return fmt.Sprintf("%s:rate:friend:%s:%d", keyPrefix, action, userID)
}

func FriendRecentPuzzleKey(userID uint64) string {
	return fmt.Sprintf("%s:friend:recent-puzzles:%d", keyPrefix, userID)
}

func AvatarUploadRateKey(userID uint64) string {
	return fmt.Sprintf("%s:rate:avatar-upload:%d", keyPrefix, userID)
}

func EndlessRunKey(runID string) string {
	return fmt.Sprintf("%s:endless:run:%s", keyPrefix, runID)
}

func CampaignRunKey(runID string) string {
	return fmt.Sprintf("%s:campaign:run:%s", keyPrefix, runID)
}

func DailyRunKey(runID string) string {
	return fmt.Sprintf("%s:daily:run:%s", keyPrefix, runID)
}

func MatchmakingQueueKey(mode string) string {
	return fmt.Sprintf("%s:matchmaking:%s:queue", keyPrefix, mode)
}

func MatchmakingTicketKey(mode, ticketID string) string {
	return fmt.Sprintf("%s:matchmaking:%s:ticket:%s", keyPrefix, mode, ticketID)
}

func MatchmakingUserKey(mode string, userID uint64) string {
	return fmt.Sprintf("%s:matchmaking:%s:user:%d", keyPrefix, mode, userID)
}

func DistributedLockKey(scope string) string {
	digest := sha256.Sum256([]byte(scope))
	return fmt.Sprintf("%s:lock:%s", keyPrefix, hex.EncodeToString(digest[:]))
}
