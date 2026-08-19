package player

import (
	"context"
	"errors"
	"testing"
	"time"
)

type friendLifecycleStoreFake struct {
	room        FriendRoom
	progress    map[uint64]FriendMatchProgress
	events      map[string]struct{}
	submissions map[uint64]FriendMatchSubmissionRecord
}

func (f *friendLifecycleStoreFake) CreateFriendRoom(_ context.Context, room FriendRoom) error {
	f.room = room
	return nil
}

func (f *friendLifecycleStoreFake) JoinFriendRoom(_ context.Context, roomCode string, player FriendRoomPlayer) error {
	if f.room.RoomCode != roomCode {
		return ErrFriendRoomNotFound
	}
	if len(f.room.Players) >= 2 {
		return ErrFriendRoomFull
	}
	f.room.Players = append(f.room.Players, player)
	return nil
}

func (f *friendLifecycleStoreFake) GetFriendRoom(_ context.Context, roomCode string) (FriendRoom, error) {
	if f.room.RoomCode != roomCode {
		return FriendRoom{}, ErrFriendRoomNotFound
	}
	return f.room, nil
}

func (f *friendLifecycleStoreFake) RemoveFriendRoomPlayer(_ context.Context, roomCode string, userID uint64) error {
	if f.room.RoomCode != roomCode {
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

func (f *friendLifecycleStoreFake) DeleteFriendRoom(_ context.Context, roomCode string) error {
	if f.room.RoomCode != roomCode {
		return ErrFriendRoomNotFound
	}
	f.room = FriendRoom{}
	return nil
}

func (f *friendLifecycleStoreFake) SaveFriendMatchProgress(_ context.Context, _ string, progress FriendMatchProgress) error {
	if f.progress == nil {
		f.progress = map[uint64]FriendMatchProgress{}
	}
	f.progress[progress.UserID] = progress
	return nil
}

func (f *friendLifecycleStoreFake) SaveFriendMatchProgressEvent(_ context.Context, _ string, progress FriendMatchProgress) (bool, error) {
	if f.events == nil {
		f.events = map[string]struct{}{}
	}
	if progress.EventID != "" {
		if _, exists := f.events[progress.EventID]; exists {
			return false, nil
		}
		f.events[progress.EventID] = struct{}{}
	}
	previous, exists := f.progress[progress.UserID]
	if exists && (progress.QuestionIndex < previous.QuestionIndex || progress.Solved < previous.Solved || progress.Score < previous.Score || progress.ElapsedMS < previous.ElapsedMS) {
		return false, ErrFriendMatchProgressStale
	}
	return true, f.SaveFriendMatchProgress(context.Background(), "", progress)
}

func (f *friendLifecycleStoreFake) GetFriendMatchProgress(_ context.Context, _ string) (map[uint64]FriendMatchProgress, error) {
	result := make(map[uint64]FriendMatchProgress, len(f.progress))
	for userID, progress := range f.progress {
		result[userID] = progress
	}
	return result, nil
}

func (f *friendLifecycleStoreFake) SetFriendRoomReady(_ context.Context, _ string, userID uint64, ready bool) (FriendRoom, error) {
	for index := range f.room.Players {
		if f.room.Players[index].UserID == userID {
			f.room.Players[index].Ready = ready
			f.room.Status = FriendRoomWaiting
			if friendRoomAllReady(f.room) {
				f.room.Status = FriendRoomReady
			}
			return f.room, nil
		}
	}
	return FriendRoom{}, ErrFriendRoomNotMember
}

func (f *friendLifecycleStoreFake) StartFriendRoom(_ context.Context, _ string, userID uint64, started FriendRoom) (FriendRoom, error) {
	if f.room.MatchID != "" {
		return f.room, nil
	}
	if !friendRoomHasPlayer(f.room, userID) || !friendRoomAllReady(f.room) {
		return FriendRoom{}, ErrFriendRoomNotReady
	}
	f.room = started
	return f.room, nil
}

func (f *friendLifecycleStoreFake) FinishFriendRoom(_ context.Context, _ string, matchID string) error {
	if f.room.MatchID != matchID {
		return errors.New("match id mismatch")
	}
	f.room.Status = FriendRoomFinished
	return nil
}

func (f *friendLifecycleStoreFake) SaveFriendMatchSubmission(_ context.Context, _ string, submission FriendMatchSubmissionRecord) error {
	if f.submissions == nil {
		f.submissions = map[uint64]FriendMatchSubmissionRecord{}
	}
	if _, exists := f.submissions[submission.UserID]; exists {
		return ErrFriendMatchSubmissionAlreadyExists
	}
	f.submissions[submission.UserID] = submission
	return nil
}

func (f *friendLifecycleStoreFake) GetFriendMatchSubmissions(_ context.Context, _ string) (map[uint64]FriendMatchSubmissionRecord, error) {
	result := make(map[uint64]FriendMatchSubmissionRecord, len(f.submissions))
	for userID, submission := range f.submissions {
		result[userID] = submission
	}
	return result, nil
}

func newFriendLifecycleRoom() FriendRoom {
	room := FriendRoom{
		Version:  1,
		RoomID:   "friend-246810",
		RoomCode: "246810",
		OwnerID:  3,
		RoomSeed: 42,
		Status:   FriendRoomWaiting,
		Rules:    FriendRoomRules{QuestionCount: 8, TimeLimitSeconds: 120, Target: 24, UseSameSeed: true, IntegerIntermediate: true},
		Players: []FriendRoomPlayer{
			{UserID: 3, Nickname: "玩家甲"},
			{UserID: 4, Nickname: "玩家乙"},
		},
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	room.QuestionHash, room.PuzzleIDs, room.Puzzles = friendRoomContract(room)
	return room
}

func TestFriendRoomRequiresBothPlayersReadyAndStartIsIdempotent(t *testing.T) {
	rooms := &friendLifecycleStoreFake{room: newFriendLifecycleRoom()}
	service := NewServiceWithRooms(leaderboardProfileReader{profile: testFriendProfile(3)}, &leaderboardStore{}, rooms)

	if _, err := service.StartFriendRoom(context.Background(), 3, rooms.room.RoomCode); err == nil {
		t.Fatal("StartFriendRoom() error = nil before both players are ready")
	}
	if _, err := service.ReadyFriendRoom(context.Background(), 3, rooms.room.RoomCode, true); err != nil {
		t.Fatalf("owner ready error = %v", err)
	}
	if rooms.room.Status != FriendRoomWaiting {
		t.Fatalf("room status after one ready = %q, want waiting", rooms.room.Status)
	}
	if _, err := service.ReadyFriendRoom(context.Background(), 4, rooms.room.RoomCode, true); err != nil {
		t.Fatalf("opponent ready error = %v", err)
	}
	if rooms.room.Status != FriendRoomCountdown {
		t.Fatalf("room status after both ready = %q, want countdown", rooms.room.Status)
	}
	first, err := service.StartFriendRoom(context.Background(), 4, rooms.room.RoomCode)
	if err != nil {
		t.Fatalf("first start error = %v", err)
	}
	second, err := service.StartFriendRoom(context.Background(), 3, rooms.room.RoomCode)
	if err != nil {
		t.Fatalf("repeated start error = %v", err)
	}
	if first.MatchID == "" || first.MatchID != second.MatchID || first.StartAt != second.StartAt || first.RoomSeed != second.RoomSeed || first.QuestionHash != second.QuestionHash {
		t.Fatalf("start contracts differ: first=%#v second=%#v", first, second)
	}
	if first.Status != FriendRoomCountdown {
		t.Fatalf("start status = %q, want countdown", first.Status)
	}
}

func TestFriendMatchProgressRejectsDuplicateEventID(t *testing.T) {
	room := newFriendLifecycleRoom()
	room.MatchID = room.RoomID
	room.Status = FriendRoomRunning
	rooms := &friendLifecycleStoreFake{room: room}
	service := NewServiceWithRooms(leaderboardProfileReader{profile: testFriendProfile(3)}, &leaderboardStore{}, rooms)
	input := FriendMatchProgressInput{
		QuestionIndex: 0, Solved: 0, Score: 0, ElapsedMS: 1000,
		MatchID: room.MatchID, QuestionHash: room.QuestionHash, EventID: "event-1",
	}
	if _, err := service.UpdateFriendMatchProgress(context.Background(), 3, room.RoomCode, input); err != nil {
		t.Fatalf("first progress error = %v", err)
	}
	if _, err := service.UpdateFriendMatchProgress(context.Background(), 3, room.RoomCode, input); err == nil {
		t.Fatal("duplicate event error = nil")
	}
}

func TestFriendMatchProgressFinishesRoomAndBlocksOpponent(t *testing.T) {
	room := newFriendLifecycleRoom()
	room.Status = FriendRoomRunning
	room.MatchID = room.RoomID
	rooms := &friendLifecycleStoreFake{room: room}
	service := NewServiceWithRooms(leaderboardProfileReader{profile: testFriendProfile(3)}, &leaderboardStore{}, rooms)
	finishedInput := FriendMatchProgressInput{
		QuestionIndex: 7, Solved: 8, Score: 800, ElapsedMS: 20000, Finished: true,
		MatchID: room.MatchID, QuestionHash: room.QuestionHash, EventID: "finished-1",
	}
	if _, err := service.UpdateFriendMatchProgress(context.Background(), 3, room.RoomCode, finishedInput); err != nil {
		t.Fatalf("finished progress error = %v", err)
	}
	if rooms.room.Status != FriendRoomFinished {
		t.Fatalf("room status = %q, want finished", rooms.room.Status)
	}
	if _, err := service.UpdateFriendMatchProgress(context.Background(), 4, room.RoomCode, FriendMatchProgressInput{
		QuestionIndex: 0, Solved: 0, Score: 0, ElapsedMS: 1000,
		MatchID: room.MatchID, QuestionHash: room.QuestionHash, EventID: "blocked-1",
	}); err == nil {
		t.Fatal("opponent progress error = nil after room finished")
	}
}

func TestFriendMatchSubmissionIsIdempotentAndFinishesRoomOnce(t *testing.T) {
	room := testFriendMatchRoom()
	room.ExpiresAt = time.Now().UTC().Add(time.Hour)
	rooms := &friendLifecycleStoreFake{room: room}
	service := NewServiceWithRooms(leaderboardProfileReader{profile: testFriendProfile(3)}, &leaderboardStore{}, rooms)
	firstInput := validFriendMatchSubmission()
	first, err := service.SubmitFriendMatch(context.Background(), 3, room.RoomCode, firstInput)
	if err != nil {
		t.Fatalf("first submission error = %v", err)
	}
	if !first.Pending || first.MatchResult != nil {
		t.Fatalf("first submission = %#v, want pending result", first)
	}
	secondInput := validFriendMatchSubmission()
	secondInput.IdempotencyKey = "friend-match-002"
	second, err := service.SubmitFriendMatch(context.Background(), 4, room.RoomCode, secondInput)
	if err != nil {
		t.Fatalf("second submission error = %v", err)
	}
	if second.MatchResult == nil || rooms.room.Status != FriendRoomFinished {
		t.Fatalf("second submission = %#v, room = %#v, want settled finished room", second, rooms.room)
	}
	replayed, err := service.SubmitFriendMatch(context.Background(), 3, room.RoomCode, firstInput)
	if err != nil {
		t.Fatalf("replayed submission error = %v", err)
	}
	if !replayed.IdempotencyReplayed || replayed.MatchResult == nil || replayed.Score != first.Score {
		t.Fatalf("replayed submission = %#v, want stable first result", replayed)
	}
}

func TestFriendMatchSubmissionCanSettleAfterOpponentFinishedFirst(t *testing.T) {
	room := testFriendMatchRoom()
	room.ExpiresAt = time.Now().UTC().Add(time.Hour)
	rooms := &friendLifecycleStoreFake{room: room}
	service := NewServiceWithRooms(leaderboardProfileReader{profile: testFriendProfile(3)}, &leaderboardStore{}, rooms)

	if _, err := service.UpdateFriendMatchProgress(context.Background(), 3, room.RoomCode, FriendMatchProgressInput{
		QuestionIndex: 7, Solved: 8, Score: 800, ElapsedMS: 20000, Finished: true,
		MatchID: room.MatchID, QuestionHash: room.QuestionHash, EventID: "finished-first",
	}); err != nil {
		t.Fatalf("finished progress error = %v", err)
	}
	if rooms.room.Status != FriendRoomFinished {
		t.Fatalf("room status = %q, want finished", rooms.room.Status)
	}

	first, err := service.SubmitFriendMatch(context.Background(), 3, room.RoomCode, validFriendMatchSubmission())
	if err != nil {
		t.Fatalf("first final submission error = %v", err)
	}
	if first.Pending || first.MatchResult == nil || first.Outcome == "" {
		t.Fatalf("first final submission = %#v, want immediate settled result", first)
	}

	secondInput := validFriendMatchSubmission()
	secondInput.IdempotencyKey = "friend-match-after-finish"
	second, err := service.SubmitFriendMatch(context.Background(), 4, room.RoomCode, secondInput)
	if err != nil {
		t.Fatalf("second final submission after room finished error = %v", err)
	}
	if second.MatchResult == nil || second.Outcome == "" {
		t.Fatalf("second final submission = %#v, want server-settled result", second)
	}
}
