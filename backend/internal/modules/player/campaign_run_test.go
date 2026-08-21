package player

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/example/go-service/internal/modules/user"
	db "github.com/example/go-service/internal/store/sqlc"
)

type campaignRunStoreFake struct {
	runs map[string]CampaignRun
}

func (f *campaignRunStoreFake) CreateCampaignRun(_ context.Context, run CampaignRun) error {
	if f.runs == nil {
		f.runs = map[string]CampaignRun{}
	}
	f.runs[run.RunID] = run
	return nil
}

func (f *campaignRunStoreFake) GetCampaignRun(_ context.Context, runID string) (CampaignRun, error) {
	run, ok := f.runs[runID]
	if !ok {
		return CampaignRun{}, ErrCampaignRunNotFound
	}
	return run, nil
}

func (f *campaignRunStoreFake) CreateEndlessRun(context.Context, EndlessRun) error {
	return errors.New("not used in campaign test")
}

func (f *campaignRunStoreFake) GetEndlessRun(context.Context, string) (EndlessRun, error) {
	return EndlessRun{}, errors.New("not used in campaign test")
}

type campaignTestStore struct {
	*leaderboardStore
	profile       db.PlayerProfile
	completed     []CompleteLevelParams
	nextReplay    bool
	completeCalls int
}

func (s *campaignTestStore) GetPlayerProfile(context.Context, uint64) (db.PlayerProfile, error) {
	return s.profile, nil
}

func (s *campaignTestStore) CompleteLevel(_ context.Context, params CompleteLevelParams) (CompleteLevelResult, error) {
	s.completeCalls++
	s.completed = append(s.completed, params)
	return CompleteLevelResult{
		LevelID:             params.LevelID,
		Stars:               params.Stars,
		BestScore:           params.Score,
		RewardCoins:         17,
		Coins:               117,
		UnlockedLevel:       params.LevelID + 1,
		IdempotencyReplayed: s.nextReplay,
	}, nil
}

func newCampaignTestStore(t *testing.T, unlocked int) *campaignTestStore {
	t.Helper()
	progress := map[string]any{"unlocked_level": unlocked}
	encoded, err := json.Marshal(progress)
	if err != nil {
		t.Fatal(err)
	}
	return &campaignTestStore{
		leaderboardStore: &leaderboardStore{},
		profile:          db.PlayerProfile{UserID: 3, ProgressJSON: string(encoded)},
	}
}

func testCampaignRun() CampaignRun {
	numbers := []int{1, 2, 3, 4}
	solution := friendSolveDetailed(numbers, 1)[0]
	return CampaignRun{
		Version:       campaignRunProtocolVersion,
		RunID:         "cr_test",
		UserID:        3,
		LevelID:       0,
		QuestionCount: 1,
		TimeLimitMS:   68000,
		AllowHint:     true,
		Questions: []CampaignPuzzle{{
			PuzzleID:      "C001-Q01",
			Numbers:       numbers,
			Rules:         friendPuzzleRules(),
			TimeLimitMS:   68000,
			SolutionSteps: solution.steps,
		}},
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
}

func validCampaignSubmission(run CampaignRun) CampaignRunSubmissionInput {
	question := run.Questions[0]
	steps := friendSolveDetailed(question.Numbers, 1)[0].steps
	return CampaignRunSubmissionInput{
		ProtocolVersion: campaignRunProtocolVersion,
		IdempotencyKey:  "campaign_" + run.RunID,
		RunID:           run.RunID,
		LevelID:         run.LevelID,
		Attempts: []CampaignRunAttemptInput{{
			PuzzleID:      question.PuzzleID,
			QuestionIndex: 0,
			ElapsedMS:     1200,
			Solved:        true,
			Mistakes:      0,
			Hints:         0,
			Score:         100,
			ScoreDelta:    100,
			SolutionSteps: steps,
		}},
		Summary: CampaignRunSummaryInput{Questions: 1, Score: 100, ElapsedMS: 1200, Stars: 3},
	}
}

func TestStartCampaignRunReturnsPublicQuestionsWithoutAnswerSteps(t *testing.T) {
	store := newCampaignTestStore(t, 0)
	runs := &campaignRunStoreFake{}
	service := NewServiceWithRoomsAndEndless(
		leaderboardProfileReader{profile: user.ProfileResponse{ID: 3}},
		store, nil, runs,
	)

	result, err := service.StartCampaignRun(context.Background(), 3, 0)
	if err != nil {
		t.Fatalf("StartCampaignRun() error = %v", err)
	}
	if result.RunID == "" || len(result.Puzzles) != 3 {
		t.Fatalf("result = %#v, want a three-question run", result)
	}
	for _, puzzle := range result.Puzzles {
		if puzzle.PuzzleID == "" || len(puzzle.Numbers) != 4 {
			t.Fatalf("public puzzle = %#v, want puzzle id and four numbers", puzzle)
		}
	}
	if stored := runs.runs[result.RunID]; len(stored.Questions) != 3 || len(stored.Questions[0].SolutionSteps) != 3 {
		t.Fatalf("stored run does not contain hidden answer steps: %#v", stored)
	}
}

func TestCampaignRunQuestionsAreStableAcrossUsersAndRuns(t *testing.T) {
	store := newCampaignTestStore(t, 1)
	runs := &campaignRunStoreFake{}
	service := NewServiceWithRoomsAndEndless(
		leaderboardProfileReader{profile: user.ProfileResponse{ID: 3}},
		store, nil, runs,
	)
	service.SetCampaignContent("v1", "test-campaign-secret")

	first, err := service.StartCampaignRun(context.Background(), 3, 0)
	if err != nil {
		t.Fatalf("first StartCampaignRun() error = %v", err)
	}
	second, err := service.StartCampaignRun(context.Background(), 4, 0)
	if err != nil {
		t.Fatalf("second StartCampaignRun() error = %v", err)
	}
	third, err := service.StartCampaignRun(context.Background(), 3, 0)
	if err != nil {
		t.Fatalf("third StartCampaignRun() error = %v", err)
	}

	if reflect.DeepEqual(first.Puzzles, second.Puzzles) == false || reflect.DeepEqual(first.Puzzles, third.Puzzles) == false {
		t.Fatalf("campaign questions are not stable: first=%#v second=%#v third=%#v", first.Puzzles, second.Puzzles, third.Puzzles)
	}
	if first.RunID == second.RunID || first.RunID == third.RunID || second.RunID == third.RunID {
		t.Fatalf("run IDs must remain independent: %q %q %q", first.RunID, second.RunID, third.RunID)
	}
	for index, puzzle := range first.Puzzles {
		wantID := fmt.Sprintf("C001-Q%02d", index+1)
		if puzzle.PuzzleID != wantID || puzzle.QuestionHash == "" {
			t.Fatalf("puzzle %d = %#v, want stable puzzle ID and hash", index, puzzle)
		}
		if len(puzzle.Numbers) != 4 || slicesContainAbove(puzzle.Numbers, campaignMaxDigit) || len(verifiedFriendSolutions(puzzle.Numbers, puzzle.Rules, 40)) == 0 {
			t.Fatalf("puzzle %d is not a validated four-number puzzle: %#v", index, puzzle)
		}
	}

	levelTwo, err := service.StartCampaignRun(context.Background(), 3, 1)
	if err != nil {
		t.Fatalf("level two StartCampaignRun() error = %v", err)
	}
	if reflect.DeepEqual(first.Puzzles, levelTwo.Puzzles) {
		t.Fatal("different campaign levels unexpectedly returned the same questions")
	}
	if first.Puzzles[0].QuestionHash == levelTwo.Puzzles[0].QuestionHash {
		t.Fatal("different campaign levels unexpectedly returned the same question hash")
	}
}

func slicesContainAbove(values []int, maximum int) bool {
	for _, value := range values {
		if value < 1 || value > maximum {
			return true
		}
	}
	return false
}

func TestCampaignRunSubmissionUsesThePersistedDeterministicQuestions(t *testing.T) {
	store := newCampaignTestStore(t, 0)
	runs := &campaignRunStoreFake{}
	service := NewServiceWithRoomsAndEndless(
		leaderboardProfileReader{profile: user.ProfileResponse{ID: 3}},
		store, nil, runs,
	)
	service.SetCampaignContent("v1", "test-campaign-secret")
	started, err := service.StartCampaignRun(context.Background(), 3, 0)
	if err != nil {
		t.Fatalf("StartCampaignRun() error = %v", err)
	}
	run := runs.runs[started.RunID]
	attempts := make([]CampaignRunAttemptInput, 0, len(run.Questions))
	previousScore := 0
	for index, question := range run.Questions {
		solution := verifiedFriendSolutions(question.Numbers, question.Rules, 40)[0].steps
		score := campaignProgressScore(index+1, len(run.Questions), 0, 0)
		attempts = append(attempts, CampaignRunAttemptInput{
			PuzzleID: question.PuzzleID, QuestionHash: question.QuestionHash,
			QuestionIndex: index, ElapsedMS: 1000, Solved: true,
			Score: score, ScoreDelta: score - previousScore, Combo: index + 1,
			SolutionSteps: solution,
		})
		previousScore = score
	}

	result, err := service.SubmitCampaignRun(context.Background(), 3, started.RunID, CampaignRunSubmissionInput{
		ProtocolVersion: campaignRunProtocolVersion,
		IdempotencyKey:  "campaign_" + started.RunID,
		RunID:           started.RunID,
		LevelID:         run.LevelID,
		Attempts:        attempts,
		Summary: CampaignRunSummaryInput{
			Questions: len(attempts), Score: previousScore, ElapsedMS: 1000 * len(attempts), Stars: 3, BestCombo: len(attempts),
		},
	})
	if err != nil {
		t.Fatalf("SubmitCampaignRun() error = %v", err)
	}
	if !result.Validated || result.Score != 100 || result.Questions != len(run.Questions) {
		t.Fatalf("submission result = %#v, want validated full deterministic run", result)
	}
}

func TestResumeCampaignRunKeepsQuestionsCreatedBeforeContentChange(t *testing.T) {
	store := newCampaignTestStore(t, 0)
	runs := &campaignRunStoreFake{}
	service := NewServiceWithRoomsAndEndless(
		leaderboardProfileReader{profile: user.ProfileResponse{ID: 3}},
		store, nil, runs,
	)
	service.SetCampaignContent("v1", "test-campaign-secret")
	started, err := service.StartCampaignRun(context.Background(), 3, 0)
	if err != nil {
		t.Fatalf("StartCampaignRun() error = %v", err)
	}
	original := append([]CampaignPuzzlePublic(nil), started.Puzzles...)

	service.SetCampaignContent("v2", "another-test-campaign-secret")
	resumed, err := service.ResumeCampaignRun(context.Background(), 3, started.RunID)
	if err != nil {
		t.Fatalf("ResumeCampaignRun() error = %v", err)
	}
	if !reflect.DeepEqual(original, resumed.Puzzles) {
		t.Fatalf("resume replaced persisted questions: original=%#v resumed=%#v", original, resumed.Puzzles)
	}
}

func TestValidateCampaignRunSubmissionRejectsForgedScore(t *testing.T) {
	run := testCampaignRun()
	input := validCampaignSubmission(run)
	input.Attempts[0].Score = 1
	input.Attempts[0].ScoreDelta = 1
	input.Summary.Score = 1
	input.Summary.Stars = 1
	if _, err := validateCampaignRunSubmission(run, input); err == nil {
		t.Fatal("validateCampaignRunSubmission() error = nil for forged score")
	}
}

func TestValidateCampaignRunSubmissionRejectsForgedStars(t *testing.T) {
	run := testCampaignRun()
	input := validCampaignSubmission(run)
	input.Summary.Stars = 1
	if _, err := validateCampaignRunSubmission(run, input); err == nil {
		t.Fatal("validateCampaignRunSubmission() error = nil for forged stars")
	}
}

func TestSubmitCampaignRunValidatesAndDelegatesIdempotentCompletion(t *testing.T) {
	store := newCampaignTestStore(t, 0)
	run := testCampaignRun()
	runs := &campaignRunStoreFake{runs: map[string]CampaignRun{run.RunID: run}}
	service := NewServiceWithRoomsAndEndless(
		leaderboardProfileReader{profile: user.ProfileResponse{ID: 3}},
		store, nil, runs,
	)

	result, err := service.SubmitCampaignRun(context.Background(), 3, run.RunID, validCampaignSubmission(run))
	if err != nil {
		t.Fatalf("SubmitCampaignRun() error = %v", err)
	}
	if !result.Validated || result.Score != 100 || result.Stars != 3 || store.completeCalls != 1 {
		t.Fatalf("result = %#v, complete calls = %d", result, store.completeCalls)
	}

	store.nextReplay = true
	replayed, err := service.SubmitCampaignRun(context.Background(), 3, run.RunID, validCampaignSubmission(run))
	if err != nil {
		t.Fatalf("replayed SubmitCampaignRun() error = %v", err)
	}
	if !replayed.IdempotencyReplayed || store.completeCalls != 2 {
		t.Fatalf("replayed result = %#v, complete calls = %d", replayed, store.completeCalls)
	}
}

func TestSubmitCampaignRunRejectsAnotherUser(t *testing.T) {
	store := newCampaignTestStore(t, 0)
	run := testCampaignRun()
	runs := &campaignRunStoreFake{runs: map[string]CampaignRun{run.RunID: run}}
	service := NewServiceWithRoomsAndEndless(
		leaderboardProfileReader{profile: user.ProfileResponse{ID: 4}},
		store, nil, runs,
	)
	if _, err := service.SubmitCampaignRun(context.Background(), 4, run.RunID, validCampaignSubmission(run)); err == nil {
		t.Fatal("SubmitCampaignRun() error = nil for another user")
	}
}
