package player

import (
	"context"
	"testing"
	"time"
)

func TestGenerateEndlessRunQuestionsProducesServerOwnedVerifiedContract(t *testing.T) {
	questions := generateEndlessRunQuestions(42, 100)
	if len(questions) != 100 {
		t.Fatalf("generated questions = %d, want 100", len(questions))
	}
	seen := map[string]bool{}
	for index, question := range questions {
		if question.PuzzleID == "" || question.PuzzleID != "ENDLESS-Q"+formatFour(index+1) {
			t.Fatalf("question %d id = %q", index, question.PuzzleID)
		}
		key := friendNumberKey(question.Numbers)
		if seen[key] {
			t.Fatalf("duplicate endless question %q", key)
		}
		seen[key] = true
		if !replayFriendSolution(question.Numbers, question.SolutionSteps, question.Rules) {
			t.Fatalf("question %d solution does not replay", index)
		}
	}
}

func TestValidateEndlessRunSubmissionRejectsClientScoreTampering(t *testing.T) {
	question := generateEndlessRunQuestions(42, 1)[0]
	run := EndlessRun{RunID: "er_test", UserID: 3, Questions: []EndlessPuzzle{question}}
	steps := question.SolutionSteps
	valid := EndlessRunSubmissionInput{
		ProtocolVersion: 1,
		IdempotencyKey:  "endless_er_test",
		RunID:           "er_test",
		Attempts: []EndlessRunAttemptInput{{
			PuzzleID: question.PuzzleID, QuestionIndex: 0, ElapsedMS: 1000, Solved: true,
			Mistakes: 0, Score: 264, ScoreDelta: 264, Combo: 1, SolutionSteps: steps,
		}},
		Summary: EndlessRunSummaryInput{Questions: 1, Score: 264, ElapsedMS: 1000, BestCombo: 1},
	}
	if err := validateEndlessRunSubmission(run, valid); err != nil {
		t.Fatalf("valid submission rejected: %v", err)
	}
	valid.Attempts[0].Score = 999999
	valid.Attempts[0].ScoreDelta = 999999
	valid.Summary.Score = 999999
	if err := validateEndlessRunSubmission(run, valid); err == nil {
		t.Fatal("tampered score accepted")
	}
}

func TestSubmitEndlessRunPersistsServerProgressAndLeaderboardScore(t *testing.T) {
	question := generateEndlessRunQuestions(42, 1)[0]
	runStore := &mutableEndlessRunStore{run: EndlessRun{RunID: "er_service", UserID: 3, ExpiresAt: time.Now().Add(time.Hour), Questions: []EndlessPuzzle{question}}}
	progress := newProgressStore(t)
	service := NewServiceWithRoomsAndEndless(leaderboardProfileReader{profile: testFriendProfile(3)}, progress, nil, runStore)
	steps := question.SolutionSteps
	input := EndlessRunSubmissionInput{
		ProtocolVersion: 1, IdempotencyKey: "endless_er_service", RunID: "er_service",
		Attempts: []EndlessRunAttemptInput{{PuzzleID: question.PuzzleID, QuestionIndex: 0, ElapsedMS: 1000, Solved: true, Score: 264, ScoreDelta: 264, Combo: 1, SolutionSteps: steps}},
		Summary:  EndlessRunSummaryInput{Questions: 1, Score: 264, ElapsedMS: 1000, BestCombo: 1},
	}
	result, err := service.SubmitEndlessRun(context.Background(), 3, "er_service", input)
	if err != nil {
		t.Fatalf("SubmitEndlessRun() error = %v", err)
	}
	if !result.Validated || result.Score != 264 || result.Coins <= 0 || len(result.Progress) == 0 {
		t.Fatalf("result = %#v", result)
	}
	if readInt(progress.state["coins"]) <= 0 {
		t.Fatalf("server progress coins = %#v, want reward", progress.state["coins"])
	}
}

type mutableEndlessRunStore struct{ run EndlessRun }

func (f *mutableEndlessRunStore) CreateEndlessRun(_ context.Context, run EndlessRun) error {
	f.run = run
	return nil
}
func (f *mutableEndlessRunStore) GetEndlessRun(_ context.Context, runID string) (EndlessRun, error) {
	if runID != f.run.RunID {
		return EndlessRun{}, ErrEndlessRunNotFound
	}
	return f.run, nil
}

func formatFour(value int) string {
	if value < 10 {
		return "000" + itoa(value)
	}
	if value < 100 {
		return "00" + itoa(value)
	}
	if value < 1000 {
		return "0" + itoa(value)
	}
	return itoa(value)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	result := ""
	for value > 0 {
		result = string(rune('0'+value%10)) + result
		value /= 10
	}
	return result
}
