package player

import "testing"

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
