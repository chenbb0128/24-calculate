package player

import (
	"context"
	"testing"
	"time"
)

func TestBotProgressAdvancesAtMostOneQuestionPerPoll(t *testing.T) {
	first, _ := botProgressForElapsed(0, 60_000, 8, 12345, 1)
	if first != 1 {
		t.Fatalf("first bot poll solved = %d, want 1", first)
	}
	second, _ := botProgressForElapsed(first, 60_000, 8, 12345, 1)
	if second != 2 {
		t.Fatalf("second bot poll solved = %d, want 2", second)
	}
}

func TestFriendBotFinalStateReplaysOneQuestionAtATime(t *testing.T) {
	room := FriendRoom{
		RoomSeed: 12345,
		Rules:    FriendRoomRules{QuestionCount: 10, TimeLimitSeconds: friendTimeLimitSecs},
		Puzzles:  make([]FriendPuzzleContract, 10),
	}
	solved, elapsed := friendBotFinalState(room, 180000)
	if solved <= 0 || solved > 10 || elapsed <= 0 {
		t.Fatalf("friendBotFinalState() = solved %d elapsed %d, want a server-generated progression", solved, elapsed)
	}
	if solved == 10 && elapsed > 180000 {
		t.Fatalf("bot elapsed %d exceeds match time limit", elapsed)
	}
	first, _ := botProgressForElapsed(0, 180000, 10, room.RoomSeed, 1)
	if first != 1 {
		t.Fatalf("first bot clock step = %d, want exactly one question", first)
	}
}

func TestFriendBotProgressStartsAfterCountdownWithoutWaitingForNextPoll(t *testing.T) {
	room := testFriendMatchRoom()
	room.Status = FriendRoomCountdown
	room.StartAt = time.Now().UTC().Add(-20 * time.Second).UnixMilli()
	room.Players[1] = FriendRoomPlayer{UserID: 0, Nickname: "对手", Ready: true}
	rooms := &friendLifecycleStoreFake{room: room}
	service := NewServiceWithRooms(leaderboardProfileReader{profile: testFriendProfile(3)}, &leaderboardStore{}, rooms)

	result, err := service.GetFriendMatchProgress(context.Background(), 3, room.RoomCode)
	if err != nil {
		t.Fatalf("GetFriendMatchProgress() error = %v", err)
	}
	if len(result.Players) != 2 || result.Players[1].UserID != 0 || result.Players[1].Solved < 1 {
		t.Fatalf("bot progress = %#v, want at least one server-generated answer after countdown", result.Players)
	}
}
