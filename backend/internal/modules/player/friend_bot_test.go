package player

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBotDifficultyFollowsServerRankTier(t *testing.T) {
	tests := map[string]int{
		RankTierBronze:   botDifficultyEasy,
		RankTierSilver:   botDifficultyStandard,
		RankTierGold:     botDifficultyAdvanced,
		RankTierPlatinum: botDifficultyHard,
		RankTierDiamond:  botDifficultyHigh,
		RankTierMaster:   botDifficultyHigh,
		RankTierKing:     botDifficultyHigh,
	}
	for tier, want := range tests {
		if got := botDifficultyForRank(tier); got != want {
			t.Fatalf("botDifficultyForRank(%q) = %d, want %d", tier, got, want)
		}
	}
}

func TestPublicBotMatchDoesNotExposeBotMetadata(t *testing.T) {
	payload, err := json.Marshal(publicMatchmakingResponse(MatchmakingTicket{
		TicketID: "mm-test", Status: "matched", IsBot: true, BotDifficulty: "high",
		Room: &FriendRoom{MatchSource: "bot", Players: []FriendRoomPlayer{{UserID: 7}, {UserID: 0, Nickname: "对手"}}},
	}))
	if err != nil {
		t.Fatalf("marshal public bot response: %v", err)
	}
	value := strings.ToLower(string(payload))
	for _, forbidden := range []string{"\"is_bot\"", "\"bot_difficulty\"", "\"bot_user_id\"", "\"ai\"", "\"match_source\":\"bot\""} {
		if strings.Contains(value, forbidden) {
			t.Fatalf("public bot response contains %q: %s", forbidden, payload)
		}
	}
}

func TestBotDoesNotCompleteAllQuestionsAfterOneSecond(t *testing.T) {
	for difficulty := botDifficultyEasy; difficulty <= botDifficultyHigh; difficulty++ {
		solved, _ := botProgressForElapsed(0, 1000, 10, 42, difficulty)
		if solved != 0 {
			t.Fatalf("difficulty %d solved %d after one second, want 0", difficulty, solved)
		}
	}
}

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
