package player

import (
	"context"
	"testing"

	db "github.com/example/go-service/internal/store/sqlc"
)

type friendSubmissionStore struct {
	*leaderboardStore
	created []db.CreateLeaderboardSubmissionParams
}

func (s *friendSubmissionStore) CreateLeaderboardSubmission(_ context.Context, params db.CreateLeaderboardSubmissionParams) error {
	s.created = append(s.created, params)
	return nil
}

func testFriendMatchRoom() FriendRoom {
	room := FriendRoom{
		RoomID:   "friend-123456",
		RoomCode: "123456",
		RoomSeed: 42,
		MatchID:  "friend-123456",
		Status:   FriendRoomRunning,
		Rules:    FriendRoomRules{QuestionCount: 8, TimeLimitSeconds: 120},
		Players: []FriendRoomPlayer{
			{UserID: 3, Nickname: "player one", Ready: true},
			{UserID: 4, Nickname: "player two", Ready: true},
		},
	}
	room.QuestionHash, room.PuzzleIDs, room.Puzzles = friendRoomContract(room)
	return room
}

func validFriendMatchSubmission() FriendMatchSubmissionInput {
	room := testFriendMatchRoom()
	firstSolution := friendSolveDetailed(room.Puzzles[0].Numbers, 1)[0].steps
	return FriendMatchSubmissionInput{
		ProtocolVersion: 2,
		Action:          "submitFriendMatch",
		IdempotencyKey:  "friend-match-001",
		MatchID:         "friend-123456",
		RoomID:          "friend-123456",
		RoomSeed:        42,
		QuestionCount:   8,
		QuestionHash:    room.QuestionHash,
		PuzzleIDs:       room.PuzzleIDs,
		Attempts: []FriendMatchAttemptInput{
			{
				ProtocolVersion: 2,
				PuzzleID:        room.PuzzleIDs[0],
				QuestionIndex:   0,
				ElapsedMS:       1000,
				Solved:          true,
				Mistakes:        0,
				Score:           714,
				ScoreDelta:      714,
				RoomSeed:        42,
				QuestionHash:    room.QuestionHash,
				EventID:         "friend-123456:0:1",
				SolutionSteps:   firstSolution,
			},
			{
				ProtocolVersion: 2,
				PuzzleID:        room.PuzzleIDs[1],
				QuestionIndex:   1,
				ElapsedMS:       2000,
				Solved:          false,
				Mistakes:        1,
				Score:           714,
				ScoreDelta:      0,
				RoomSeed:        42,
				QuestionHash:    room.QuestionHash,
				EventID:         "friend-123456:1:2",
			},
		},
		Summary: FriendMatchSummaryInput{
			PlayerSolved:   1,
			PlayerScore:    714,
			PlayerMistakes: 1,
			PlayerElapsed:  2,
			Outcome:        "win",
		},
	}
}

func TestSubmitFriendMatchAcceptsValidatedSubmission(t *testing.T) {
	store := &friendSubmissionStore{leaderboardStore: &leaderboardStore{}}
	rooms := &friendRoomStoreFake{room: testFriendMatchRoom()}
	service := NewServiceWithRooms(leaderboardProfileReader{profile: testFriendProfile(3)}, store, rooms)

	result, err := service.SubmitFriendMatch(context.Background(), 3, "123456", validFriendMatchSubmission())
	if err != nil {
		t.Fatalf("SubmitFriendMatch() error = %v", err)
	}
	if !result.Validated || result.Mode != LeaderboardFriend || result.Score != 714 || result.Questions != 1 || result.ElapsedMS != 2000 {
		t.Fatalf("result = %#v, want validated friend submission", result)
	}
	if len(store.created) != 1 {
		t.Fatalf("created submissions = %d, want 1", len(store.created))
	}
	if store.created[0].RoomID != "friend-123456" || store.created[0].Outcome != "" {
		t.Fatalf("stored submission = %#v, want room and server-derived pending outcome", store.created[0])
	}
}

func TestSubmitFriendMatchAcceptsRetryAndRecomputesScore(t *testing.T) {
	room := testFriendMatchRoom()
	input := validFriendMatchSubmission()
	firstSolution := friendSolveDetailed(room.Puzzles[0].Numbers, 1)[0].steps
	input.Attempts = []FriendMatchAttemptInput{
		{
			ProtocolVersion: 2, PuzzleID: room.PuzzleIDs[0], QuestionIndex: 0,
			ElapsedMS: 1000, Solved: false, Mistakes: 1, Score: 0, ScoreDelta: 0,
			RoomSeed: 42, QuestionHash: room.QuestionHash, EventID: "friend-123456:0:wrong",
		},
		{
			ProtocolVersion: 2, PuzzleID: room.PuzzleIDs[0], QuestionIndex: 0,
			ElapsedMS: 2000, Solved: true, Mistakes: 1, Score: 703, ScoreDelta: 703,
			RoomSeed: 42, QuestionHash: room.QuestionHash, EventID: "friend-123456:0:right",
			SolutionSteps: firstSolution,
		},
	}
	input.Summary = FriendMatchSummaryInput{PlayerSolved: 1, PlayerScore: 703, PlayerMistakes: 1, PlayerElapsed: 2}

	store := &friendSubmissionStore{leaderboardStore: &leaderboardStore{}}
	rooms := &friendRoomStoreFake{room: room}
	service := NewServiceWithRooms(leaderboardProfileReader{profile: testFriendProfile(3)}, store, rooms)
	result, err := service.SubmitFriendMatch(context.Background(), 3, room.RoomCode, input)
	if err != nil {
		t.Fatalf("SubmitFriendMatch() error = %v", err)
	}
	if result.Score != 703 || result.Questions != 1 || result.ElapsedMS != 2000 {
		t.Fatalf("result = %#v, want server-recomputed retry score", result)
	}
}

func TestSubmitFriendMatchRejectsScoreTamperingWithinLegacyRange(t *testing.T) {
	store := &friendSubmissionStore{leaderboardStore: &leaderboardStore{}}
	rooms := &friendRoomStoreFake{room: testFriendMatchRoom()}
	service := NewServiceWithRooms(leaderboardProfileReader{profile: testFriendProfile(3)}, store, rooms)
	input := validFriendMatchSubmission()
	input.Attempts[0].Score = 999
	input.Attempts[0].ScoreDelta = 999
	input.Summary.PlayerScore = 999
	if _, err := service.SubmitFriendMatch(context.Background(), 3, "123456", input); err == nil {
		t.Fatal("SubmitFriendMatch() error = nil for a score within the old range but outside the scoring formula")
	}
}

func TestSubmitFriendMatchRejectsTamperedScoreAndQuestionOrder(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*FriendMatchSubmissionInput)
	}{
		{
			name: "score delta",
			mutate: func(input *FriendMatchSubmissionInput) {
				input.Attempts[1].ScoreDelta = 100
			},
		},
		{
			name: "question order",
			mutate: func(input *FriendMatchSubmissionInput) {
				input.Attempts[1].PuzzleID = input.PuzzleIDs[2]
			},
		},
		{
			name: "solution steps",
			mutate: func(input *FriendMatchSubmissionInput) {
				input.Attempts[0].SolutionSteps[0].Operator = "-"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &friendSubmissionStore{leaderboardStore: &leaderboardStore{}}
			rooms := &friendRoomStoreFake{room: testFriendMatchRoom()}
			service := NewServiceWithRooms(leaderboardProfileReader{profile: testFriendProfile(3)}, store, rooms)
			input := validFriendMatchSubmission()
			tt.mutate(&input)

			if _, err := service.SubmitFriendMatch(context.Background(), 3, "123456", input); err == nil {
				t.Fatal("SubmitFriendMatch() error = nil, want validation error")
			}
			if len(store.created) != 0 {
				t.Fatalf("created submissions = %d, want 0 for invalid input", len(store.created))
			}
		})
	}
}

func TestSubmitFriendMatchFinishesImmediatelyWhenPlayerCompletesRound(t *testing.T) {
	room := testFriendMatchRoom()
	room.Rules.QuestionCount = 1
	room.Rules.TimeLimitSeconds = friendTimeLimitSecs
	room.Puzzles = generateFriendPuzzleContract(room.RoomSeed, 1)
	room.QuestionHash, room.PuzzleIDs, room.Puzzles = friendRoomContract(room)
	rooms := &friendLifecycleStoreFake{room: room}
	service := NewServiceWithRooms(leaderboardProfileReader{profile: testFriendProfile(3)}, &leaderboardStore{}, rooms)
	solution := friendSolveDetailed(room.Puzzles[0].Numbers, 1)[0].steps
	score := friendMatchScoreDelta(friendTimeLimitSecs, 1000, 0, 0)
	result, err := service.SubmitFriendMatch(context.Background(), 3, room.RoomCode, FriendMatchSubmissionInput{
		ProtocolVersion: 2, Action: "submitFriendMatch", IdempotencyKey: "immediate-001",
		MatchID: room.MatchID, RoomID: room.RoomID, RoomSeed: room.RoomSeed,
		QuestionCount: 1, QuestionHash: room.QuestionHash, PuzzleIDs: room.PuzzleIDs,
		Attempts: []FriendMatchAttemptInput{{
			ProtocolVersion: 2, PuzzleID: room.PuzzleIDs[0], QuestionIndex: 0,
			ElapsedMS: 1000, Solved: true, Mistakes: 0, Score: score, ScoreDelta: score,
			RoomSeed: room.RoomSeed, QuestionHash: room.QuestionHash, EventID: "immediate-001:0", SolutionSteps: solution,
		}},
		Summary: FriendMatchSummaryInput{PlayerSolved: 1, PlayerScore: score, PlayerElapsed: 1},
	})
	if err != nil {
		t.Fatalf("SubmitFriendMatch() error = %v", err)
	}
	if result.Pending || result.MatchResult == nil || rooms.room.Status != FriendRoomFinished {
		t.Fatalf("result = %#v, room = %#v, want immediate settled result", result, rooms.room)
	}
	if result.MatchResult.Outcome != "win" {
		t.Fatalf("match result = %#v, want win against non-submitting opponent", result.MatchResult)
	}
}
