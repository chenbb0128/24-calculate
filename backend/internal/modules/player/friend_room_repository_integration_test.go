package player

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	goRedis "github.com/redis/go-redis/v9"

	redisplatform "github.com/example/go-service/internal/platform/redis"
)

func TestFriendRoomRepositoryRedisLifecycle(t *testing.T) {
	if os.Getenv("GO_SERVICE_RUN_REDIS_TESTS") != "1" {
		t.Skip("set GO_SERVICE_RUN_REDIS_TESTS=1 to run Redis integration tests")
	}
	ctx := context.Background()
	client := goRedis.NewClient(&goRedis.Options{Addr: "127.0.0.1:6379"})
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("local Redis is unavailable: %v", err)
	}
	store := NewFriendRoomRepository(&redisplatform.Client{Client: client})
	roomCode := time.Now().UTC().Format("150405")
	lockScope := "integration-lock:" + roomCode
	firstToken, acquired, err := store.AcquireDistributedLock(ctx, lockScope, time.Minute)
	if err != nil || !acquired || firstToken == "" {
		t.Fatalf("first distributed lock acquired=%v token=%q error=%v", acquired, firstToken, err)
	}
	defer store.ReleaseDistributedLock(ctx, lockScope, firstToken)
	if _, acquired, err := store.AcquireDistributedLock(ctx, lockScope, time.Minute); err != nil || acquired {
		t.Fatalf("second distributed lock acquired=%v error=%v, want unavailable", acquired, err)
	}
	if err := store.ReleaseDistributedLock(ctx, lockScope, "wrong-token"); err != nil {
		t.Fatalf("wrong distributed lock release error = %v", err)
	}
	room := FriendRoom{
		Version: 1, RoomID: "friend-" + roomCode, RoomCode: roomCode, RoomSeed: 12345,
		OwnerID: 1, Status: FriendRoomWaiting,
		Rules:     FriendRoomRules{QuestionCount: 8, TimeLimitSeconds: 120, Target: 24, UseSameSeed: true, IntegerIntermediate: true},
		Players:   []FriendRoomPlayer{{UserID: 1, Ready: false, LastSeenAt: time.Now().UTC()}},
		CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(30 * time.Minute),
	}
	room.QuestionHash, room.PuzzleIDs, room.Puzzles = friendRoomContract(room)
	defer store.DeleteFriendRoom(ctx, roomCode)

	if err := store.CreateFriendRoom(ctx, room); err != nil {
		t.Fatalf("CreateFriendRoom() error = %v", err)
	}
	if got, err := store.GetFriendRoom(ctx, roomCode); err != nil || got.Status != FriendRoomWaiting {
		t.Fatalf("initial room = %#v, error = %v", got, err)
	}
	if err := store.JoinFriendRoom(ctx, roomCode, FriendRoomPlayer{UserID: 2, LastSeenAt: time.Now().UTC()}); err != nil {
		t.Fatalf("JoinFriendRoom() error = %v", err)
	}
	if err := store.JoinFriendRoom(ctx, roomCode, FriendRoomPlayer{UserID: 2}); !errors.Is(err, ErrFriendRoomAlreadyIn) {
		t.Fatalf("duplicate JoinFriendRoom() error = %v, want ErrFriendRoomAlreadyIn", err)
	}
	if _, err := store.SetFriendRoomReady(ctx, roomCode, 1, true); err != nil {
		t.Fatalf("owner ready error = %v", err)
	}
	ready, err := store.SetFriendRoomReady(ctx, roomCode, 2, true)
	if err != nil || ready.Status != FriendRoomReady {
		t.Fatalf("ready room = %#v, error = %v", ready, err)
	}
	room.MatchID = room.RoomID
	room.Status = FriendRoomCountdown
	room.StartAt = time.Now().UTC().Add(1200 * time.Millisecond).UnixMilli()
	started, err := store.StartFriendRoom(ctx, roomCode, 1, room)
	if err != nil || started.MatchID != room.MatchID {
		t.Fatalf("start room = %#v, error = %v", started, err)
	}
	progress := FriendMatchProgress{UserID: 1, MatchID: room.MatchID, QuestionHash: room.QuestionHash, EventID: "redis-event-1", UpdatedAt: time.Now().UTC()}
	accepted, err := store.SaveFriendMatchProgressEvent(ctx, roomCode, progress)
	if err != nil || !accepted {
		t.Fatalf("first progress accepted=%v error=%v", accepted, err)
	}
	accepted, err = store.SaveFriendMatchProgressEvent(ctx, roomCode, progress)
	if err != nil || accepted {
		t.Fatalf("duplicate progress accepted=%v error=%v, want false", accepted, err)
	}
	time.Sleep(1300 * time.Millisecond)
	running, err := store.GetFriendRoom(ctx, roomCode)
	if err != nil || running.Status != FriendRoomRunning {
		t.Fatalf("running room = %#v, error = %v", running, err)
	}
	if err := store.FinishFriendRoom(ctx, roomCode, room.MatchID); err != nil {
		t.Fatalf("FinishFriendRoom() error = %v", err)
	}
	finished, err := store.GetFriendRoom(ctx, roomCode)
	if err != nil || finished.Status != FriendRoomFinished {
		t.Fatalf("finished room = %#v, error = %v", finished, err)
	}
}

func TestMatchmakingRepositoryRedisLifecycle(t *testing.T) {
	if os.Getenv("GO_SERVICE_RUN_REDIS_TESTS") != "1" {
		t.Skip("set GO_SERVICE_RUN_REDIS_TESTS=1 to run Redis integration tests")
	}
	ctx := context.Background()
	client := goRedis.NewClient(&goRedis.Options{Addr: "127.0.0.1:6379"})
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("local Redis is unavailable: %v", err)
	}
	store := NewFriendRoomRepository(&redisplatform.Client{Client: client})
	suffix := time.Now().UTC().UnixNano()
	first := MatchmakingTicket{TicketID: "redis-mm-a-" + time.Now().UTC().Format("150405.000"), UserID: uint64(100000 + suffix%10000), Mode: matchmakingModeFriend, RulesVersion: matchmakingRulesV1, Region: "local", Status: "searching", CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(matchmakingTTL)}
	second := first
	second.TicketID = "redis-mm-b-" + time.Now().UTC().Format("150405.000")
	second.UserID++
	first.QueueKey = matchmakingQueueDiscriminator(first)
	second.QueueKey = matchmakingQueueDiscriminator(second)
	defer func() {
		_ = store.CancelMatchmaking(ctx, first.Mode, first.TicketID, first.UserID)
		_ = store.CancelMatchmaking(ctx, second.Mode, second.TicketID, second.UserID)
	}()
	queued, err := store.EnqueueMatchmaking(ctx, first)
	if err != nil || queued.MatchedTicketID != "" {
		t.Fatalf("first enqueue = %#v, error = %v", queued, err)
	}
	queued, err = store.EnqueueMatchmaking(ctx, second)
	if err != nil || queued.MatchedTicketID != first.TicketID {
		t.Fatalf("second enqueue = %#v, error = %v, want first ticket match", queued, err)
	}
	if err := store.SaveMatchmakingPair(ctx, queued, first); err != nil {
		t.Fatalf("SaveMatchmakingPair() error = %v", err)
	}
	matched, err := store.GetMatchmakingTicket(ctx, matchmakingModeFriend, second.TicketID)
	if err != nil || matched.TicketID != second.TicketID {
		t.Fatalf("matched ticket = %#v, error = %v", matched, err)
	}
	if err := store.MarkMatchmakingExpired(ctx, matched); err != nil {
		t.Fatalf("MarkMatchmakingExpired() error = %v", err)
	}
	expired, err := store.GetMatchmakingTicket(ctx, matchmakingModeFriend, second.TicketID)
	if err != nil || expired.Status != "expired" {
		t.Fatalf("expired ticket = %#v, error = %v", expired, err)
	}
}
