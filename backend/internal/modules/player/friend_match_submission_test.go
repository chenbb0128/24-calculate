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
				Score:           100,
				ScoreDelta:      100,
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
				Score:           100,
				ScoreDelta:      0,
				RoomSeed:        42,
				QuestionHash:    room.QuestionHash,
				EventID:         "friend-123456:1:2",
			},
		},
		Summary: FriendMatchSummaryInput{
			PlayerSolved:   1,
			PlayerScore:    100,
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
	if !result.Validated || result.Mode != LeaderboardFriend || result.Score != 100 || result.Questions != 1 || result.ElapsedMS != 2000 {
		t.Fatalf("result = %#v, want validated friend submission", result)
	}
	if len(store.created) != 1 {
		t.Fatalf("created submissions = %d, want 1", len(store.created))
	}
	if store.created[0].RoomID != "friend-123456" || store.created[0].Outcome != "" {
		t.Fatalf("stored submission = %#v, want room and server-derived pending outcome", store.created[0])
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
