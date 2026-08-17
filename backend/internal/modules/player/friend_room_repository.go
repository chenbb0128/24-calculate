package player

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	goRedis "github.com/redis/go-redis/v9"

	redisplatform "github.com/example/go-service/internal/platform/redis"
)

var (
	ErrFriendRoomNotFound                 = errors.New("friend room not found")
	ErrFriendRoomFull                     = errors.New("friend room is full")
	ErrFriendRoomCodeTaken                = errors.New("friend room code is already taken")
	ErrFriendRoomAlreadyIn                = errors.New("player is already in friend room")
	ErrFriendRoomCannotJoinSelf           = errors.New("room owner cannot join own friend room")
	ErrFriendRoomStarted                  = errors.New("friend room has already started")
	ErrFriendRoomExpired                  = errors.New("friend room has expired")
	ErrFriendRoomCancelled                = errors.New("friend room has been cancelled")
	ErrFriendRoomNotMember                = errors.New("player is not a member of friend room")
	ErrFriendRoomNotReady                 = errors.New("friend room is not ready")
	ErrFriendRoomAlreadyStarted           = errors.New("friend room has already been started")
	ErrFriendMatchSubmissionAlreadyExists = errors.New("friend match submission already exists")
	ErrFriendMatchProgressStale           = errors.New("friend match progress is stale")
)

const friendRoomJoinScript = `
if redis.call('EXISTS', KEYS[1]) == 0 then
    return -2
end
local status = redis.call('GET', KEYS[2])
if status == 'expired' then
    return -3
end
if status == 'cancelled' then
    return -4
end
if status == 'countdown' or status == 'running' or status == 'finished' then
    return -5
end
if redis.call('HEXISTS', KEYS[3], ARGV[1]) == 1 then
    return 0
end
if tonumber(redis.call('HLEN', KEYS[3])) >= 2 then
    return -1
end
redis.call('HSET', KEYS[3], ARGV[1], ARGV[2])
redis.call('HSET', KEYS[4], ARGV[1], '0')
redis.call('HSET', KEYS[5], ARGV[1], ARGV[4])
redis.call('EXPIRE', KEYS[3], ARGV[3])
redis.call('EXPIRE', KEYS[4], ARGV[3])
redis.call('EXPIRE', KEYS[5], ARGV[3])
return 1
`

const friendRoomReadyScript = `
if redis.call('EXISTS', KEYS[1]) == 0 then
    return -2
end
local status = redis.call('GET', KEYS[2])
if status == 'expired' then return -3 end
if status == 'cancelled' then return -4 end
if status == 'countdown' or status == 'running' or status == 'finished' then return -5 end
if redis.call('HEXISTS', KEYS[3], ARGV[1]) == 0 then return -6 end
redis.call('HSET', KEYS[4], ARGV[1], ARGV[2])
redis.call('HSET', KEYS[5], ARGV[1], ARGV[3])
local count = tonumber(redis.call('HLEN', KEYS[3]))
local allReady = count == 2
if allReady then
    local values = redis.call('HVALS', KEYS[4])
    for _, value in ipairs(values) do
        if value ~= '1' then allReady = false break end
    end
end
if allReady then
    redis.call('SET', KEYS[2], 'ready', 'EX', ARGV[4])
else
    redis.call('SET', KEYS[2], 'waiting', 'EX', ARGV[4])
end
redis.call('EXPIRE', KEYS[4], ARGV[4])
redis.call('EXPIRE', KEYS[5], ARGV[4])
return 1
`

const friendRoomStartScript = `
if redis.call('EXISTS', KEYS[1]) == 0 then return -2 end
local status = redis.call('GET', KEYS[2])
if status == 'expired' then return -3 end
if status == 'cancelled' then return -4 end
if status == 'countdown' or status == 'running' or status == 'finished' then return 0 end
if redis.call('HEXISTS', KEYS[3], ARGV[3]) == 0 then return -5 end
if redis.call('HGET', KEYS[4], ARGV[3]) ~= '1' then return -6 end
if tonumber(redis.call('HLEN', KEYS[3])) ~= 2 then return -7 end
local values = redis.call('HVALS', KEYS[4])
for _, value in ipairs(values) do
    if value ~= '1' then return -7 end
end
redis.call('SET', KEYS[1], ARGV[1], 'EX', ARGV[2])
redis.call('SET', KEYS[2], 'countdown', 'EX', ARGV[2])
return 1
`

const friendMatchProgressScript = `
if ARGV[5] ~= '' and redis.call('SISMEMBER', KEYS[3], ARGV[5]) == 1 then
    return 0
end
local old = redis.call('HGET', KEYS[1], ARGV[1])
if old then
    local oldIndex, oldSolved, oldScore, oldElapsed, oldFinished = string.match(old, '^(%-?%d+)|(%-?%d+)|(%-?%d+)|(%-?%d+)|(%d+)$')
    if oldIndex and (tonumber(ARGV[6]) < tonumber(oldIndex) or tonumber(ARGV[7]) < tonumber(oldSolved) or tonumber(ARGV[8]) < tonumber(oldScore) or tonumber(ARGV[9]) < tonumber(oldElapsed) or (tonumber(oldFinished) == 1 and tonumber(ARGV[10]) == 0)) then
        return -1
    end
end
redis.call('HSET', KEYS[1], ARGV[1], ARGV[2])
redis.call('HSET', KEYS[2], ARGV[1], ARGV[3])
if ARGV[5] ~= '' then redis.call('SADD', KEYS[3], ARGV[5]) end
redis.call('EXPIRE', KEYS[1], ARGV[4])
redis.call('EXPIRE', KEYS[2], ARGV[4])
redis.call('EXPIRE', KEYS[3], ARGV[4])
return 1
`

type FriendRoomRepository struct {
	redis *redisplatform.Client
}

func NewFriendRoomRepository(redis *redisplatform.Client) *FriendRoomRepository {
	return &FriendRoomRepository{redis: redis}
}

func (r *FriendRoomRepository) AcquireDistributedLock(ctx context.Context, scope string, ttl time.Duration) (string, bool, error) {
	if r == nil || r.redis == nil {
		return "", false, fmt.Errorf("friend room redis repository is not initialized")
	}
	return r.redis.AcquireDistributedLock(ctx, scope, ttl)
}

func (r *FriendRoomRepository) ReleaseDistributedLock(ctx context.Context, scope, token string) error {
	if r == nil || r.redis == nil {
		return fmt.Errorf("friend room redis repository is not initialized")
	}
	return r.redis.ReleaseDistributedLock(ctx, scope, token)
}

func (r *FriendRoomRepository) AllowFriendRoomAction(ctx context.Context, userID uint64, action string, limit int64, window time.Duration) (bool, error) {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return false, fmt.Errorf("friend room redis repository is not initialized")
	}
	if limit <= 0 || window <= 0 {
		return false, fmt.Errorf("friend room rate limit configuration is invalid")
	}
	key := redisplatform.FriendRoomRateKey(action, userID)
	count, err := r.redis.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if count == 1 {
		if err := r.redis.Expire(ctx, key, window).Err(); err != nil {
			return false, err
		}
	}
	return count <= limit, nil
}

func (r *FriendRoomRepository) GetRecentFriendPuzzleHashes(ctx context.Context, userID uint64) (map[string]struct{}, error) {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return nil, fmt.Errorf("friend room redis repository is not initialized")
	}
	values, err := r.redis.SMembers(ctx, redisplatform.FriendRecentPuzzleKey(userID)).Result()
	if err != nil {
		return nil, err
	}
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		if len(value) == 64 {
			result[value] = struct{}{}
		}
	}
	return result, nil
}

func (r *FriendRoomRepository) RecordFriendPuzzleHashes(ctx context.Context, userID uint64, hashes []string, ttl time.Duration) error {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return fmt.Errorf("friend room redis repository is not initialized")
	}
	if len(hashes) == 0 || ttl <= 0 {
		return nil
	}
	values := make([]interface{}, 0, len(hashes))
	for _, hash := range hashes {
		if len(hash) == 64 {
			values = append(values, hash)
		}
	}
	if len(values) == 0 {
		return nil
	}
	key := redisplatform.FriendRecentPuzzleKey(userID)
	if err := r.redis.SAdd(ctx, key, values...).Err(); err != nil {
		return err
	}
	return r.redis.Expire(ctx, key, ttl).Err()
}

func (r *FriendRoomRepository) TouchFriendRoomPlayer(ctx context.Context, roomCode string, userID uint64) error {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return fmt.Errorf("friend room redis repository is not initialized")
	}
	if exists, err := r.redis.Exists(ctx, redisplatform.FriendRoomKey(roomCode)).Result(); err != nil {
		return err
	} else if exists == 0 {
		return ErrFriendRoomNotFound
	}
	playerKey := strconv.FormatUint(userID, 10)
	if exists, err := r.redis.HExists(ctx, redisplatform.FriendRoomPlayersKey(roomCode), playerKey).Result(); err != nil {
		return err
	} else if !exists {
		return ErrFriendRoomNotMember
	}
	lastSeenKey := redisplatform.FriendRoomLastSeenKey(roomCode)
	if err := r.redis.HSet(ctx, lastSeenKey, playerKey, time.Now().UTC().UnixMilli()).Err(); err != nil {
		return err
	}
	return r.redis.Expire(ctx, lastSeenKey, friendRoomTTL).Err()
}

func (r *FriendRoomRepository) CreateFriendRoom(ctx context.Context, room FriendRoom) error {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return fmt.Errorf("friend room redis repository is not initialized")
	}
	payload, err := json.Marshal(room)
	if err != nil {
		return fmt.Errorf("encode friend room: %w", err)
	}
	roomKey := redisplatform.FriendRoomKey(room.RoomCode)
	playersKey := redisplatform.FriendRoomPlayersKey(room.RoomCode)
	stateKey := redisplatform.FriendRoomStateKey(room.RoomCode)
	readyKey := redisplatform.FriendRoomReadyKey(room.RoomCode)
	lastSeenKey := redisplatform.FriendRoomLastSeenKey(room.RoomCode)
	ok, err := r.redis.SetNX(ctx, roomKey, payload, friendRoomTTL).Result()
	if err != nil {
		return err
	}
	if !ok {
		return ErrFriendRoomCodeTaken
	}

	if len(room.Players) == 0 {
		_ = r.redis.Del(ctx, roomKey).Err()
		return fmt.Errorf("friend room owner is missing")
	}
	owner := room.Players[0]
	ownerPayload, err := json.Marshal(owner)
	if err != nil {
		_ = r.redis.Del(ctx, roomKey).Err()
		return fmt.Errorf("encode friend room owner: %w", err)
	}
	if err := r.redis.HSet(ctx, playersKey, strconv.FormatUint(owner.UserID, 10), ownerPayload).Err(); err != nil {
		_ = r.redis.Del(ctx, roomKey, playersKey, stateKey, readyKey, lastSeenKey).Err()
		return err
	}
	if err := r.redis.Set(ctx, stateKey, room.Status, friendRoomTTL).Err(); err != nil {
		_ = r.redis.Del(ctx, roomKey, playersKey, stateKey, readyKey, lastSeenKey).Err()
		return err
	}
	if err := r.redis.HSet(ctx, readyKey, strconv.FormatUint(owner.UserID, 10), boolToRedis(room.Players[0].Ready)).Err(); err != nil {
		_ = r.redis.Del(ctx, roomKey, playersKey, stateKey, readyKey, lastSeenKey).Err()
		return err
	}
	if err := r.redis.HSet(ctx, lastSeenKey, strconv.FormatUint(owner.UserID, 10), owner.LastSeenAt.UnixMilli()).Err(); err != nil {
		_ = r.redis.Del(ctx, roomKey, playersKey, stateKey, readyKey, lastSeenKey).Err()
		return err
	}
	if err := r.redis.Expire(ctx, playersKey, friendRoomTTL).Err(); err != nil {
		_ = r.redis.Del(ctx, roomKey, playersKey, stateKey, readyKey, lastSeenKey).Err()
		return err
	}
	_ = r.redis.Expire(ctx, readyKey, friendRoomTTL).Err()
	_ = r.redis.Expire(ctx, lastSeenKey, friendRoomTTL).Err()
	return nil
}

func (r *FriendRoomRepository) JoinFriendRoom(ctx context.Context, roomCode string, player FriendRoomPlayer) error {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return fmt.Errorf("friend room redis repository is not initialized")
	}
	payload, err := json.Marshal(player)
	if err != nil {
		return fmt.Errorf("encode friend room player: %w", err)
	}
	result, err := r.redis.Eval(ctx, friendRoomJoinScript,
		[]string{
			redisplatform.FriendRoomKey(roomCode),
			redisplatform.FriendRoomStateKey(roomCode),
			redisplatform.FriendRoomPlayersKey(roomCode),
			redisplatform.FriendRoomReadyKey(roomCode),
			redisplatform.FriendRoomLastSeenKey(roomCode),
		},
		strconv.FormatUint(player.UserID, 10), string(payload), int64(friendRoomTTL/time.Second), time.Now().UTC().UnixMilli(),
	).Int64()
	if err != nil {
		return err
	}
	switch result {
	case -2:
		return ErrFriendRoomNotFound
	case -1:
		return ErrFriendRoomFull
	case 0:
		return ErrFriendRoomAlreadyIn
	case -3:
		return ErrFriendRoomExpired
	case -4:
		return ErrFriendRoomCancelled
	case -5:
		return ErrFriendRoomStarted
	default:
		return nil
	}
}

func (r *FriendRoomRepository) GetFriendRoom(ctx context.Context, roomCode string) (FriendRoom, error) {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return FriendRoom{}, fmt.Errorf("friend room redis repository is not initialized")
	}
	payload, err := r.redis.Get(ctx, redisplatform.FriendRoomKey(roomCode)).Bytes()
	if errors.Is(err, goRedis.Nil) {
		return FriendRoom{}, ErrFriendRoomNotFound
	}
	if err != nil {
		return FriendRoom{}, err
	}
	var room FriendRoom
	if err := json.Unmarshal(payload, &room); err != nil {
		return FriendRoom{}, fmt.Errorf("decode friend room: %w", err)
	}

	playerValues, err := r.redis.HGetAll(ctx, redisplatform.FriendRoomPlayersKey(roomCode)).Result()
	if err != nil {
		return FriendRoom{}, err
	}
	readyValues, err := r.redis.HGetAll(ctx, redisplatform.FriendRoomReadyKey(roomCode)).Result()
	if err != nil {
		return FriendRoom{}, err
	}
	lastSeenValues, err := r.redis.HGetAll(ctx, redisplatform.FriendRoomLastSeenKey(roomCode)).Result()
	if err != nil {
		return FriendRoom{}, err
	}
	players := make([]FriendRoomPlayer, 0, len(playerValues))
	now := time.Now().UTC()
	for userIDText, value := range playerValues {
		var player FriendRoomPlayer
		if err := json.Unmarshal([]byte(value), &player); err != nil {
			return FriendRoom{}, fmt.Errorf("decode friend room player: %w", err)
		}
		if ready, exists := readyValues[userIDText]; exists {
			player.Ready = ready == "1" || ready == "true"
		}
		if lastSeen, exists := lastSeenValues[userIDText]; exists {
			if milliseconds, parseErr := strconv.ParseInt(lastSeen, 10, 64); parseErr == nil && milliseconds > 0 {
				player.LastSeenAt = time.UnixMilli(milliseconds).UTC()
			}
		}
		players = append(players, player)
	}
	sort.Slice(players, func(i, j int) bool {
		if players[i].UserID == room.OwnerID && players[j].UserID == room.OwnerID {
			return false
		}
		if players[i].UserID == room.OwnerID {
			return true
		}
		if players[j].UserID == room.OwnerID {
			return false
		}
		return players[i].UserID < players[j].UserID
	})
	room.Players = players
	status, statusErr := r.redis.Get(ctx, redisplatform.FriendRoomStateKey(roomCode)).Result()
	if statusErr != nil && !errors.Is(statusErr, goRedis.Nil) {
		return FriendRoom{}, statusErr
	}
	if errors.Is(statusErr, goRedis.Nil) || status == "" {
		status = room.Status
	}
	if status == "" {
		status = FriendRoomWaiting
	}
	if !room.ExpiresAt.IsZero() && now.After(room.ExpiresAt) && !friendRoomIsTerminal(status) && status != FriendRoomFinished {
		status = FriendRoomExpired
		_ = r.redis.Set(ctx, redisplatform.FriendRoomStateKey(roomCode), status, friendRoomTTL).Err()
	}
	if (status == FriendRoomWaiting || status == FriendRoomReady) && len(players) == 2 && friendRoomAllReady(room) {
		status = FriendRoomReady
	} else if status == FriendRoomWaiting || status == FriendRoomReady {
		status = FriendRoomWaiting
	}
	if status == FriendRoomCountdown && room.StartAt > 0 && now.UnixMilli() >= room.StartAt {
		status = FriendRoomRunning
		room.Status = status
		if encoded, marshalErr := json.Marshal(room); marshalErr == nil {
			_ = r.redis.Set(ctx, redisplatform.FriendRoomKey(roomCode), encoded, time.Until(room.ExpiresAt)).Err()
		}
		_ = r.redis.Set(ctx, redisplatform.FriendRoomStateKey(roomCode), status, time.Until(room.ExpiresAt)).Err()
	}
	room.Status = status
	for index := range room.Players {
		lastSeen := room.Players[index].LastSeenAt
		room.Players[index].Disconnected = (status == FriendRoomCountdown || status == FriendRoomRunning) && !lastSeen.IsZero() && now.Sub(lastSeen) > 15*time.Second
	}
	return room, nil
}

func (r *FriendRoomRepository) SetFriendRoomReady(ctx context.Context, roomCode string, userID uint64, ready bool) (FriendRoom, error) {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return FriendRoom{}, fmt.Errorf("friend room redis repository is not initialized")
	}
	readyValue := "0"
	if ready {
		readyValue = "1"
	}
	result, err := r.redis.Eval(ctx, friendRoomReadyScript, []string{
		redisplatform.FriendRoomKey(roomCode),
		redisplatform.FriendRoomStateKey(roomCode),
		redisplatform.FriendRoomPlayersKey(roomCode),
		redisplatform.FriendRoomReadyKey(roomCode),
		redisplatform.FriendRoomLastSeenKey(roomCode),
	}, strconv.FormatUint(userID, 10), readyValue, time.Now().UTC().UnixMilli(), int64(friendRoomTTL/time.Second)).Int64()
	if err != nil {
		return FriendRoom{}, err
	}
	switch result {
	case -2:
		return FriendRoom{}, ErrFriendRoomNotFound
	case -3:
		return FriendRoom{}, ErrFriendRoomExpired
	case -4:
		return FriendRoom{}, ErrFriendRoomCancelled
	case -5:
		return FriendRoom{}, ErrFriendRoomStarted
	case -6:
		return FriendRoom{}, ErrFriendRoomNotMember
	default:
		return r.GetFriendRoom(ctx, roomCode)
	}
}

func (r *FriendRoomRepository) StartFriendRoom(ctx context.Context, roomCode string, userID uint64, started FriendRoom) (FriendRoom, error) {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return FriendRoom{}, fmt.Errorf("friend room redis repository is not initialized")
	}
	payload, err := json.Marshal(started)
	if err != nil {
		return FriendRoom{}, fmt.Errorf("encode started friend room: %w", err)
	}
	ttl := time.Until(started.ExpiresAt)
	if ttl <= 0 {
		return FriendRoom{}, ErrFriendRoomExpired
	}
	result, err := r.redis.Eval(ctx, friendRoomStartScript, []string{
		redisplatform.FriendRoomKey(roomCode),
		redisplatform.FriendRoomStateKey(roomCode),
		redisplatform.FriendRoomPlayersKey(roomCode),
		redisplatform.FriendRoomReadyKey(roomCode),
	}, string(payload), int64(ttl/time.Second), strconv.FormatUint(userID, 10)).Int64()
	if err != nil {
		return FriendRoom{}, err
	}
	switch result {
	case -2:
		return FriendRoom{}, ErrFriendRoomNotFound
	case -3:
		return FriendRoom{}, ErrFriendRoomExpired
	case -4:
		return FriendRoom{}, ErrFriendRoomCancelled
	case -5:
		return FriendRoom{}, ErrFriendRoomNotMember
	case -6:
		return FriendRoom{}, ErrFriendRoomNotReady
	case -7:
		return FriendRoom{}, ErrFriendRoomNotReady
	case 0:
		// A concurrent start already won. Returning the persisted contract makes
		// repeated start requests idempotent and keeps both players in sync.
		return r.GetFriendRoom(ctx, roomCode)
	default:
		return r.GetFriendRoom(ctx, roomCode)
	}
}

func (r *FriendRoomRepository) FinishFriendRoom(ctx context.Context, roomCode string, matchID string) error {
	room, err := r.GetFriendRoom(ctx, roomCode)
	if err != nil {
		return err
	}
	if room.MatchID != "" && matchID != "" && room.MatchID != matchID {
		return fmt.Errorf("friend match id does not match room")
	}
	if room.Status == FriendRoomFinished {
		return nil
	}
	if room.Status != FriendRoomCountdown && room.Status != FriendRoomRunning {
		return ErrFriendRoomStarted
	}
	room.Status = FriendRoomFinished
	payload, err := json.Marshal(room)
	if err != nil {
		return err
	}
	ttl := time.Until(room.ExpiresAt)
	if ttl <= 0 {
		ttl = friendRoomTTL
	}
	pipe := r.redis.TxPipeline()
	pipe.Set(ctx, redisplatform.FriendRoomKey(roomCode), payload, ttl)
	pipe.Set(ctx, redisplatform.FriendRoomStateKey(roomCode), FriendRoomFinished, ttl)
	_, err = pipe.Exec(ctx)
	return err
}

func (r *FriendRoomRepository) RemoveFriendRoomPlayer(ctx context.Context, roomCode string, userID uint64) error {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return fmt.Errorf("friend room redis repository is not initialized")
	}
	if exists, err := r.redis.Exists(ctx, redisplatform.FriendRoomKey(roomCode)).Result(); err != nil {
		return err
	} else if exists == 0 {
		return ErrFriendRoomNotFound
	}
	if err := r.redis.HDel(ctx, redisplatform.FriendRoomPlayersKey(roomCode), strconv.FormatUint(userID, 10)).Err(); err != nil {
		return err
	}
	if err := r.redis.HDel(ctx, redisplatform.FriendRoomMatchProgressKey(roomCode), strconv.FormatUint(userID, 10)).Err(); err != nil {
		return err
	}
	_ = r.redis.HDel(ctx, redisplatform.FriendRoomMatchProgressStateKey(roomCode), strconv.FormatUint(userID, 10)).Err()
	_ = r.redis.HDel(ctx, redisplatform.FriendRoomLastSeenKey(roomCode), strconv.FormatUint(userID, 10)).Err()
	return r.redis.HDel(ctx, redisplatform.FriendRoomMatchResultsKey(roomCode), strconv.FormatUint(userID, 10)).Err()
}

func (r *FriendRoomRepository) DeleteFriendRoom(ctx context.Context, roomCode string) error {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return fmt.Errorf("friend room redis repository is not initialized")
	}
	return r.redis.Del(ctx,
		redisplatform.FriendRoomKey(roomCode),
		redisplatform.FriendRoomPlayersKey(roomCode),
		redisplatform.FriendRoomStateKey(roomCode),
		redisplatform.FriendRoomReadyKey(roomCode),
		redisplatform.FriendRoomLastSeenKey(roomCode),
		redisplatform.FriendRoomMatchProgressKey(roomCode),
		redisplatform.FriendRoomMatchProgressStateKey(roomCode),
		redisplatform.FriendRoomMatchProgressEventsKey(roomCode),
		redisplatform.FriendRoomMatchResultsKey(roomCode),
	).Err()
}

func (r *FriendRoomRepository) SaveFriendMatchProgress(ctx context.Context, roomCode string, progress FriendMatchProgress) error {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return fmt.Errorf("friend match progress redis repository is not initialized")
	}
	payload, err := json.Marshal(progress)
	if err != nil {
		return fmt.Errorf("encode friend match progress: %w", err)
	}
	key := redisplatform.FriendRoomMatchProgressKey(roomCode)
	if err := r.redis.HSet(ctx, key, strconv.FormatUint(progress.UserID, 10), payload).Err(); err != nil {
		return err
	}
	state := progressStateValue(progress)
	if err := r.redis.HSet(ctx, redisplatform.FriendRoomMatchProgressStateKey(roomCode), strconv.FormatUint(progress.UserID, 10), state).Err(); err != nil {
		return err
	}
	_ = r.redis.HSet(ctx, redisplatform.FriendRoomLastSeenKey(roomCode), strconv.FormatUint(progress.UserID, 10), progress.UpdatedAt.UnixMilli()).Err()
	if err := r.redis.Expire(ctx, key, friendRoomTTL).Err(); err != nil {
		return err
	}
	_ = r.redis.Expire(ctx, redisplatform.FriendRoomMatchProgressStateKey(roomCode), friendRoomTTL).Err()
	_ = r.redis.Expire(ctx, redisplatform.FriendRoomLastSeenKey(roomCode), friendRoomTTL).Err()
	return nil
}

func (r *FriendRoomRepository) SaveFriendMatchProgressEvent(ctx context.Context, roomCode string, progress FriendMatchProgress) (bool, error) {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return false, fmt.Errorf("friend match progress redis repository is not initialized")
	}
	payload, err := json.Marshal(progress)
	if err != nil {
		return false, fmt.Errorf("encode friend match progress: %w", err)
	}
	result, err := r.redis.Eval(ctx, friendMatchProgressScript, []string{
		redisplatform.FriendRoomMatchProgressKey(roomCode),
		redisplatform.FriendRoomMatchProgressStateKey(roomCode),
		redisplatform.FriendRoomMatchProgressEventsKey(roomCode),
	}, strconv.FormatUint(progress.UserID, 10), string(payload), progressStateValue(progress), int64(friendRoomTTL/time.Second), progress.EventID,
		progress.QuestionIndex, progress.Solved, progress.Score, progress.ElapsedMS, boolToInt(progress.Finished)).Int64()
	if err != nil {
		return false, err
	}
	if result == -1 {
		return false, ErrFriendMatchProgressStale
	}
	if result == 0 {
		return false, nil
	}
	_ = r.redis.HSet(ctx, redisplatform.FriendRoomLastSeenKey(roomCode), strconv.FormatUint(progress.UserID, 10), progress.UpdatedAt.UnixMilli()).Err()
	_ = r.redis.Expire(ctx, redisplatform.FriendRoomLastSeenKey(roomCode), friendRoomTTL).Err()
	return true, nil
}

func (r *FriendRoomRepository) GetFriendMatchProgress(ctx context.Context, roomCode string) (map[uint64]FriendMatchProgress, error) {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return nil, fmt.Errorf("friend match progress redis repository is not initialized")
	}
	values, err := r.redis.HGetAll(ctx, redisplatform.FriendRoomMatchProgressKey(roomCode)).Result()
	if err != nil {
		return nil, err
	}
	result := make(map[uint64]FriendMatchProgress, len(values))
	for key, value := range values {
		userID, err := strconv.ParseUint(key, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse friend match progress user id: %w", err)
		}
		var progress FriendMatchProgress
		if err := json.Unmarshal([]byte(value), &progress); err != nil {
			return nil, fmt.Errorf("decode friend match progress: %w", err)
		}
		result[userID] = progress
	}
	return result, nil
}

func (r *FriendRoomRepository) CreateEndlessRun(ctx context.Context, run EndlessRun) error {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return fmt.Errorf("endless run redis repository is not initialized")
	}
	payload, err := json.Marshal(run)
	if err != nil {
		return fmt.Errorf("encode endless run: %w", err)
	}
	ttl := time.Until(run.ExpiresAt)
	if ttl <= 0 {
		ttl = 2 * time.Hour
	}
	ok, err := r.redis.SetNX(ctx, redisplatform.EndlessRunKey(run.RunID), payload, ttl).Result()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("endless run id already exists")
	}
	return nil
}

func (r *FriendRoomRepository) GetEndlessRun(ctx context.Context, runID string) (EndlessRun, error) {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return EndlessRun{}, fmt.Errorf("endless run redis repository is not initialized")
	}
	payload, err := r.redis.Get(ctx, redisplatform.EndlessRunKey(runID)).Bytes()
	if errors.Is(err, goRedis.Nil) {
		return EndlessRun{}, ErrEndlessRunNotFound
	}
	if err != nil {
		return EndlessRun{}, err
	}
	var run EndlessRun
	if err := json.Unmarshal(payload, &run); err != nil {
		return EndlessRun{}, fmt.Errorf("decode endless run: %w", err)
	}
	return run, nil
}

func (r *FriendRoomRepository) UpdateEndlessRun(ctx context.Context, run EndlessRun) error {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return fmt.Errorf("endless run redis repository is not initialized")
	}
	payload, err := json.Marshal(run)
	if err != nil {
		return fmt.Errorf("encode endless run: %w", err)
	}
	ttl := time.Until(run.ExpiresAt)
	if ttl <= 0 {
		ttl = time.Minute
	}
	return r.redis.Set(ctx, redisplatform.EndlessRunKey(run.RunID), payload, ttl).Err()
}

func (r *FriendRoomRepository) CreateCampaignRun(ctx context.Context, run CampaignRun) error {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return fmt.Errorf("campaign run redis repository is not initialized")
	}
	payload, err := json.Marshal(run)
	if err != nil {
		return fmt.Errorf("encode campaign run: %w", err)
	}
	ttl := time.Until(run.ExpiresAt)
	if ttl <= 0 {
		ttl = campaignRunTTL
	}
	ok, err := r.redis.SetNX(ctx, redisplatform.CampaignRunKey(run.RunID), payload, ttl).Result()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("campaign run id already exists")
	}
	return nil
}

func (r *FriendRoomRepository) GetCampaignRun(ctx context.Context, runID string) (CampaignRun, error) {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return CampaignRun{}, fmt.Errorf("campaign run redis repository is not initialized")
	}
	payload, err := r.redis.Get(ctx, redisplatform.CampaignRunKey(runID)).Bytes()
	if errors.Is(err, goRedis.Nil) {
		return CampaignRun{}, ErrCampaignRunNotFound
	}
	if err != nil {
		return CampaignRun{}, err
	}
	var run CampaignRun
	if err := json.Unmarshal(payload, &run); err != nil {
		return CampaignRun{}, fmt.Errorf("decode campaign run: %w", err)
	}
	return run, nil
}

func (r *FriendRoomRepository) UpdateCampaignRun(ctx context.Context, run CampaignRun) error {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return fmt.Errorf("campaign run redis repository is not initialized")
	}
	payload, err := json.Marshal(run)
	if err != nil {
		return fmt.Errorf("encode campaign run: %w", err)
	}
	ttl := time.Until(run.ExpiresAt)
	if ttl <= 0 {
		ttl = time.Minute
	}
	return r.redis.Set(ctx, redisplatform.CampaignRunKey(run.RunID), payload, ttl).Err()
}

func (r *FriendRoomRepository) CreateDailyRun(ctx context.Context, run DailyRun) error {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return fmt.Errorf("daily run redis repository is not initialized")
	}
	payload, err := json.Marshal(run)
	if err != nil {
		return fmt.Errorf("encode daily run: %w", err)
	}
	ttl := time.Until(run.ExpiresAt)
	if ttl <= 0 {
		ttl = dailyRunTTL
	}
	ok, err := r.redis.SetNX(ctx, redisplatform.DailyRunKey(run.RunID), payload, ttl).Result()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("daily run id already exists")
	}
	return nil
}

func (r *FriendRoomRepository) GetDailyRun(ctx context.Context, runID string) (DailyRun, error) {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return DailyRun{}, fmt.Errorf("daily run redis repository is not initialized")
	}
	payload, err := r.redis.Get(ctx, redisplatform.DailyRunKey(runID)).Bytes()
	if errors.Is(err, goRedis.Nil) {
		return DailyRun{}, ErrDailyRunNotFound
	}
	if err != nil {
		return DailyRun{}, err
	}
	var run DailyRun
	if err := json.Unmarshal(payload, &run); err != nil {
		return DailyRun{}, fmt.Errorf("decode daily run: %w", err)
	}
	return run, nil
}

func (r *FriendRoomRepository) UpdateDailyRun(ctx context.Context, run DailyRun) error {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return fmt.Errorf("daily run redis repository is not initialized")
	}
	payload, err := json.Marshal(run)
	if err != nil {
		return fmt.Errorf("encode daily run: %w", err)
	}
	ttl := time.Until(run.ExpiresAt)
	if ttl <= 0 {
		ttl = time.Minute
	}
	return r.redis.Set(ctx, redisplatform.DailyRunKey(run.RunID), payload, ttl).Err()
}

func (r *FriendRoomRepository) SaveFriendMatchSubmission(ctx context.Context, roomCode string, submission FriendMatchSubmissionRecord) error {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return fmt.Errorf("friend match result redis repository is not initialized")
	}
	payload, err := json.Marshal(submission)
	if err != nil {
		return fmt.Errorf("encode friend match submission: %w", err)
	}
	key := redisplatform.FriendRoomMatchResultsKey(roomCode)
	ok, err := r.redis.HSetNX(ctx, key, strconv.FormatUint(submission.UserID, 10), payload).Result()
	if err != nil {
		return err
	}
	if !ok {
		return ErrFriendMatchSubmissionAlreadyExists
	}
	if err := r.redis.Expire(ctx, key, friendRoomTTL).Err(); err != nil {
		return err
	}
	return nil
}

func (r *FriendRoomRepository) GetFriendMatchSubmissions(ctx context.Context, roomCode string) (map[uint64]FriendMatchSubmissionRecord, error) {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return nil, fmt.Errorf("friend match result redis repository is not initialized")
	}
	values, err := r.redis.HGetAll(ctx, redisplatform.FriendRoomMatchResultsKey(roomCode)).Result()
	if err != nil {
		return nil, err
	}
	result := make(map[uint64]FriendMatchSubmissionRecord, len(values))
	for key, value := range values {
		userID, err := strconv.ParseUint(key, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse friend match result user id: %w", err)
		}
		var submission FriendMatchSubmissionRecord
		if err := json.Unmarshal([]byte(value), &submission); err != nil {
			return nil, fmt.Errorf("decode friend match submission: %w", err)
		}
		result[userID] = submission
	}
	return result, nil
}

func progressStateValue(progress FriendMatchProgress) string {
	return fmt.Sprintf("%d|%d|%d|%d|%d", progress.QuestionIndex, progress.Solved, progress.Score, progress.ElapsedMS, boolToInt(progress.Finished))
}

func boolToRedis(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
