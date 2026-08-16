package player

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	goRedis "github.com/redis/go-redis/v9"

	redisplatform "github.com/example/go-service/internal/platform/redis"
)

var ErrMatchmakingNotFound = errors.New("matchmaking ticket not found")

const matchmakingEnqueueScript = `
if redis.call('EXISTS', KEYS[3]) == 1 then
    return '-1'
end
redis.call('SET', KEYS[2], ARGV[1], 'EX', ARGV[2])
redis.call('SET', KEYS[3], ARGV[4], 'EX', ARGV[3])
redis.call('ZADD', KEYS[1], ARGV[5], ARGV[6])
redis.call('EXPIRE', KEYS[1], ARGV[3])
local candidates = redis.call('ZRANGE', KEYS[1], 0, 9)
for _, candidate in ipairs(candidates) do
    if candidate ~= ARGV[6] then
        local candidateKey = ARGV[7] .. candidate
        if redis.call('EXISTS', candidateKey) == 1 then
            redis.call('ZREM', KEYS[1], candidate, ARGV[6])
            return candidate
        end
        redis.call('ZREM', KEYS[1], candidate)
    end
end
return ''
`

func (r *FriendRoomRepository) EnqueueMatchmaking(ctx context.Context, ticket MatchmakingTicket) (MatchmakingTicket, error) {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return MatchmakingTicket{}, fmt.Errorf("matchmaking redis repository is not initialized")
	}
	payload, err := json.Marshal(ticket)
	if err != nil {
		return MatchmakingTicket{}, fmt.Errorf("encode matchmaking ticket: %w", err)
	}
	mode := ticket.Mode
	if mode == "" {
		mode = matchmakingModeFriend
	}
	ttl := int64(matchmakingTTL / time.Second)
	retention := int64(matchmakingRetention / time.Second)
	queueKey := ticket.QueueKey
	if queueKey == "" {
		queueKey = matchmakingQueueDiscriminator(ticket)
	}
	ticket.QueueKey = queueKey
	payload, err = json.Marshal(ticket)
	if err != nil {
		return MatchmakingTicket{}, fmt.Errorf("encode matchmaking ticket: %w", err)
	}
	result, err := r.redis.Eval(ctx, matchmakingEnqueueScript,
		[]string{
			redisplatform.MatchmakingQueueKey(queueKey),
			redisplatform.MatchmakingTicketKey(mode, ticket.TicketID),
			redisplatform.MatchmakingUserKey(mode, ticket.UserID),
		},
		string(payload), retention, ttl, ticket.TicketID, time.Now().UnixMilli(), ticket.TicketID,
		redisplatform.MatchmakingTicketKey(mode, ""),
	).Result()
	if err != nil {
		return MatchmakingTicket{}, err
	}
	matchedID, ok := result.(string)
	if !ok {
		return MatchmakingTicket{}, fmt.Errorf("unexpected matchmaking script result %T", result)
	}
	if matchedID == "-1" {
		return MatchmakingTicket{}, ErrMatchmakingAlreadyQueued
	}
	ticket.MatchedTicketID = matchedID
	return ticket, nil
}

func (r *FriendRoomRepository) GetMatchmakingTicket(ctx context.Context, mode, ticketID string) (MatchmakingTicket, error) {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return MatchmakingTicket{}, fmt.Errorf("matchmaking redis repository is not initialized")
	}
	payload, err := r.redis.Get(ctx, redisplatform.MatchmakingTicketKey(mode, ticketID)).Bytes()
	if errors.Is(err, goRedis.Nil) {
		return MatchmakingTicket{}, ErrMatchmakingNotFound
	}
	if err != nil {
		return MatchmakingTicket{}, err
	}
	var ticket MatchmakingTicket
	if err := json.Unmarshal(payload, &ticket); err != nil {
		return MatchmakingTicket{}, fmt.Errorf("decode matchmaking ticket: %w", err)
	}
	return ticket, nil
}

func (r *FriendRoomRepository) GetMatchmakingTicketByUser(ctx context.Context, mode string, userID uint64) (MatchmakingTicket, error) {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return MatchmakingTicket{}, fmt.Errorf("matchmaking redis repository is not initialized")
	}
	ticketID, err := r.redis.Get(ctx, redisplatform.MatchmakingUserKey(mode, userID)).Result()
	if errors.Is(err, goRedis.Nil) {
		return MatchmakingTicket{}, ErrMatchmakingNotFound
	}
	if err != nil {
		return MatchmakingTicket{}, err
	}
	return r.GetMatchmakingTicket(ctx, mode, ticketID)
}

func (r *FriendRoomRepository) SaveMatchmakingPair(ctx context.Context, current, opponent MatchmakingTicket) error {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return fmt.Errorf("matchmaking redis repository is not initialized")
	}
	currentPayload, err := json.Marshal(current)
	if err != nil {
		return fmt.Errorf("encode current matchmaking ticket: %w", err)
	}
	opponentPayload, err := json.Marshal(opponent)
	if err != nil {
		return fmt.Errorf("encode opponent matchmaking ticket: %w", err)
	}
	ttl := matchmakingRetention
	pipe := r.redis.TxPipeline()
	pipe.Set(ctx, redisplatform.MatchmakingTicketKey(current.Mode, current.TicketID), currentPayload, ttl)
	pipe.Set(ctx, redisplatform.MatchmakingTicketKey(opponent.Mode, opponent.TicketID), opponentPayload, ttl)
	pipe.Set(ctx, redisplatform.MatchmakingUserKey(current.Mode, current.UserID), current.TicketID, matchmakingRetention)
	pipe.Set(ctx, redisplatform.MatchmakingUserKey(opponent.Mode, opponent.UserID), opponent.TicketID, matchmakingRetention)
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}
	return nil
}

func (r *FriendRoomRepository) MarkMatchmakingExpired(ctx context.Context, ticket MatchmakingTicket) error {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return fmt.Errorf("matchmaking redis repository is not initialized")
	}
	ticket.Status = "expired"
	ticket.MatchedTicketID = ""
	queueKey := ticket.QueueKey
	if queueKey == "" {
		queueKey = matchmakingQueueDiscriminator(ticket)
	}
	payload, err := json.Marshal(ticket)
	if err != nil {
		return err
	}
	pipe := r.redis.TxPipeline()
	pipe.ZRem(ctx, redisplatform.MatchmakingQueueKey(queueKey), ticket.TicketID)
	pipe.Del(ctx, redisplatform.MatchmakingUserKey(ticket.Mode, ticket.UserID))
	pipe.Set(ctx, redisplatform.MatchmakingTicketKey(ticket.Mode, ticket.TicketID), payload, matchmakingRetention)
	_, err = pipe.Exec(ctx)
	return err
}

func (r *FriendRoomRepository) CancelMatchmaking(ctx context.Context, mode, ticketID string, userID uint64) error {
	if r == nil || r.redis == nil || r.redis.Client == nil {
		return fmt.Errorf("matchmaking redis repository is not initialized")
	}
	ticket, getErr := r.GetMatchmakingTicket(ctx, mode, ticketID)
	queueKey := mode
	if getErr == nil && ticket.QueueKey != "" {
		queueKey = ticket.QueueKey
	}
	pipe := r.redis.TxPipeline()
	pipe.ZRem(ctx, redisplatform.MatchmakingQueueKey(queueKey), ticketID)
	pipe.Del(ctx, redisplatform.MatchmakingTicketKey(mode, ticketID))
	pipe.Del(ctx, redisplatform.MatchmakingUserKey(mode, userID))
	_, err := pipe.Exec(ctx)
	return err
}
