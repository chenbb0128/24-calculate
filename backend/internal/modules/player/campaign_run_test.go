package player

import (
	"context"
	"encoding/json"
	"errors"
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
