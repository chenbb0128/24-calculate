package player

import (
	"context"
	"testing"

	"github.com/example/go-service/internal/apperror"
	"github.com/example/go-service/internal/modules/user"
)

type friendRoomStoreFake struct {
	created  FriendRoom
	room     FriendRoom
	progress map[uint64]FriendMatchProgress
}

func (f *friendRoomStoreFake) CreateFriendRoom(_ context.Context, room FriendRoom) error {
	f.created = room
	return nil
}

func (f *friendRoomStoreFake) JoinFriendRoom(_ context.Context, roomCode string, player FriendRoomPlayer) error {
	if roomCode != f.room.RoomCode {
		return ErrFriendRoomNotFound
	}
	f.room.Players = append(f.room.Players, player)
	return nil
}

func (f *friendRoomStoreFake) GetFriendRoom(_ context.Context, roomCode string) (FriendRoom, error) {
	if roomCode != f.room.RoomCode {
		return FriendRoom{}, ErrFriendRoomNotFound
	}
	return f.room, nil
}

func (f *friendRoomStoreFake) RemoveFriendRoomPlayer(_ context.Context, roomCode string, userID uint64) error {
	if roomCode != f.room.RoomCode {
		return ErrFriendRoomNotFound
	}
	players := make([]FriendRoomPlayer, 0, len(f.room.Players))
	for _, player := range f.room.Players {
		if player.UserID != userID {
			players = append(players, player)
		}
	}
	f.room.Players = players
	return nil
}

func (f *friendRoomStoreFake) DeleteFriendRoom(_ context.Context, roomCode string) error {
	if roomCode != f.room.RoomCode {
		return ErrFriendRoomNotFound
	}
	f.room = FriendRoom{}
	f.progress = nil
	return nil
}

func (f *friendRoomStoreFake) SaveFriendMatchProgress(_ context.Context, _ string, progress FriendMatchProgress) error {
	if f.progress == nil {
		f.progress = make(map[uint64]FriendMatchProgress)
	}
	f.progress[progress.UserID] = progress
	return nil
}

func (f *friendRoomStoreFake) GetFriendMatchProgress(_ context.Context, _ string) (map[uint64]FriendMatchProgress, error) {
	result := make(map[uint64]FriendMatchProgress, len(f.progress))
	for userID, progress := range f.progress {
		result[userID] = progress
	}
	return result, nil
}

func TestCreateFriendRoomBuildsServerRoom(t *testing.T) {
	store := &friendRoomStoreFake{}
	service := NewServiceWithRooms(leaderboardProfileReader{profile: testFriendProfile(3)}, &leaderboardStore{}, store)

	room, err := service.CreateFriendRoom(context.Background(), 3)
	if err != nil {
		t.Fatalf("CreateFriendRoom() error = %v", err)
	}
	if len(room.RoomCode) != 6 || room.RoomID != "friend-"+room.RoomCode {
		t.Fatalf("room identity = %#v, want six-digit room code", room)
	}
	if room.Status != FriendRoomWaiting || len(room.Players) != 1 || room.Players[0].UserID != 3 {
		t.Fatalf("room = %#v, want waiting room with owner", room)
	}
	if store.created.RoomCode != room.RoomCode {
		t.Fatalf("stored room code = %q, want %q", store.created.RoomCode, room.RoomCode)
	}
}

func TestJoinFriendRoomRejectsInvalidCode(t *testing.T) {
	service := NewServiceWithRooms(leaderboardProfileReader{profile: testFriendProfile(3)}, &leaderboardStore{}, &friendRoomStoreFake{})

	_, err := service.JoinFriendRoom(context.Background(), 3, "abc")
	if err == nil {
		t.Fatal("JoinFriendRoom() error = nil, want bad request")
	}
	appErr, ok := err.(*apperror.AppError)
	if !ok || appErr.HTTPStatus != 400 {
		t.Fatalf("error = %#v, want HTTP 400 app error", err)
	}
}

func TestFriendMatchProgressIsScopedToRoomPlayers(t *testing.T) {
	room := FriendRoom{
		RoomID:   "friend-123456",
		RoomCode: "123456",
		MatchID:  "friend-123456",
		RoomSeed: 42,
		Status:   FriendRoomRunning,
		Rules:    FriendRoomRules{QuestionCount: 8, TimeLimitSeconds: 120},
		Players: []FriendRoomPlayer{
			{UserID: 3, Nickname: "玩家甲", Ready: true},
			{UserID: 4, Nickname: "玩家乙", Ready: true},
		},
	}
	room.QuestionHash, room.PuzzleIDs, room.Puzzles = friendRoomContract(room)
	store := &friendRoomStoreFake{room: room}
	service := NewServiceWithRooms(leaderboardProfileReader{profile: testFriendProfile(3)}, &leaderboardStore{}, store)

	result, err := service.UpdateFriendMatchProgress(context.Background(), 3, "123456", FriendMatchProgressInput{
		QuestionIndex: 2,
		Solved:        2,
		Score:         180,
		ElapsedMS:     12000,
		MatchID:       "friend-123456",
		QuestionHash:  room.QuestionHash,
	})
	if err != nil {
		t.Fatalf("UpdateFriendMatchProgress() error = %v", err)
	}
	if len(result.Players) != 2 || !result.Players[0].IsMe || result.Players[0].Solved != 2 {
		t.Fatalf("progress response = %#v, want current player progress and both room players", result)
	}
	if result.Players[1].IsMe || result.Players[1].Solved != 0 {
		t.Fatalf("opponent progress = %#v, want empty real opponent state", result.Players[1])
	}

	read, err := service.GetFriendMatchProgress(context.Background(), 3, "123456")
	if err != nil || len(read.Players) != 2 || read.Players[0].Score != 180 {
		t.Fatalf("GetFriendMatchProgress() = %#v, error = %v", read, err)
	}
}

func TestFriendMatchProgressSupportsTwoPlayersWithoutTwoWechatAccounts(t *testing.T) {
	room := FriendRoom{
		RoomID:   "friend-654321",
		RoomCode: "654321",
		MatchID:  "friend-654321",
		RoomSeed: 42,
		Status:   FriendRoomRunning,
		Rules:    FriendRoomRules{QuestionCount: 8, TimeLimitSeconds: 120},
		Players: []FriendRoomPlayer{
			{UserID: 3, Nickname: "测试玩家甲", Ready: true},
			{UserID: 4, Nickname: "测试玩家乙", Ready: true},
		},
	}
	room.QuestionHash, room.PuzzleIDs, room.Puzzles = friendRoomContract(room)
	store := &friendRoomStoreFake{room: room}
	service := NewServiceWithRooms(leaderboardProfileReader{profile: testFriendProfile(3)}, &leaderboardStore{}, store)

	if _, err := service.UpdateFriendMatchProgress(context.Background(), 3, "654321", FriendMatchProgressInput{
		QuestionIndex: 3,
		Solved:        3,
		Score:         270,
		ElapsedMS:     18000,
		MatchID:       "friend-654321",
		QuestionHash:  room.QuestionHash,
	}); err != nil {
		t.Fatalf("player 3 progress error = %v", err)
	}
	if _, err := service.UpdateFriendMatchProgress(context.Background(), 4, "654321", FriendMatchProgressInput{
		QuestionIndex: 7,
		Solved:        6,
		Score:         510,
		ElapsedMS:     46000,
		Finished:      true,
		MatchID:       "friend-654321",
		QuestionHash:  room.QuestionHash,
	}); err != nil {
		t.Fatalf("player 4 progress error = %v", err)
	}

	result, err := service.GetFriendMatchProgress(context.Background(), 3, "654321")
	if err != nil {
		t.Fatalf("GetFriendMatchProgress() error = %v", err)
	}
	if len(result.Players) != 2 || result.Players[0].Solved != 3 || result.Players[1].Solved != 6 || !result.Players[1].Finished {
		t.Fatalf("two-player progress = %#v, want both independent player states", result.Players)
	}
}

func TestFriendMatchProgressRejectsBackwardUpdates(t *testing.T) {
	room := FriendRoom{
		RoomID:   "friend-246810",
		RoomCode: "246810",
		MatchID:  "friend-246810",
		RoomSeed: 42,
		Status:   FriendRoomRunning,
		Rules:    FriendRoomRules{QuestionCount: 8, TimeLimitSeconds: 120},
		Players: []FriendRoomPlayer{
			{UserID: 3, Nickname: "player one", Ready: true},
			{UserID: 4, Nickname: "player two", Ready: true},
		},
	}
	room.QuestionHash, room.PuzzleIDs, room.Puzzles = friendRoomContract(room)
	store := &friendRoomStoreFake{room: room}
	service := NewServiceWithRooms(leaderboardProfileReader{profile: testFriendProfile(3)}, &leaderboardStore{}, store)

	if _, err := service.UpdateFriendMatchProgress(context.Background(), 3, "246810", FriendMatchProgressInput{
		QuestionIndex: 3,
		Solved:        3,
		Score:         270,
		ElapsedMS:     18000,
		MatchID:       "friend-246810",
		QuestionHash:  room.QuestionHash,
	}); err != nil {
		t.Fatalf("initial progress error = %v", err)
	}

	if _, err := service.UpdateFriendMatchProgress(context.Background(), 3, "246810", FriendMatchProgressInput{
		QuestionIndex: 2,
		Solved:        2,
		Score:         180,
		ElapsedMS:     12000,
		MatchID:       "friend-246810",
		QuestionHash:  room.QuestionHash,
	}); err == nil {
		t.Fatal("backward progress error = nil, want bad request")
	}

	result, err := service.GetFriendMatchProgress(context.Background(), 3, "246810")
	if err != nil {
		t.Fatalf("GetFriendMatchProgress() error = %v", err)
	}
	if result.Players[0].QuestionIndex != 3 || result.Players[0].Solved != 3 || result.Players[0].Score != 270 || result.Players[0].ElapsedMS != 18000 {
		t.Fatalf("stored progress = %#v, want original progress unchanged", result.Players[0])
	}
}

func TestLeaveFriendRoomRemovesGuestAndOwnerDeletesRoom(t *testing.T) {
	store := &friendRoomStoreFake{room: FriendRoom{
		RoomID:   "friend-246810",
		RoomCode: "246810",
		OwnerID:  3,
		Status:   FriendRoomReady,
		Rules:    FriendRoomRules{QuestionCount: 8, TimeLimitSeconds: 120},
		Players: []FriendRoomPlayer{
			{UserID: 3, Ready: true},
			{UserID: 4, Ready: true},
		},
	}}
	service := NewServiceWithRooms(leaderboardProfileReader{profile: testFriendProfile(3)}, &leaderboardStore{}, store)

	if err := service.LeaveFriendRoom(context.Background(), 4, "246810"); err != nil {
		t.Fatalf("guest LeaveFriendRoom() error = %v", err)
	}
	if len(store.room.Players) != 1 || store.room.Players[0].UserID != 3 {
		t.Fatalf("players after guest leave = %#v, want owner only", store.room.Players)
	}
	if err := service.LeaveFriendRoom(context.Background(), 3, "246810"); err != nil {
		t.Fatalf("owner LeaveFriendRoom() error = %v", err)
	}
	if store.room.RoomCode != "" {
		t.Fatalf("room after owner leave = %#v, want deleted", store.room)
	}
}

func testFriendProfile(id uint64) user.ProfileResponse {
	return user.ProfileResponse{ID: id, Nickname: "测试玩家"}
}
