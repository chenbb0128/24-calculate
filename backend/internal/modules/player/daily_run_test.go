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

type dailyRunStoreFake struct {
	runs map[string]DailyRun
}

func (f *dailyRunStoreFake) CreateDailyRun(_ context.Context, run DailyRun) error {
	if f.runs == nil {
		f.runs = map[string]DailyRun{}
	}
	f.runs[run.RunID] = run
	return nil
}

func (f *dailyRunStoreFake) GetDailyRun(_ context.Context, runID string) (DailyRun, error) {
	run, ok := f.runs[runID]
	if !ok {
		return DailyRun{}, ErrDailyRunNotFound
	}
	return run, nil
}

func (f *dailyRunStoreFake) CreateEndlessRun(context.Context, EndlessRun) error {
	return errors.New("not used in daily test")
}

func (f *dailyRunStoreFake) GetEndlessRun(context.Context, string) (EndlessRun, error) {
	return EndlessRun{}, errors.New("not used in daily test")
}

type dailyTestStore struct {
	*leaderboardStore
	profile    db.PlayerProfile
	dailyCalls int
	nextReplay bool
	completed  CompleteDailyParams
}

func (s *dailyTestStore) GetPlayerProfile(context.Context, uint64) (db.PlayerProfile, error) {
	return s.profile, nil
}

func (s *dailyTestStore) CompleteDaily(_ context.Context, params CompleteDailyParams) (CompleteDailyResult, error) {
	s.dailyCalls++
	s.completed = params
	return CompleteDailyResult{
		DateKey:             time.Now().In(shanghaiLocation).Format("2006-01-02"),
		Score:               params.Score,
		BestScore:           params.Score,
		Streak:              1,
		RewardCoins:         15,
		Coins:               115,
		IdempotencyReplayed: s.nextReplay,
	}, nil
}

func testDailyRun() DailyRun {
	dateKey := time.Now().In(shanghaiLocation).Format("2006-01-02")
	seed, _ := dailyDateSeed(dateKey)
	rule := dailyRuleForIndex(1)
	questions := generateDailyRunQuestions(dateKey, seed, rule)
	return DailyRun{
		Version:       dailyRunProtocolVersion,
		RunID:         "dr_test",
		UserID:        3,
		DateKey:       dateKey,
		Seed:          seed,
		Rule:          rule,
		QuestionCount: dailyQuestionCount,
		TimeLimitMS:   rule.TimeLimitSeconds * 1000,
		Questions:     questions,
		CreatedAt:     time.Now().UTC(),
		ExpiresAt:     time.Now().UTC().Add(time.Hour),
	}
}

func validDailySubmission(run DailyRun) DailyRunSubmissionInput {
	input := DailyRunSubmissionInput{
		ProtocolVersion: dailyRunProtocolVersion,
		IdempotencyKey:  "daily_" + run.DateKey,
		RunID:           run.RunID,
		DateKey:         run.DateKey,
	}
	questionCount := len(run.Questions)
	previousScore := 0
	for index, question := range run.Questions {
		steps := friendSolveDetailed(question.Numbers, 500)
		var solution []FriendMatchSolutionStep
		for _, candidate := range steps {
			if dailySolutionValid(question.Numbers, candidate.steps, question.Rules) {
				solution = candidate.steps
				break
			}
		}
		elapsed := 1200
		delta := dailyScoreDelta(question.TimeLimitMS, elapsed, index, 0)
		previousScore += delta
		input.Attempts = append(input.Attempts, DailyRunAttemptInput{
			PuzzleID: question.PuzzleID, QuestionIndex: index, ElapsedMS: elapsed,
			Solved: true, Score: previousScore, ScoreDelta: delta, Combo: index + 1,
			SolutionSteps: solution,
		})
	}
	input.Summary = DailyRunSummaryInput{
		DateKey: run.DateKey, Questions: questionCount, Score: previousScore,
		ElapsedMS: 1200 * questionCount, BestCombo: questionCount,
	}
	return input
}

func TestDailyGenerationHonorsEveryRule(t *testing.T) {
	for index := 0; index < dailyRuleCount; index++ {
		rule := dailyRuleForIndex(index)
		questions := generateDailyRunQuestions("2026-08-16", int64(index+1), rule)
		if len(questions) != dailyQuestionCount {
			t.Fatalf("rule %d (%s) generated %d questions, want %d", index, rule.ID, len(questions), dailyQuestionCount)
		}
		for _, question := range questions {
			if !dailySolutionValid(question.Numbers, question.SolutionSteps, question.Rules) {
				t.Fatalf("rule %s generated invalid question %#v", rule.ID, question)
			}
		}
	}
}

func TestDailyGenerationUsesFiveQuestionDifficultyContract(t *testing.T) {
	for index := 0; index < dailyRuleCount; index++ {
		rule := dailyRuleForIndex(index)
		questions := generateDailyRunQuestions("2026-08-16", int64(index+1), rule)
		if rule.HintCount != 1 || !rule.AllowHint {
			t.Fatalf("rule %s hint contract = %+v, want one allowed hint", rule.ID, rule)
		}
		wantSeconds := 75
		if rule.TimeBonus {
			wantSeconds = 55
		}
		if rule.TimeLimitSeconds != wantSeconds {
			t.Fatalf("rule %s time limit = %d, want %d", rule.ID, rule.TimeLimitSeconds, wantSeconds)
		}
		if len(questions) != dailyQuestionCount || dailyQuestionCount != 5 {
			t.Fatalf("rule %s generated %d questions, want five", rule.ID, len(questions))
		}
		for stage, question := range questions {
			spec := dailyQuestionSpecForRule(rule, stage)
			if question.Difficulty != spec.Difficulty {
				t.Fatalf("rule %s stage %d difficulty = %q, want %q", rule.ID, stage, question.Difficulty, spec.Difficulty)
			}
			if !dailySolutionCountMatches(question.SolutionCount, spec) {
				t.Fatalf("rule %s stage %d solution count = %d, want %+v", rule.ID, stage, question.SolutionCount, spec)
			}
			if maxNumber(question.Numbers) > rule.MaxDigit {
				t.Fatalf("rule %s stage %d has digit above max %d: %#v", rule.ID, stage, rule.MaxDigit, question.Numbers)
			}
			if question.TimeLimitMS != wantSeconds*1000 {
				t.Fatalf("rule %s stage %d time limit = %d, want %d", rule.ID, stage, question.TimeLimitMS, wantSeconds*1000)
			}
		}
	}
}

func TestDailySeedIsStableWithinADateAndChangesAcrossDates(t *testing.T) {
	first, err := dailyDateSeedWithSecret("2026-08-16", "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := dailyDateSeedWithSecret("2026-08-16", "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	otherDate, err := dailyDateSeedWithSecret("2026-08-17", "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	if first != repeated || first == otherDate {
		t.Fatalf("daily seeds = %d, %d, %d; want same-day stable and next-day different", first, repeated, otherDate)
	}
	firstQuestions := generateDailyRunQuestions("2026-08-16", first, dailyRuleForIndex(int(first%dailyRuleCount)))
	repeatedQuestions := generateDailyRunQuestions("2026-08-16", repeated, dailyRuleForIndex(int(repeated%dailyRuleCount)))
	if len(firstQuestions) != len(repeatedQuestions) || firstQuestions[0].QuestionHash != repeatedQuestions[0].QuestionHash {
		t.Fatal("daily question contract is not stable for the same date and secret")
	}
}

func TestStartDailyRunDoesNotExposeAnswerSteps(t *testing.T) {
	store := &dailyTestStore{
		leaderboardStore: &leaderboardStore{},
		profile:          db.PlayerProfile{UserID: 3, ProgressJSON: DefaultProgressJSON},
	}
	runs := &dailyRunStoreFake{}
	service := NewServiceWithRoomsAndEndless(
		leaderboardProfileReader{profile: user.ProfileResponse{ID: 3}}, store, nil, runs,
	)
	result, err := service.StartDailyRun(context.Background(), 3)
	if err != nil {
		t.Fatalf("StartDailyRun() error = %v", err)
	}
	if result.RunID == "" || result.DateKey == "" || len(result.Puzzles) != dailyQuestionCount {
		t.Fatalf("result = %#v", result)
	}
	stored := runs.runs[result.RunID]
	if len(stored.Questions) != dailyQuestionCount || len(stored.Questions[0].SolutionSteps) != 3 {
		t.Fatalf("stored daily run does not contain hidden solutions: %#v", stored)
	}
}

func TestStartDailyRunReturnsCompletedForTheSameShanghaiDate(t *testing.T) {
	state := map[string]any{}
	if err := json.Unmarshal([]byte(DefaultProgressJSON), &state); err != nil {
		t.Fatal(err)
	}
	dateKey := time.Now().In(shanghaiLocation).Format("2006-01-02")
	daily := ensureObject(state, "daily")
	completed := ensureObject(daily, "completed")
	completed[dateKey] = true
	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	store := &dailyTestStore{
		leaderboardStore: &leaderboardStore{},
		profile:          db.PlayerProfile{UserID: 3, ProgressJSON: string(encoded)},
	}
	runs := &dailyRunStoreFake{}
	service := NewServiceWithRoomsAndEndless(
		leaderboardProfileReader{profile: user.ProfileResponse{ID: 3}}, store, nil, runs,
	)
	result, err := service.StartDailyRun(context.Background(), 3)
	if err != nil {
		t.Fatalf("StartDailyRun() error = %v", err)
	}
	if !result.Completed || result.Status != RunFinished || result.DateKey != dateKey || len(runs.runs) != 0 {
		t.Fatalf("result = %#v, runs = %#v; want completed without a new run", result, runs.runs)
	}
}

func TestValidateDailyRunRejectsForgedScore(t *testing.T) {
	run := testDailyRun()
	input := validDailySubmission(run)
	input.Attempts[0].Score++
	input.Summary.Score++
	if _, err := validateDailyRunSubmission(run, input); err == nil {
		t.Fatal("validateDailyRunSubmission() error = nil for forged score")
	}
}

func TestValidateDailyRunAcceptsLegacyThreeQuestionRun(t *testing.T) {
	run := testDailyRun()
	run.QuestionCount = dailyLegacyQuestionCount
	run.Questions = run.Questions[:dailyLegacyQuestionCount]
	input := validDailySubmission(run)
	if _, err := validateDailyRunSubmission(run, input); err != nil {
		t.Fatalf("validateDailyRunSubmission() rejected legacy run: %v", err)
	}
}

func TestSubmitDailyRunValidatesAndDelegatesCompletion(t *testing.T) {
	store := &dailyTestStore{
		leaderboardStore: &leaderboardStore{},
		profile:          db.PlayerProfile{UserID: 3, ProgressJSON: DefaultProgressJSON},
	}
	run := testDailyRun()
	runs := &dailyRunStoreFake{runs: map[string]DailyRun{run.RunID: run}}
	service := NewServiceWithRoomsAndEndless(
		leaderboardProfileReader{profile: user.ProfileResponse{ID: 3}}, store, nil, runs,
	)
	result, err := service.SubmitDailyRun(context.Background(), 3, run.RunID, validDailySubmission(run))
	if err != nil {
		t.Fatalf("SubmitDailyRun() error = %v", err)
	}
	if !result.Validated || result.Score <= 0 || store.dailyCalls != 1 || store.completed.IdempotencyKey != "daily_"+run.DateKey {
		t.Fatalf("result = %#v, store = %#v", result, store)
	}
}
