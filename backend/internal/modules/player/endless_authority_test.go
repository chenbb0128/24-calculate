package player

import (
	"context"
	"testing"
	"time"
)

type authorityEndlessRunStore struct {
	run EndlessRun
}

func (s *authorityEndlessRunStore) CreateEndlessRun(_ context.Context, run EndlessRun) error {
	s.run = run
	return nil
}

func (s *authorityEndlessRunStore) GetEndlessRun(_ context.Context, runID string) (EndlessRun, error) {
	if runID != s.run.RunID {
		return EndlessRun{}, ErrEndlessRunNotFound
	}
	return s.run, nil
}

func (s *authorityEndlessRunStore) UpdateEndlessRun(_ context.Context, run EndlessRun) error {
	s.run = run
	return nil
}

func newAuthorityEndlessRun(t *testing.T, userID uint64) (*authorityEndlessRunStore, EndlessPuzzle) {
	t.Helper()
	now := time.Now().UTC()
	puzzle, ok := generateEndlessPuzzle(20260818, 0, nil, now)
	if !ok {
		t.Fatal("generateEndlessPuzzle() failed")
	}
	return &authorityEndlessRunStore{run: EndlessRun{
		Version: 1, StateVersion: endlessRunStateVersion, RunID: "er_authority_test", UserID: userID,
		RunSeed: 20260818, Questions: []EndlessPuzzle{puzzle}, Status: RunRunning,
		UsedFingerprints: []string{endlessQuestionFingerprint(puzzle)},
		NextResults:      map[string]EndlessNextQuestionResponse{}, NextRequestHashes: map[string]string{},
		SubmitResults: map[string]EndlessRunSubmissionResponse{}, TimeLimitMS: 60000,
		CreatedAt: now, StartedAt: now, LastActivityAt: now, DeadlineAt: now.Add(time.Hour), ExpiresAt: now.Add(time.Hour * 2),
	}}, puzzle
}

func TestEndlessNextQuestionIsServerAuthoritativeAndIdempotent(t *testing.T) {
	store, puzzle := newAuthorityEndlessRun(t, 3)
	service := NewServiceWithRoomsAndEndless(leaderboardProfileReader{profile: testFriendProfile(3)}, &leaderboardStore{}, nil, store)
	input := EndlessNextQuestionInput{
		ProtocolVersion: 1, IdempotencyKey: "endless-q-001", QuestionIndex: 0, PuzzleID: puzzle.PuzzleID,
		ElapsedMS: 1, Solved: true, Mistakes: 999, ScoreDelta: 999999, Combo: 999, SolutionSteps: puzzle.SolutionSteps,
	}
	first, err := service.NextEndlessQuestion(context.Background(), 3, store.run.RunID, input)
	if err != nil {
		t.Fatalf("NextEndlessQuestion() error = %v", err)
	}
	if !first.Accepted || !first.Validated || first.QuestionIndex != 1 || first.Score == 999999 || first.ScoreDelta == 999999 || first.NextPuzzle == nil {
		t.Fatalf("first result = %#v", first)
	}
	second, err := service.NextEndlessQuestion(context.Background(), 3, store.run.RunID, input)
	if err != nil {
		t.Fatalf("replayed NextEndlessQuestion() error = %v", err)
	}
	if !second.IdempotencyReplayed || second.Score != first.Score || store.run.QuestionIndex != 1 {
		t.Fatalf("replay result = %#v, run = %#v", second, store.run)
	}
	input.ScoreDelta++
	if _, err := service.NextEndlessQuestion(context.Background(), 3, store.run.RunID, input); err == nil {
		t.Fatal("same idempotency key accepted with a different request")
	}
}

func TestEndlessWrongAnswerDoesNotFinishTheRun(t *testing.T) {
	store, puzzle := newAuthorityEndlessRun(t, 3)
	service := NewServiceWithRoomsAndEndless(leaderboardProfileReader{profile: testFriendProfile(3)}, &leaderboardStore{}, nil, store)
	result, err := service.NextEndlessQuestion(context.Background(), 3, store.run.RunID, EndlessNextQuestionInput{
		ProtocolVersion: 1, IdempotencyKey: "endless-wrong-1", QuestionIndex: 0, PuzzleID: puzzle.PuzzleID,
		Solved: false, ScoreDelta: 777, Combo: 777,
	})
	if err != nil {
		t.Fatalf("wrong answer error = %v", err)
	}
	if !result.Accepted || result.Status != RunRunning || result.QuestionIndex != 0 || result.Mistakes != 1 || result.NextPuzzle == nil || store.run.Status != RunRunning {
		t.Fatalf("wrong answer result = %#v, run = %#v", result, store.run)
	}
}

func TestEndlessRejectsForgedAnswerAndKeepsTheCurrentPuzzle(t *testing.T) {
	store, puzzle := newAuthorityEndlessRun(t, 3)
	service := NewServiceWithRoomsAndEndless(leaderboardProfileReader{profile: testFriendProfile(3)}, &leaderboardStore{}, nil, store)
	forged := cloneSolutionSteps(puzzle.SolutionSteps)
	forged[0].Operator = "%"
	if _, err := service.NextEndlessQuestion(context.Background(), 3, store.run.RunID, EndlessNextQuestionInput{
		ProtocolVersion: 1, IdempotencyKey: "endless-forged-1", QuestionIndex: 0, PuzzleID: puzzle.PuzzleID,
		Solved: true, SolutionSteps: forged,
	}); err == nil {
		t.Fatal("forged solution was accepted")
	}
	if store.run.QuestionIndex != 0 || len(store.run.Attempts) != 0 {
		t.Fatalf("run changed after forged solution: %#v", store.run)
	}
}

func TestEndlessDeadlineFinishesWithoutAcceptingLateAnswer(t *testing.T) {
	store, puzzle := newAuthorityEndlessRun(t, 3)
	store.run.DeadlineAt = time.Now().UTC().Add(-time.Second)
	service := NewServiceWithRoomsAndEndless(leaderboardProfileReader{profile: testFriendProfile(3)}, &leaderboardStore{}, nil, store)
	result, err := service.NextEndlessQuestion(context.Background(), 3, store.run.RunID, EndlessNextQuestionInput{
		ProtocolVersion: 1, IdempotencyKey: "endless-timeout-1", QuestionIndex: 0, PuzzleID: puzzle.PuzzleID,
		Solved: true, SolutionSteps: puzzle.SolutionSteps,
	})
	if err != nil {
		t.Fatalf("deadline request error = %v", err)
	}
	if result.Accepted || result.Status != RunFinished || result.NextPuzzle != nil || result.QuestionsSolved != 0 || store.run.Status != RunFinished {
		t.Fatalf("deadline result = %#v, run = %#v", result, store.run)
	}
}

func TestEndlessCanAdvanceThroughOneHundredQuestions(t *testing.T) {
	store, puzzle := newAuthorityEndlessRun(t, 3)
	store.run.DeadlineAt = time.Now().UTC().Add(time.Hour)
	service := NewServiceWithRoomsAndEndless(leaderboardProfileReader{profile: testFriendProfile(3)}, &leaderboardStore{}, nil, store)
	seen := map[string]bool{endlessQuestionFingerprint(puzzle): true}
	for index := 0; index < 100; index++ {
		current := store.run.Questions[store.run.QuestionIndex]
		result, err := service.NextEndlessQuestion(context.Background(), 3, store.run.RunID, EndlessNextQuestionInput{
			ProtocolVersion: 1, IdempotencyKey: "endless-advance-" + formatThree(index), QuestionIndex: store.run.QuestionIndex,
			PuzzleID: current.PuzzleID, Solved: true, SolutionSteps: current.SolutionSteps,
		})
		if err != nil {
			t.Fatalf("question %d error = %v", index, err)
		}
		if result.QuestionIndex != index+1 || result.Status != RunRunning {
			t.Fatalf("question %d result = %#v", index, result)
		}
		if result.NextPuzzle == nil {
			t.Fatalf("question %d has no next puzzle", index)
		}
		fingerprint := endlessQuestionFingerprint(store.run.Questions[store.run.QuestionIndex])
		if seen[fingerprint] {
			t.Fatalf("question %d repeated fingerprint %q", index+1, fingerprint)
		}
		seen[fingerprint] = true
	}
	if store.run.QuestionIndex != 100 || endlessQuestionsSolved(store.run.Attempts) != 100 {
		t.Fatalf("run after 100 questions = %#v", store.run)
	}
}

func TestEndlessSubmitUsesSavedStateInsteadOfClientSummary(t *testing.T) {
	store, puzzle := newAuthorityEndlessRun(t, 3)
	progress := newProgressStore(t)
	service := NewServiceWithRoomsAndEndless(leaderboardProfileReader{profile: testFriendProfile(3)}, progress, nil, store)
	if _, err := service.NextEndlessQuestion(context.Background(), 3, store.run.RunID, EndlessNextQuestionInput{
		ProtocolVersion: 1, IdempotencyKey: "endless-submit-q", QuestionIndex: 0, PuzzleID: puzzle.PuzzleID,
		Solved: true, SolutionSteps: puzzle.SolutionSteps,
	}); err != nil {
		t.Fatalf("advance before submit error = %v", err)
	}
	result, err := service.SubmitEndlessRun(context.Background(), 3, store.run.RunID, EndlessRunSubmissionInput{
		ProtocolVersion: 1, IdempotencyKey: "endless-submit-1", RunID: store.run.RunID,
		Attempts: []EndlessRunAttemptInput{{QuestionIndex: 0, Score: 999999, ScoreDelta: 999999}},
		Summary:  EndlessRunSummaryInput{Questions: 999, Score: 999999, ElapsedMS: 1, Mistakes: 0, BestCombo: 999},
	})
	if err != nil {
		t.Fatalf("submit error = %v", err)
	}
	if result.Status != RunSubmitted || result.Score != store.run.Score || result.Questions != 1 || result.Score == 999999 || !result.Validated || !result.LeaderboardUpdated {
		t.Fatalf("submit result = %#v, run = %#v", result, store.run)
	}
	replayed, err := service.SubmitEndlessRun(context.Background(), 3, store.run.RunID, EndlessRunSubmissionInput{
		ProtocolVersion: 1, IdempotencyKey: "endless-submit-1", RunID: store.run.RunID,
	})
	if err != nil || !replayed.IdempotencyReplayed || replayed.RewardCoins != result.RewardCoins {
		t.Fatalf("submit replay = %#v, error = %v", replayed, err)
	}
}

func formatThree(value int) string {
	if value < 10 {
		return "00" + itoa(value)
	}
	if value < 100 {
		return "0" + itoa(value)
	}
	return itoa(value)
}
