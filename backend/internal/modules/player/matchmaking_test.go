package player

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/example/go-service/internal/modules/user"
)

type matchmakingProfileReader struct{}

func (matchmakingProfileReader) GetProfile(_ context.Context, id uint64) (user.ProfileResponse, error) {
	return user.ProfileResponse{ID: id, Nickname: "玩家" + string(rune('A'+id-3))}, nil
}

type matchmakingStoreFake struct {
	tickets map[string]MatchmakingTicket
}

func (f *matchmakingStoreFake) EnqueueMatchmaking(_ context.Context, ticket MatchmakingTicket) (MatchmakingTicket, error) {
	if f.tickets == nil {
		f.tickets = map[string]MatchmakingTicket{}
	}
	for _, existing := range f.tickets {
		if existing.UserID == ticket.UserID && existing.Status == "searching" {
			return MatchmakingTicket{}, ErrMatchmakingAlreadyQueued
		}
	}
	for _, existing := range f.tickets {
		if existing.Status == "searching" && existing.Mode == ticket.Mode {
			ticket.MatchedTicketID = existing.TicketID
			f.tickets[ticket.TicketID] = ticket
			return ticket, nil
		}
	}
	f.tickets[ticket.TicketID] = ticket
	return ticket, nil
}

func (f *matchmakingStoreFake) GetMatchmakingTicket(_ context.Context, _ string, ticketID string) (MatchmakingTicket, error) {
	ticket, ok := f.tickets[ticketID]
	if !ok {
		return MatchmakingTicket{}, ErrMatchmakingNotFound
	}
	return ticket, nil
}

func (f *matchmakingStoreFake) GetMatchmakingTicketByUser(_ context.Context, _ string, userID uint64) (MatchmakingTicket, error) {
	for _, ticket := range f.tickets {
		if ticket.UserID == userID {
			return ticket, nil
		}
	}
	return MatchmakingTicket{}, ErrMatchmakingNotFound
}

func (f *matchmakingStoreFake) SaveMatchmakingPair(_ context.Context, current, opponent MatchmakingTicket) error {
	f.tickets[current.TicketID] = current
	f.tickets[opponent.TicketID] = opponent
	return nil
}

func (f *matchmakingStoreFake) CancelMatchmaking(_ context.Context, _ string, ticketID string, _ uint64) error {
	delete(f.tickets, ticketID)
	return nil
}

func (f *matchmakingStoreFake) CreateEndlessRun(context.Context, EndlessRun) error {
	return errors.New("not used in matchmaking test")
}

func (f *matchmakingStoreFake) GetEndlessRun(context.Context, string) (EndlessRun, error) {
	return EndlessRun{}, errors.New("not used in matchmaking test")
}

type matchmakingRoomStoreFake struct {
	room FriendRoom
}

type matchmakingLockFake struct {
	deny  bool
	calls int
}

func (f *matchmakingLockFake) AcquireDistributedLock(context.Context, string, time.Duration) (string, bool, error) {
	f.calls++
	if f.deny {
		return "", false, nil
	}
	return "ticket-lock", true, nil
}

func (f *matchmakingLockFake) ReleaseDistributedLock(context.Context, string, string) error {
	return nil
}

func (f *matchmakingRoomStoreFake) CreateFriendRoom(_ context.Context, room FriendRoom) error {
	f.room = room
	return nil
}

func (f *matchmakingRoomStoreFake) JoinFriendRoom(_ context.Context, roomCode string, player FriendRoomPlayer) error {
	if f.room.RoomCode != roomCode {
		return ErrFriendRoomNotFound
	}
	f.room.Players = append(f.room.Players, player)
	return nil
}

func (f *matchmakingRoomStoreFake) GetFriendRoom(_ context.Context, roomCode string) (FriendRoom, error) {
	if f.room.RoomCode != roomCode {
		return FriendRoom{}, ErrFriendRoomNotFound
	}
	f.room.Status = FriendRoomWaiting
	if len(f.room.Players) >= 2 {
		f.room.Status = FriendRoomReady
	}
	return f.room, nil
}

func (f *matchmakingRoomStoreFake) RemoveFriendRoomPlayer(context.Context, string, uint64) error {
	return nil
}

func (f *matchmakingRoomStoreFake) DeleteFriendRoom(context.Context, string) error {
	f.room = FriendRoom{}
	return nil
}

func (f *matchmakingRoomStoreFake) SaveFriendMatchProgress(context.Context, string, FriendMatchProgress) error {
	return nil
}

func (f *matchmakingRoomStoreFake) GetFriendMatchProgress(context.Context, string) (map[uint64]FriendMatchProgress, error) {
	return map[uint64]FriendMatchProgress{}, nil
}

func TestMatchmakingMatchesTwoPlayersIntoServerRoom(t *testing.T) {
	matchmaking := &matchmakingStoreFake{}
	rooms := &matchmakingRoomStoreFake{}
	service := NewServiceWithRoomsAndEndless(matchmakingProfileReader{}, &leaderboardStore{}, rooms, matchmaking)
	input := func(ticket string) JoinMatchmakingInput {
		return JoinMatchmakingInput{Mode: matchmakingModeFriend, RulesVersion: matchmakingRulesV1, ClientTicket: ticket, Region: "local"}
	}

	first, err := service.JoinMatchmaking(context.Background(), 3, input("client-aaa"))
	if err != nil || first.Status != "searching" || first.TicketID == "" {
		t.Fatalf("first matchmaking result = %#v, error = %v", first, err)
	}
	second, err := service.JoinMatchmaking(context.Background(), 4, input("client-bbb"))
	if err != nil {
		t.Fatalf("second JoinMatchmaking() error = %v", err)
	}
	if second.Status != "matched" || second.Room == nil || second.Room.Status != FriendRoomReady || len(second.Room.Players) != 2 {
		t.Fatalf("second matchmaking result = %#v, want matched ready room", second)
	}
	status, err := service.GetMatchmakingStatus(context.Background(), 3, first.TicketID)
	if err != nil || status.Status != "matched" || status.Room == nil || len(status.Room.Players) != 2 {
		t.Fatalf("first status = %#v, error = %v", status, err)
	}
}

func TestPublicFriendRoomCannotEnableRankedMode(t *testing.T) {
	rooms := &matchmakingRoomStoreFake{}
	service := NewServiceWithRoomsAndEndless(matchmakingProfileReader{}, &leaderboardStore{}, rooms, &matchmakingStoreFake{})

	room, err := service.CreateFriendRoomWithRules(context.Background(), 3, FriendRoomCreateInput{
		Ranked:      true,
		SeasonID:    "not-a-real-season",
		MatchSource: "matchmaking",
	})
	if err != nil {
		t.Fatalf("CreateFriendRoomWithRules() error = %v", err)
	}
	if room.Ranked || room.RankedEligible || room.SeasonID != "" || room.MatchSource != "manual" {
		t.Fatalf("public room = %#v, want server-forced casual manual room", room)
	}
}

func TestRankedMatchmakingUsesServerRankSnapshot(t *testing.T) {
	matchmaking := &matchmakingStoreFake{}
	rooms := &matchmakingRoomStoreFake{}
	rankStore := &rankStoreFake{profile: RankProfile{
		UserID: 3, Rating: 1500, Tier: RankTierPlatinum, Division: 2, Stars: 4,
	}}
	service := NewServiceWithRoomsAndEndless(matchmakingProfileReader{}, &leaderboardStore{}, rooms, matchmaking)
	service.SetRankStore(rankStore)

	result, err := service.JoinMatchmaking(context.Background(), 3, JoinMatchmakingInput{
		Mode: matchmakingModeFriend, RulesVersion: matchmakingRulesV1, ClientTicket: "client-ranked-1",
		Ranked: true, SeasonID: "1999-S1", RankTier: RankTierKing, RankDivision: 1, RankStars: 4,
	})
	if err != nil {
		t.Fatalf("JoinMatchmaking() error = %v", err)
	}
	if !result.Ranked || result.SeasonID == "1999-S1" || result.RankSnapshot == nil ||
		result.RankSnapshot.Rating != 1500 || result.RankSnapshot.Tier != RankTierPlatinum ||
		result.RankSnapshot.Division != 2 || result.RankSnapshot.Stars != 4 {
		t.Fatalf("ranked matchmaking result = %#v, want server rank snapshot", result)
	}
}

func TestMatchmakingQueueDiscriminatorSeparatesRankedDimensions(t *testing.T) {
	casual := MatchmakingTicket{Mode: matchmakingModeFriend, RulesVersion: matchmakingRulesV1, Region: "local"}
	rankedBronze := MatchmakingTicket{Mode: matchmakingModeFriend, RulesVersion: matchmakingRulesV1, Region: "local", Ranked: true, SeasonID: "2026-S3", RankTier: RankTierBronze, RankDivision: 3}
	rankedSilver := rankedBronze
	rankedSilver.RankTier = RankTierSilver
	if matchmakingQueueDiscriminator(casual) == matchmakingQueueDiscriminator(rankedBronze) {
		t.Fatal("casual and ranked queues must be isolated")
	}
	if matchmakingQueueDiscriminator(rankedBronze) == matchmakingQueueDiscriminator(rankedSilver) {
		t.Fatal("different ranked divisions must be isolated")
	}
}

func TestMatchmakingTicketIsOwnedAndCanBeCancelled(t *testing.T) {
	matchmaking := &matchmakingStoreFake{}
	rooms := &matchmakingRoomStoreFake{}
	service := NewServiceWithRoomsAndEndless(matchmakingProfileReader{}, &leaderboardStore{}, rooms, matchmaking)
	result, err := service.JoinMatchmaking(context.Background(), 3, JoinMatchmakingInput{
		Mode: matchmakingModeFriend, RulesVersion: matchmakingRulesV1, ClientTicket: "client-ccc",
	})
	if err != nil {
		t.Fatalf("JoinMatchmaking() error = %v", err)
	}
	if _, err := service.GetMatchmakingStatus(context.Background(), 4, result.TicketID); err == nil {
		t.Fatal("GetMatchmakingStatus() error = nil for another user")
	}
	cancelled, err := service.CancelMatchmaking(context.Background(), 3, result.TicketID)
	if err != nil || cancelled.Status != "cancelled" {
		t.Fatalf("CancelMatchmaking() = %#v, error = %v", cancelled, err)
	}
	if _, err := service.GetMatchmakingStatus(context.Background(), 3, result.TicketID); err == nil {
		t.Fatal("status after cancel error = nil, want not found")
	}
}

func TestBotMatchCreationReturnsCurrentTicketWhenDistributedLockIsBusy(t *testing.T) {
	matchmaking := &matchmakingStoreFake{}
	rooms := &matchmakingRoomStoreFake{}
	ticket := MatchmakingTicket{
		TicketID: "mm-lock-test", UserID: 3, Mode: matchmakingModeFriend,
		RulesVersion: matchmakingRulesV1, Status: "searching",
		CreatedAt: time.Now().UTC().Add(-16 * time.Second), ExpiresAt: time.Now().UTC().Add(time.Minute),
	}
	matchmaking.tickets = map[string]MatchmakingTicket{ticket.TicketID: ticket}
	lock := &matchmakingLockFake{deny: true}
	service := NewServiceWithRoomsAndEndless(matchmakingProfileReader{}, &leaderboardStore{}, rooms, matchmaking)
	service.locks = lock

	result, err := service.createBotMatch(context.Background(), ticket)
	if err != nil {
		t.Fatalf("createBotMatch() error = %v", err)
	}
	if result.Status != "searching" || rooms.room.RoomCode != "" {
		t.Fatalf("result = %#v, room = %#v, want current ticket without duplicate room", result, rooms.room)
	}
	if lock.calls != 1 {
		t.Fatalf("distributed lock calls = %d, want 1", lock.calls)
	}
}

func TestDefaultMatchmakingWaitIsFifteenSeconds(t *testing.T) {
	service := NewServiceWithRoomsAndEndless(matchmakingProfileReader{}, &leaderboardStore{}, &matchmakingRoomStoreFake{}, &matchmakingStoreFake{})
	if service.matchmakingWait != 15*time.Second {
		t.Fatalf("default matchmaking wait = %s, want 15s", service.matchmakingWait)
	}
}
