package player

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/example/go-service/internal/apperror"
	"github.com/example/go-service/internal/store"
	db "github.com/example/go-service/internal/store/sqlc"
)

const (
	endlessRunProtocolVersion = 1
	endlessRunQuestionCount   = 100
	endlessRunTTL             = 2 * time.Hour
	endlessRunTimeLimit       = 60 * time.Second
	endlessRunStateVersion    = 2
)

type EndlessRunStore interface {
	CreateEndlessRun(context.Context, EndlessRun) error
	GetEndlessRun(context.Context, string) (EndlessRun, error)
}

type EndlessRunStartStore interface {
	GetEndlessRunByStartKey(context.Context, uint64, string) (EndlessRun, error)
}

var ErrEndlessRunNotFound = errors.New("endless run not found")

type EndlessRun struct {
	Version           int                                     `json:"version"`
	StateVersion      int                                     `json:"state_version"`
	RunID             string                                  `json:"run_id"`
	UserID            uint64                                  `json:"user_id"`
	RunSeed           int64                                   `json:"run_seed"`
	Questions         []EndlessPuzzle                         `json:"questions"`
	Attempts          []EndlessRunAttemptInput                `json:"attempts,omitempty"`
	StartKey          string                                  `json:"start_key,omitempty"`
	UsedFingerprints  []string                                `json:"used_fingerprints,omitempty"`
	UsedNumberKeys    []string                                `json:"used_number_keys,omitempty"`
	NextResults       map[string]EndlessNextQuestionResponse  `json:"next_results,omitempty"`
	NextRequestHashes map[string]string                       `json:"next_request_hashes,omitempty"`
	SubmitResults     map[string]EndlessRunSubmissionResponse `json:"submit_results,omitempty"`
	Status            string                                  `json:"status"`
	IdempotencyKey    string                                  `json:"idempotency_key,omitempty"`
	QuestionIndex     int                                     `json:"question_index"`
	Score             int                                     `json:"score"`
	TimeLimitMS       int                                     `json:"time_limit_ms"`
	ElapsedMS         int                                     `json:"elapsed_ms"`
	Mistakes          int                                     `json:"mistakes"`
	Combo             int                                     `json:"combo"`
	BestCombo         int                                     `json:"best_combo"`
	FinishedAt        *time.Time                              `json:"finished_at,omitempty"`
	SubmittedAt       *time.Time                              `json:"submitted_at,omitempty"`
	CreatedAt         time.Time                               `json:"created_at"`
	StartedAt         time.Time                               `json:"started_at"`
	LastActivityAt    time.Time                               `json:"last_activity_at"`
	DeadlineAt        time.Time                               `json:"deadline_at"`
	ExpiresAt         time.Time                               `json:"expires_at"`
}

// EndlessPuzzle contains the answer proof only inside the server-side run
// record. The public response deliberately omits SolutionSteps.
type EndlessPuzzle struct {
	PuzzleID      string                    `json:"puzzle_id"`
	Numbers       []int                     `json:"numbers"`
	Target        int                       `json:"target"`
	Rules         FriendPuzzleRules         `json:"rules"`
	QuestionHash  string                    `json:"question_hash"`
	SourceSeed    string                    `json:"source_seed"`
	SolutionCount int                       `json:"solution_count"`
	ShortestSteps int                       `json:"shortest_steps"`
	SolutionHash  string                    `json:"solution_hash"`
	Difficulty    string                    `json:"difficulty"`
	TimeLimitMS   int                       `json:"time_limit_ms"`
	ServedAt      time.Time                 `json:"served_at"`
	SolutionSteps []FriendMatchSolutionStep `json:"solution_steps"`
}

type EndlessPuzzlePublic struct {
	PuzzleID     string            `json:"puzzle_id"`
	Numbers      []int             `json:"numbers"`
	Target       int               `json:"target"`
	Rules        FriendPuzzleRules `json:"rules"`
	QuestionHash string            `json:"question_hash"`
	Difficulty   string            `json:"difficulty"`
	TimeLimitMS  int               `json:"time_limit_ms"`
}

type EndlessRunResponse struct {
	Version         int                      `json:"version"`
	RunID           string                   `json:"run_id"`
	RunSeed         int64                    `json:"run_seed"`
	QuestionCount   int                      `json:"question_count"`
	QuestionsSolved int                      `json:"questions_solved"`
	TimeLimitMS     int                      `json:"time_limit_ms"`
	CreatedAt       time.Time                `json:"created_at"`
	StartedAt       time.Time                `json:"started_at"`
	LastActivityAt  time.Time                `json:"last_activity_at"`
	DeadlineAt      time.Time                `json:"deadline_at"`
	ExpiresAt       time.Time                `json:"expires_at"`
	Puzzles         []EndlessPuzzlePublic    `json:"puzzles"`
	Questions       []EndlessPuzzlePublic    `json:"questions"`
	Puzzle          *EndlessPuzzlePublic     `json:"puzzle,omitempty"`
	Attempts        []EndlessRunAttemptInput `json:"attempts"`
	Status          string                   `json:"status"`
	QuestionIndex   int                      `json:"question_index"`
	Score           int                      `json:"score"`
	ElapsedMS       int                      `json:"elapsed_ms"`
	Mistakes        int                      `json:"mistakes"`
	BestCombo       int                      `json:"best_combo"`
	FinishedAt      *time.Time               `json:"finished_at,omitempty"`
}

type EndlessRunAttemptInput struct {
	PuzzleID      string                    `json:"puzzle_id"`
	QuestionHash  string                    `json:"question_hash,omitempty"`
	QuestionIndex int                       `json:"question_index"`
	ElapsedMS     int                       `json:"elapsed_ms"`
	Solved        bool                      `json:"solved"`
	Mistakes      int                       `json:"mistakes"`
	Score         int                       `json:"score"`
	ScoreDelta    int                       `json:"score_delta"`
	Combo         int                       `json:"combo"`
	SolutionSteps []FriendMatchSolutionStep `json:"solution_steps"`
}

type EndlessRunSummaryInput struct {
	Questions int `json:"questions"`
	Score     int `json:"score"`
	ElapsedMS int `json:"elapsed_ms"`
	Mistakes  int `json:"mistakes"`
	BestCombo int `json:"best_combo"`
}

type EndlessRunSubmissionInput struct {
	ProtocolVersion     int                      `json:"protocol_version"`
	IdempotencyKey      string                   `json:"idempotency_key"`
	RunID               string                   `json:"run_id"`
	Attempts            []EndlessRunAttemptInput `json:"attempts"`
	Summary             EndlessRunSummaryInput   `json:"summary"`
	ClientAuthoritative bool                     `json:"client_authoritative"`
}

type EndlessRunSubmissionResponse struct {
	Mode                string          `json:"mode"`
	RunID               string          `json:"run_id"`
	Status              string          `json:"status"`
	Score               int             `json:"score"`
	Questions           int             `json:"questions"`
	ElapsedMS           int             `json:"elapsed_ms"`
	Mistakes            int             `json:"mistakes"`
	BestCombo           int             `json:"best_combo"`
	RewardCoins         int             `json:"reward_coins"`
	Coins               int             `json:"coins"`
	Validated           bool            `json:"validated"`
	LeaderboardUpdated  bool            `json:"leaderboard_updated"`
	IdempotencyReplayed bool            `json:"idempotency_replayed"`
	Progress            json.RawMessage `json:"progress,omitempty"`
}

func (s *Service) StartEndlessRun(ctx context.Context, userID uint64) (EndlessRunResponse, error) {
	return s.StartEndlessRunWithInput(ctx, userID, EndlessRunStartInput{})
}

func (s *Service) StartEndlessRunWithInput(ctx context.Context, userID uint64, input EndlessRunStartInput) (EndlessRunResponse, error) {
	if s.endlessRuns == nil {
		return EndlessRunResponse{}, apperror.ServiceUnavailable("endless run service is unavailable", nil)
	}
	if input.ProtocolVersion != 0 && input.ProtocolVersion != endlessRunProtocolVersion {
		return EndlessRunResponse{}, apperror.BadRequest("endless start protocol is invalid", nil)
	}
	if _, err := s.profiles.GetProfile(ctx, userID); err != nil {
		return EndlessRunResponse{}, err
	}
	startKey := strings.TrimSpace(input.IdempotencyKey)
	startRelease := func() {}
	if startKey != "" {
		if len(startKey) < 8 || len(startKey) > 128 {
			return EndlessRunResponse{}, apperror.BadRequest("idempotency_key length is invalid", nil)
		}
		if release, lockErr := s.acquireSettlementLock(ctx, fmt.Sprintf("endless-start:%d:%s", userID, startKey)); lockErr != nil {
			return EndlessRunResponse{}, lockErr
		} else {
			startRelease = release
		}
		defer startRelease()
		if startStore, ok := s.endlessRuns.(EndlessRunStartStore); ok {
			existing, lookupErr := startStore.GetEndlessRunByStartKey(ctx, userID, startKey)
			if lookupErr == nil {
				return publicEndlessRun(existing), nil
			}
			if !errors.Is(lookupErr, ErrEndlessRunNotFound) {
				return EndlessRunResponse{}, lookupErr
			}
		}
	}
	seed, err := randomEndlessSeed()
	if err != nil {
		return EndlessRunResponse{}, err
	}
	runID, err := randomEndlessRunID()
	if err != nil {
		return EndlessRunResponse{}, err
	}
	now := time.Now().UTC()
	first, ok := generateEndlessPuzzle(seed, 0, nil, now)
	if !ok {
		return EndlessRunResponse{}, apperror.ServiceUnavailable("endless puzzle generation failed", nil)
	}
	run := EndlessRun{
		Version: endlessRunProtocolVersion, StateVersion: endlessRunStateVersion,
		RunID: runID, UserID: userID, RunSeed: seed,
		Questions: []EndlessPuzzle{first}, Status: RunRunning, StartKey: startKey,
		UsedFingerprints: []string{endlessQuestionFingerprint(first)},
		UsedNumberKeys:   []string{friendNumberKey(first.Numbers)},
		NextResults:      map[string]EndlessNextQuestionResponse{}, NextRequestHashes: map[string]string{},
		SubmitResults: map[string]EndlessRunSubmissionResponse{},
		TimeLimitMS:   int(endlessRunTimeLimit / time.Millisecond),
		CreatedAt:     now, StartedAt: now, LastActivityAt: now,
		DeadlineAt: now.Add(endlessRunTimeLimit), ExpiresAt: now.Add(endlessRunTTL),
	}
	if err := s.endlessRuns.CreateEndlessRun(ctx, run); err != nil {
		if startKey != "" {
			if startStore, ok := s.endlessRuns.(EndlessRunStartStore); ok {
				existing, lookupErr := startStore.GetEndlessRunByStartKey(ctx, userID, startKey)
				if lookupErr == nil {
					return publicEndlessRun(existing), nil
				}
			}
		}
		return EndlessRunResponse{}, err
	}
	return publicEndlessRun(run), nil
}

func (s *Service) startEndlessRunLegacy(ctx context.Context, userID uint64) (EndlessRunResponse, error) {
	if s.endlessRuns == nil {
		return EndlessRunResponse{}, apperror.ServiceUnavailable("无尽模式服务暂不可用", nil)
	}
	if _, err := s.profiles.GetProfile(ctx, userID); err != nil {
		return EndlessRunResponse{}, err
	}
	seed, err := randomEndlessSeed()
	if err != nil {
		return EndlessRunResponse{}, err
	}
	runID, err := randomEndlessRunID()
	if err != nil {
		return EndlessRunResponse{}, err
	}
	now := time.Now().UTC()
	questions := generateEndlessRunQuestions(seed, endlessRunQuestionCount)
	if len(questions) != endlessRunQuestionCount {
		return EndlessRunResponse{}, apperror.ServiceUnavailable("无尽模式题目暂时生成失败", nil)
	}
	run := EndlessRun{
		Version:   endlessRunProtocolVersion,
		RunID:     runID,
		UserID:    userID,
		RunSeed:   seed,
		Questions: questions,
		Status:    RunRunning,
		CreatedAt: now,
		ExpiresAt: now.Add(endlessRunTTL),
	}
	if err := s.endlessRuns.CreateEndlessRun(ctx, run); err != nil {
		return EndlessRunResponse{}, err
	}
	return publicEndlessRun(run), nil
}

func (s *Service) SubmitEndlessRun(ctx context.Context, userID uint64, runID string, input EndlessRunSubmissionInput) (EndlessRunSubmissionResponse, error) {
	if s.endlessRuns == nil {
		return EndlessRunSubmissionResponse{}, apperror.ServiceUnavailable("endless run service is unavailable", nil)
	}
	run, err := s.endlessRuns.GetEndlessRun(ctx, strings.TrimSpace(runID))
	if err == nil && (run.StateVersion >= endlessRunStateVersion || !run.StartedAt.IsZero()) {
		return s.submitEndlessRunAuthoritative(ctx, userID, runID, input)
	}
	return s.submitEndlessRunLegacy(ctx, userID, runID, input)
}

func (s *Service) submitEndlessRunLegacy(ctx context.Context, userID uint64, runID string, input EndlessRunSubmissionInput) (EndlessRunSubmissionResponse, error) {
	if s.endlessRuns == nil {
		return EndlessRunSubmissionResponse{}, apperror.ServiceUnavailable("无尽模式服务暂不可用", nil)
	}
	runID = strings.TrimSpace(runID)
	if runID == "" || input.RunID != runID || input.ProtocolVersion != endlessRunProtocolVersion || input.ClientAuthoritative {
		return EndlessRunSubmissionResponse{}, apperror.BadRequest("无尽模式结算协议无效", nil)
	}
	if len(strings.TrimSpace(input.IdempotencyKey)) < 8 || len(strings.TrimSpace(input.IdempotencyKey)) > 128 {
		return EndlessRunSubmissionResponse{}, apperror.BadRequest("idempotency_key length is invalid", nil)
	}
	if strings.TrimSpace(input.IdempotencyKey) != "endless_"+runID {
		return EndlessRunSubmissionResponse{}, apperror.BadRequest("无尽模式幂等键与对局不匹配", nil)
	}
	release, lockErr := s.acquireSettlementLock(ctx, "endless:"+runID)
	if lockErr != nil {
		return EndlessRunSubmissionResponse{}, lockErr
	}
	defer release()
	run, err := s.endlessRuns.GetEndlessRun(ctx, runID)
	if errors.Is(err, ErrEndlessRunNotFound) {
		return EndlessRunSubmissionResponse{}, apperror.NotFound("无尽模式对局不存在或已过期", err)
	}
	if err != nil {
		return EndlessRunSubmissionResponse{}, err
	}
	if run.UserID != userID || time.Now().UTC().After(run.ExpiresAt) {
		return EndlessRunSubmissionResponse{}, apperror.New(10004, 403, "无权提交该无尽模式对局", nil)
	}
	key := strings.TrimSpace(input.IdempotencyKey)
	if run.Status == "" {
		run.Status = RunRunning
	}
	if runStatusTerminal(run.Status) && run.IdempotencyKey != key {
		return EndlessRunSubmissionResponse{}, apperror.New(10003, 409, "endless run is already finished", nil)
	}
	previous, err := s.store.GetLeaderboardSubmissionByKey(ctx, leaderboardSubmissionKey(userID, LeaderboardEndless, input.IdempotencyKey))
	if err == nil {
		rewardCoins := storedMetadataInt(previous.MetadataJSON, "reward_coins")
		progress, progressErr := s.store.MutatePlayerProgress(ctx, userID, nil)
		if progressErr != nil {
			return EndlessRunSubmissionResponse{}, progressErr
		}
		return EndlessRunSubmissionResponse{Mode: LeaderboardEndless, RunID: runID, Score: int(previous.Score), Questions: int(previous.Questions), ElapsedMS: int(previous.ElapsedMs), RewardCoins: rewardCoins, Coins: progressCoins(string(progress)), Progress: progress, Validated: true, IdempotencyReplayed: true}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return EndlessRunSubmissionResponse{}, err
	}
	if err := validateEndlessRunSubmission(run, input); err != nil {
		return EndlessRunSubmissionResponse{}, err
	}

	calculated := calculateEndlessSummary(input.Attempts)
	dateKey := time.Now().In(shanghaiLocation).Format("2006-01-02")
	rewardCoins := 0
	progress, err := s.store.MutatePlayerProgress(ctx, userID, func(state map[string]any) error {
		var applyErr error
		rewardCoins, applyErr = applyEndlessServerResult(state, calculated, dateKey, input.RunID)
		return applyErr
	})
	if err != nil {
		return EndlessRunSubmissionResponse{}, err
	}
	metadata := map[string]any{
		"protocol_version": input.ProtocolVersion,
		"run_id":           runID,
		"attempt_count":    len(input.Attempts),
		"reward_coins":     rewardCoins,
	}
	encoded, _ := json.Marshal(metadata)
	if err := s.store.CreateLeaderboardSubmission(ctx, createLeaderboardSubmissionParams(userID, LeaderboardEndless, input.IdempotencyKey, calculated.Score, calculated.Questions, calculated.ElapsedMS, "", "", string(encoded))); err != nil {
		if !store.IsDuplicateEntry(err) {
			return EndlessRunSubmissionResponse{}, err
		}
		return EndlessRunSubmissionResponse{Mode: LeaderboardEndless, RunID: runID, Score: calculated.Score, Questions: calculated.Questions, ElapsedMS: calculated.ElapsedMS, Validated: true, IdempotencyReplayed: true, Progress: progress}, nil
	}
	finishedAt := time.Now().UTC()
	run.Status = RunFinished
	run.IdempotencyKey = key
	run.QuestionIndex = len(input.Attempts)
	run.Score = calculated.Score
	run.ElapsedMS = calculated.ElapsedMS
	run.Mistakes = calculated.Mistakes
	run.BestCombo = calculated.BestCombo
	run.FinishedAt = &finishedAt
	run.Attempts = append([]EndlessRunAttemptInput(nil), input.Attempts...)
	if stateStore, ok := s.endlessRuns.(EndlessRunStateStore); ok {
		if err := stateStore.UpdateEndlessRun(ctx, run); err != nil {
			return EndlessRunSubmissionResponse{}, err
		}
	}
	return EndlessRunSubmissionResponse{
		Mode: LeaderboardEndless, RunID: runID, Score: calculated.Score, Questions: calculated.Questions,
		ElapsedMS: calculated.ElapsedMS, RewardCoins: rewardCoins, Coins: progressCoins(string(progress)),
		Validated: true, Progress: progress,
	}, nil
}

func storedMetadataInt(raw, key string) int {
	values := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return 0
	}
	return readInt(values[key])
}

type endlessSummary struct {
	Score     int
	Questions int
	ElapsedMS int
	Mistakes  int
	BestCombo int
}

func validateEndlessRunSubmission(run EndlessRun, input EndlessRunSubmissionInput) error {
	if len(input.Attempts) == 0 || len(input.Attempts) > len(run.Questions) {
		return apperror.BadRequest("无尽模式答题记录数量无效", nil)
	}
	previousScore, previousCombo := 0, 0
	for index, attempt := range input.Attempts {
		puzzle := run.Questions[index]
		if strings.TrimSpace(attempt.QuestionHash) != "" && attempt.QuestionHash != puzzle.QuestionHash {
			return apperror.BadRequest("question_hash is invalid", nil)
		}
		if attempt.QuestionIndex != index || strings.TrimSpace(attempt.PuzzleID) != puzzle.PuzzleID {
			return apperror.BadRequest("无尽模式题目顺序无效", nil)
		}
		if attempt.ElapsedMS < 0 || attempt.ElapsedMS > puzzle.TimeLimitMS {
			return apperror.BadRequest("无尽模式答题时间无效", nil)
		}
		if attempt.Mistakes < 0 || attempt.Mistakes > 10000 || attempt.Score < previousScore || attempt.ScoreDelta != attempt.Score-previousScore {
			return apperror.BadRequest("无尽模式分数记录无效", nil)
		}
		if attempt.Combo < 0 || attempt.Combo > len(run.Questions) || attempt.Combo < previousCombo && attempt.Solved {
			return apperror.BadRequest("无尽模式连击记录无效", nil)
		}
		if attempt.Solved {
			if attempt.Combo != previousCombo+1 || !replayFriendSolution(puzzle.Numbers, attempt.SolutionSteps, puzzle.Rules) {
				return apperror.BadRequest("无尽模式答案步骤无效", nil)
			}
			remainingSeconds := float64(puzzle.TimeLimitMS-attempt.ElapsedMS) / 1000
			expectedDelta := int(math.Round(remainingSeconds*6 + float64(previousCombo*30) - float64(attempt.Mistakes*5)))
			if expectedDelta < 10 {
				expectedDelta = 10
			}
			if attempt.ScoreDelta != expectedDelta {
				return apperror.BadRequest("无尽模式分数计算无效", nil)
			}
		} else if attempt.Combo != 0 || attempt.ScoreDelta != 0 || len(attempt.SolutionSteps) != 0 {
			return apperror.BadRequest("无尽模式未解答记录无效", nil)
		}
		previousScore = attempt.Score
		if attempt.Solved {
			previousCombo = attempt.Combo
		}
	}
	calculated := calculateEndlessSummary(input.Attempts)
	if input.Summary.Questions != calculated.Questions || input.Summary.Score != calculated.Score || input.Summary.ElapsedMS != calculated.ElapsedMS || input.Summary.Mistakes != calculated.Mistakes || input.Summary.BestCombo != calculated.BestCombo {
		return apperror.BadRequest("无尽模式结算摘要与答题记录不一致", nil)
	}
	return nil
}

func calculateEndlessSummary(attempts []EndlessRunAttemptInput) endlessSummary {
	result := endlessSummary{}
	for _, attempt := range attempts {
		if attempt.Solved {
			result.Questions++
		}
		result.Score = attempt.Score
		result.ElapsedMS += attempt.ElapsedMS
		result.Mistakes = attempt.Mistakes
		result.BestCombo = maxInt(result.BestCombo, attempt.Combo)
	}
	return result
}

func generateEndlessRunQuestions(seed int64, count int) []EndlessPuzzle {
	random := newFriendSeededRandom(seed)
	used := make(map[string]struct{}, count)
	result := make([]EndlessPuzzle, 0, count)
	for index, attempts := 0, 0; len(result) < count && attempts < count*500; attempts++ {
		numbers := []int{random.int(1, 9), random.int(1, 9), random.int(1, 9), random.int(1, 9)}
		key := friendNumberKey(numbers)
		if _, exists := used[key]; exists {
			continue
		}
		rules := friendPuzzleRules()
		// The raw solver may find a mathematically valid expression that uses a
		// negative intermediate result. Endless mode disallows those results, so
		// select the answer proof from the same verified set that is exposed by
		// the friend-match generator.
		allSolutions := verifiedFriendSolutions(numbers, rules, 40)
		if len(allSolutions) == 0 {
			continue
		}
		used[key] = struct{}{}
		stage := index / 3
		timeLimit := maxInt(18000, 45000-stage*1000)
		result = append(result, EndlessPuzzle{
			PuzzleID:      fmt.Sprintf("ENDLESS-Q%04d", index+1),
			Numbers:       append([]int(nil), numbers...),
			Rules:         rules,
			QuestionHash:  puzzleQuestionHash(numbers, rules, seed, index),
			SourceSeed:    fmt.Sprintf("%d", seed),
			SolutionCount: len(allSolutions),
			ShortestSteps: shortestSolutionSteps(allSolutions),
			TimeLimitMS:   timeLimit,
			SolutionSteps: append([]FriendMatchSolutionStep(nil), allSolutions[0].steps...),
		})
		index++
	}
	return result
}

func publicEndlessRun(run EndlessRun) EndlessRunResponse {
	puzzles := make([]EndlessPuzzlePublic, len(run.Questions))
	for index, puzzle := range run.Questions {
		puzzles[index] = publicEndlessPuzzle(puzzle)
	}
	attempts := run.Attempts
	if attempts == nil {
		attempts = make([]EndlessRunAttemptInput, 0)
	}
	var current *EndlessPuzzlePublic
	if run.QuestionIndex >= 0 && run.QuestionIndex < len(puzzles) {
		value := puzzles[run.QuestionIndex]
		current = &value
	}
	questionsSolved := 0
	for _, attempt := range attempts {
		if attempt.Solved {
			questionsSolved++
		}
	}
	return EndlessRunResponse{
		Version: run.Version, RunID: run.RunID, RunSeed: run.RunSeed, QuestionCount: len(puzzles), QuestionsSolved: questionsSolved,
		TimeLimitMS: run.TimeLimitMS, CreatedAt: run.CreatedAt, StartedAt: run.StartedAt, LastActivityAt: run.LastActivityAt,
		DeadlineAt: run.DeadlineAt, ExpiresAt: run.ExpiresAt, Puzzles: puzzles, Questions: puzzles, Puzzle: current,
		Attempts: attempts, Status: runStatusOrRunning(run.Status), QuestionIndex: run.QuestionIndex, Score: run.Score,
		ElapsedMS: run.ElapsedMS, Mistakes: run.Mistakes, BestCombo: run.BestCombo, FinishedAt: run.FinishedAt,
	}
}

func publicEndlessPuzzle(puzzle EndlessPuzzle) EndlessPuzzlePublic {
	target := puzzle.Target
	if target == 0 {
		target = 24
	}
	return EndlessPuzzlePublic{PuzzleID: puzzle.PuzzleID, Numbers: append([]int(nil), puzzle.Numbers...), Target: target, Rules: puzzle.Rules, QuestionHash: puzzle.QuestionHash, Difficulty: puzzle.Difficulty, TimeLimitMS: puzzle.TimeLimitMS}
}

func randomEndlessSeed() (int64, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(2147483647))
	if err != nil {
		return 0, fmt.Errorf("generate endless seed: %w", err)
	}
	return value.Int64(), nil
}

func randomEndlessRunID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate endless run id: %w", err)
	}
	return fmt.Sprintf("er_%x", bytes), nil
}

func leaderboardSubmissionKey(userID uint64, mode, key string) db.GetLeaderboardSubmissionByKeyParams {
	return db.GetLeaderboardSubmissionByKeyParams{UserID: userID, Mode: mode, IdempotencyKey: key}
}

func createLeaderboardSubmissionParams(userID uint64, mode, key string, score, questions, elapsed int, roomID, outcome, metadata string) db.CreateLeaderboardSubmissionParams {
	return db.CreateLeaderboardSubmissionParams{UserID: userID, Mode: mode, IdempotencyKey: key, Score: uint32(score), Questions: uint32(questions), ElapsedMs: uint32(elapsed), RoomID: roomID, Outcome: outcome, MetadataJSON: metadata, CreatedAt: time.Now().UTC()}
}
