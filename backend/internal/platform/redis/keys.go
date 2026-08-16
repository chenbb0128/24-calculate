package redis

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const keyPrefix = "go-service"

func RefreshTokenKey(jti string) string {
	return fmt.Sprintf("%s:auth:refresh:%s", keyPrefix, jti)
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

func FriendRoomMatchProgressStateKey(roomCode string) string {
	return fmt.Sprintf("%s:friend:room:%s:progress-state", keyPrefix, roomCode)
}

func FriendRoomMatchProgressEventsKey(roomCode string) string {
	return fmt.Sprintf("%s:friend:room:%s:progress-events", keyPrefix, roomCode)
}

func FriendRoomMatchResultsKey(roomCode string) string {
	return fmt.Sprintf("%s:friend:room:%s:results", keyPrefix, roomCode)
}

func FriendRoomRateKey(action string, userID uint64) string {
	return fmt.Sprintf("%s:rate:friend:%s:%d", keyPrefix, action, userID)
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
