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
)

const (
	campaignRunProtocolVersion = 1
	campaignRunTTL             = 2 * time.Hour
	campaignLevelCount         = 200
)

var ErrCampaignRunNotFound = errors.New("campaign run not found")

// CampaignRunStore keeps the server-owned question contract. The concrete
// implementation is Redis in production, while tests can use an in-memory
// implementation.
type CampaignRunStore interface {
	CreateCampaignRun(context.Context, CampaignRun) error
	GetCampaignRun(context.Context, string) (CampaignRun, error)
}

func campaignRunStoreFrom(value any) CampaignRunStore {
	store, _ := value.(CampaignRunStore)
	return store
}

type CampaignRun struct {
	Version        int                       `json:"version"`
	RunID          string                    `json:"run_id"`
	UserID         uint64                    `json:"user_id"`
	LevelID        int                       `json:"level_id"`
	QuestionCount  int                       `json:"question_count"`
	TimeLimitMS    int                       `json:"time_limit_ms"`
	AllowHint      bool                      `json:"allow_hint"`
	Questions      []CampaignPuzzle          `json:"questions"`
	Attempts       []CampaignRunAttemptInput `json:"attempts,omitempty"`
	Status         string                    `json:"status"`
	IdempotencyKey string                    `json:"idempotency_key,omitempty"`
	QuestionIndex  int                       `json:"question_index"`
	Score          int                       `json:"score"`
	ElapsedMS      int                       `json:"elapsed_ms"`
	Mistakes       int                       `json:"mistakes"`
	HintsUsed      int                       `json:"hints_used"`
	BestCombo      int                       `json:"best_combo"`
	FinishedAt     *time.Time                `json:"finished_at,omitempty"`
	CreatedAt      time.Time                 `json:"created_at"`
	ExpiresAt      time.Time                 `json:"expires_at"`
}

type CampaignPuzzle struct {
	PuzzleID      string                    `json:"puzzle_id"`
	Numbers       []int                     `json:"numbers"`
	Rules         FriendPuzzleRules         `json:"rules"`
	QuestionHash  string                    `json:"question_hash"`
	SourceSeed    string                    `json:"source_seed"`
	SolutionCount int                       `json:"solution_count"`
	ShortestSteps int                       `json:"shortest_steps"`
	TimeLimitMS   int                       `json:"time_limit_ms"`
	SolutionSteps []FriendMatchSolutionStep `json:"solution_steps"`
}

type CampaignPuzzlePublic struct {
	PuzzleID     string            `json:"puzzle_id"`
	Numbers      []int             `json:"numbers"`
	Rules        FriendPuzzleRules `json:"rules"`
	QuestionHash string            `json:"question_hash"`
	TimeLimitMS  int               `json:"time_limit_ms"`
}

type CampaignRunResponse struct {
	Version       int                       `json:"version"`
	RunID         string                    `json:"run_id"`
	LevelID       int                       `json:"level_id"`
	QuestionCount int                       `json:"question_count"`
	TimeLimitMS   int                       `json:"time_limit_ms"`
	AllowHint     bool                      `json:"allow_hint"`
	CreatedAt     time.Time                 `json:"created_at"`
	ExpiresAt     time.Time                 `json:"expires_at"`
	Puzzles       []CampaignPuzzlePublic    `json:"puzzles"`
	Questions     []CampaignPuzzlePublic    `json:"questions"`
	Attempts      []CampaignRunAttemptInput `json:"attempts"`
	Status        string                    `json:"status"`
	QuestionIndex int                       `json:"question_index"`
	Score         int                       `json:"score"`
	ElapsedMS     int                       `json:"elapsed_ms"`
	Mistakes      int                       `json:"mistakes"`
	HintsUsed     int                       `json:"hints_used"`
	BestCombo     int                       `json:"best_combo"`
	FinishedAt    *time.Time                `json:"finished_at,omitempty"`
}

type CampaignRunStartInput struct {
	LevelID int `json:"level_id"`
}

type CampaignRunAttemptInput struct {
	PuzzleID      string                    `json:"puzzle_id"`
	QuestionHash  string                    `json:"question_hash,omitempty"`
	QuestionIndex int                       `json:"question_index"`
	ElapsedMS     int                       `json:"elapsed_ms"`
	Solved        bool                      `json:"solved"`
	Mistakes      int                       `json:"mistakes"`
	Hints         int                       `json:"hints"`
	Score         int                       `json:"score"`
	ScoreDelta    int                       `json:"score_delta"`
	Combo         int                       `json:"combo"`
	SolutionSteps []FriendMatchSolutionStep `json:"solution_steps"`
}

type CampaignRunSummaryInput struct {
	Questions int `json:"questions"`
	Score     int `json:"score"`
	ElapsedMS int `json:"elapsed_ms"`
	Mistakes  int `json:"mistakes"`
	Hints     int `json:"hints"`
	Stars     int `json:"stars"`
	BestCombo int `json:"best_combo"`
}

type CampaignRunSubmissionInput struct {
	ProtocolVersion     int                       `json:"protocol_version"`
	IdempotencyKey      string                    `json:"idempotency_key"`
	RunID               string                    `json:"run_id"`
	LevelID             int                       `json:"level_id"`
	Attempts            []CampaignRunAttemptInput `json:"attempts"`
	Summary             CampaignRunSummaryInput   `json:"summary"`
	ClientAuthoritative bool                      `json:"client_authoritative"`
}

type CampaignRunSubmissionResponse struct {
	Mode                string          `json:"mode"`
	RunID               string          `json:"run_id"`
	LevelID             int             `json:"level_id"`
	Score               int             `json:"score"`
	Stars               int             `json:"stars"`
	BestScore           int             `json:"best_score"`
	Questions           int             `json:"questions"`
	ElapsedMS           int             `json:"elapsed_ms"`
	RewardCoins         int             `json:"reward_coins"`
	Coins               int             `json:"coins"`
	UnlockedLevel       int             `json:"unlocked_level"`
	Progress            json.RawMessage `json:"progress,omitempty"`
	Validated           bool            `json:"validated"`
	IdempotencyReplayed bool            `json:"idempotency_replayed"`
}

func (s *Service) StartCampaignRun(ctx context.Context, userID uint64, levelID int) (CampaignRunResponse, error) {
	if s.campaignRuns == nil {
		return CampaignRunResponse{}, apperror.ServiceUnavailable("闯关服务暂不可用", nil)
	}
	if levelID < 0 || levelID >= campaignLevelCount {
		return CampaignRunResponse{}, apperror.BadRequest("level_id is out of range", nil)
	}
	if _, err := s.profiles.GetProfile(ctx, userID); err != nil {
		return CampaignRunResponse{}, err
	}
	if err := s.ensureCampaignLevelUnlocked(ctx, userID, levelID); err != nil {
		return CampaignRunResponse{}, err
	}

	questionCount, timeLimitMS, allowHint := campaignLevelConfig(levelID)
	seed, err := randomCampaignSeed()
	if err != nil {
		return CampaignRunResponse{}, err
	}
	runID, err := randomCampaignRunID()
	if err != nil {
		return CampaignRunResponse{}, err
	}
	questions := generateCampaignRunQuestions(levelID, seed, questionCount, timeLimitMS)
	if len(questions) != questionCount {
		return CampaignRunResponse{}, apperror.ServiceUnavailable("闯关题目暂时生成失败", nil)
	}
	now := time.Now().UTC()
	run := CampaignRun{
		Version:       campaignRunProtocolVersion,
		RunID:         runID,
		UserID:        userID,
		LevelID:       levelID,
		QuestionCount: questionCount,
		TimeLimitMS:   timeLimitMS,
		AllowHint:     allowHint,
		Status:        RunRunning,
		Questions:     questions,
		CreatedAt:     now,
		ExpiresAt:     now.Add(campaignRunTTL),
	}
	if err := s.campaignRuns.CreateCampaignRun(ctx, run); err != nil {
		return CampaignRunResponse{}, err
	}
	return publicCampaignRun(run), nil
}

func (s *Service) SubmitCampaignRun(ctx context.Context, userID uint64, runID string, input CampaignRunSubmissionInput) (CampaignRunSubmissionResponse, error) {
	if s.campaignRuns == nil {
		return CampaignRunSubmissionResponse{}, apperror.ServiceUnavailable("闯关服务暂不可用", nil)
	}
	runID = strings.TrimSpace(runID)
	if runID == "" || input.RunID != runID || input.ProtocolVersion != campaignRunProtocolVersion || input.ClientAuthoritative {
		return CampaignRunSubmissionResponse{}, apperror.BadRequest("闯关结算协议无效", nil)
	}
	key := strings.TrimSpace(input.IdempotencyKey)
	if len(key) < 8 || len(key) > 128 || key != "campaign_"+runID {
		return CampaignRunSubmissionResponse{}, apperror.BadRequest("闯关幂等键与对局不匹配", nil)
	}
	release, lockErr := s.acquireSettlementLock(ctx, "campaign:"+runID)
	if lockErr != nil {
		return CampaignRunSubmissionResponse{}, lockErr
	}
	defer release()
	run, err := s.campaignRuns.GetCampaignRun(ctx, runID)
	if errors.Is(err, ErrCampaignRunNotFound) {
		return CampaignRunSubmissionResponse{}, apperror.NotFound("闯关对局不存在或已过期", err)
	}
	if err != nil {
		return CampaignRunSubmissionResponse{}, err
	}
	if run.UserID != userID || time.Now().UTC().After(run.ExpiresAt) {
		return CampaignRunSubmissionResponse{}, apperror.New(10004, 403, "无权提交该闯关对局", nil)
	}
	if run.Status == "" {
		run.Status = RunRunning
	}
	if runStatusTerminal(run.Status) && run.IdempotencyKey != key {
		return CampaignRunSubmissionResponse{}, apperror.New(10003, 409, "闂叧娓告垙宸茬粡缁撴潫", nil)
	}
	if runStatusTerminal(run.Status) && run.IdempotencyKey == key {
		result, replayErr := s.completeLevel(ctx, userID, run.LevelID, CompleteLevelInput{IdempotencyKey: key, Score: run.Score, Stars: campaignStars(run.Score)}, CompletionMetrics{Questions: run.QuestionCount, ElapsedMS: run.ElapsedMS, Mistakes: run.Mistakes, Hints: run.HintsUsed, BestCombo: run.BestCombo})
		if replayErr != nil {
			return CampaignRunSubmissionResponse{}, replayErr
		}
		return CampaignRunSubmissionResponse{Mode: LeaderboardCampaign, RunID: runID, LevelID: result.LevelID, Score: result.BestScore, Stars: result.Stars, BestScore: result.BestScore, Questions: run.QuestionCount, ElapsedMS: run.ElapsedMS, RewardCoins: result.RewardCoins, Coins: result.Coins, UnlockedLevel: result.UnlockedLevel, Progress: result.Progress, Validated: true, IdempotencyReplayed: true}, nil
	}
	if input.LevelID != run.LevelID {
		return CampaignRunSubmissionResponse{}, apperror.BadRequest("闯关关卡合同无效", nil)
	}
	calculated, err := validateCampaignRunSubmission(run, input)
	if err != nil {
		return CampaignRunSubmissionResponse{}, err
	}
	finishedAt := time.Now().UTC()
	run.Status = RunFinished
	run.IdempotencyKey = key
	run.QuestionIndex = run.QuestionCount
	run.Score = calculated.Score
	run.ElapsedMS = calculated.ElapsedMS
	run.Mistakes = calculated.Mistakes
	run.HintsUsed = calculated.Hints
	run.BestCombo = calculated.BestCombo
	run.FinishedAt = &finishedAt
	run.Attempts = append([]CampaignRunAttemptInput(nil), input.Attempts...)
	if stateStore, ok := s.campaignRuns.(CampaignRunStateStore); ok {
		if err := stateStore.UpdateCampaignRun(ctx, run); err != nil {
			return CampaignRunSubmissionResponse{}, err
		}
	}

	result, err := s.completeLevel(ctx, userID, run.LevelID, CompleteLevelInput{
		IdempotencyKey: key,
		Score:          calculated.Score,
		Stars:          calculated.Stars,
	}, CompletionMetrics{
		Questions: calculated.Questions,
		ElapsedMS: calculated.ElapsedMS,
		FastestMS: calculated.FastestMS,
		Mistakes:  calculated.Mistakes,
		Hints:     calculated.Hints,
		BestCombo: calculated.BestCombo,
		Operators: campaignAttemptOperators(input.Attempts),
	})
	if err != nil {
		return CampaignRunSubmissionResponse{}, err
	}
	return CampaignRunSubmissionResponse{
		Mode:                LeaderboardCampaign,
		RunID:               runID,
		LevelID:             result.LevelID,
		Score:               calculated.Score,
		Stars:               result.Stars,
		BestScore:           result.BestScore,
		Questions:           calculated.Questions,
		ElapsedMS:           calculated.ElapsedMS,
		RewardCoins:         result.RewardCoins,
		Coins:               result.Coins,
		UnlockedLevel:       result.UnlockedLevel,
		Progress:            result.Progress,
		Validated:           true,
		IdempotencyReplayed: result.IdempotencyReplayed,
	}, nil
}

type campaignRunCalculation struct {
	Score     int
	Stars     int
	Questions int
	ElapsedMS int
	Mistakes  int
	Hints     int
	BestCombo int
	FastestMS int
}

func validateCampaignRunSubmission(run CampaignRun, input CampaignRunSubmissionInput) (campaignRunCalculation, error) {
	if run.Version != campaignRunProtocolVersion || run.QuestionCount != len(run.Questions) || run.QuestionCount <= 0 {
		return campaignRunCalculation{}, apperror.BadRequest("闯关题目合同无效", nil)
	}
	if len(input.Attempts) != len(run.Questions) {
		return campaignRunCalculation{}, apperror.BadRequest("闯关必须完成全部题目", nil)
	}
	previousScore := 0
	previousMistakes := 0
	previousHints := 0
	calculated := campaignRunCalculation{Questions: len(input.Attempts)}
	for index, attempt := range input.Attempts {
		question := run.Questions[index]
		if strings.TrimSpace(attempt.QuestionHash) != "" && attempt.QuestionHash != question.QuestionHash {
			return campaignRunCalculation{}, apperror.BadRequest("question_hash is invalid", nil)
		}
		if attempt.QuestionIndex != index || strings.TrimSpace(attempt.PuzzleID) != question.PuzzleID {
			return campaignRunCalculation{}, apperror.BadRequest("闯关题目顺序无效", nil)
		}
		if !attempt.Solved || len(attempt.SolutionSteps) == 0 || !replayFriendSolution(question.Numbers, attempt.SolutionSteps, question.Rules) {
			return campaignRunCalculation{}, apperror.BadRequest("闯关答案步骤无效", nil)
		}
		if attempt.ElapsedMS < 0 || attempt.ElapsedMS > question.TimeLimitMS {
			return campaignRunCalculation{}, apperror.BadRequest("闯关答题时间无效", nil)
		}
		if attempt.Mistakes < previousMistakes || attempt.Mistakes > 10000 || attempt.Hints < previousHints || attempt.Hints < 0 || attempt.Hints > 1 {
			return campaignRunCalculation{}, apperror.BadRequest("闯关答题记录无效", nil)
		}
		if !run.AllowHint && attempt.Hints > 0 {
			return campaignRunCalculation{}, apperror.BadRequest("当前关卡不允许使用提示", nil)
		}
		if attempt.ScoreDelta != attempt.Score-previousScore {
			return campaignRunCalculation{}, apperror.BadRequest("闯关分数增量无效", nil)
		}
		if attempt.Score < 0 || attempt.Score > 100 {
			return campaignRunCalculation{}, apperror.BadRequest("闯关分数超出范围", nil)
		}
		if attempt.Combo < 0 || attempt.Combo > index+1 {
			return campaignRunCalculation{}, apperror.BadRequest("campaign combo record is invalid", nil)
		}
		expectedScore := campaignProgressScore(index+1, len(run.Questions), attempt.Mistakes, attempt.Hints)
		if attempt.Score != expectedScore {
			return campaignRunCalculation{}, apperror.BadRequest("闯关分数计算无效", nil)
		}
		calculated.ElapsedMS += attempt.ElapsedMS
		if attempt.ElapsedMS > 0 && (calculated.FastestMS == 0 || attempt.ElapsedMS < calculated.FastestMS) {
			calculated.FastestMS = attempt.ElapsedMS
		}
		calculated.BestCombo = maxInt(calculated.BestCombo, attempt.Combo)
		previousScore = attempt.Score
		previousMistakes = attempt.Mistakes
		previousHints = attempt.Hints
	}
	calculated.Score = previousScore
	calculated.Mistakes = previousMistakes
	calculated.Hints = previousHints
	calculated.Stars = campaignStars(calculated.Score)
	if input.Summary.Questions != calculated.Questions || input.Summary.Score != calculated.Score || input.Summary.ElapsedMS != calculated.ElapsedMS || input.Summary.Mistakes != calculated.Mistakes || input.Summary.Hints != calculated.Hints || input.Summary.Stars != calculated.Stars || (input.Summary.BestCombo > 0 && input.Summary.BestCombo != calculated.BestCombo) {
		return campaignRunCalculation{}, apperror.BadRequest("闯关结算摘要与答题记录不一致", nil)
	}
	return calculated, nil
}

func campaignProgressScore(solvedQuestions, totalQuestions, mistakes, hints int) int {
	if totalQuestions <= 0 {
		return 0
	}
	base := 100 * float64(solvedQuestions) / float64(totalQuestions)
	penalty := float64(maxInt(0, mistakes) * 20)
	if hints > 0 {
		penalty += 10
	}
	return maxInt(0, minInt(100, int(math.Round(base-penalty))))
}

func campaignStars(score int) int {
	score = maxInt(0, minInt(100, score))
	switch {
	case score >= 100:
		return 3
	case score >= 80:
		return 2
	default:
		// Completing every question is itself a one-star clear. This matches
		// the existing client flow, which clamps a completed level to at least
		// one star when it sends the legacy completion request.
		return 1
	}
}

func campaignLevelConfig(levelID int) (questionCount, timeLimitMS int, allowHint bool) {
	chapterIndex := levelID / 20
	chapterLevel := levelID % 20
	challenge := levelID%5 == 4
	questionCount = 3
	if challenge {
		questionCount = 5
	}
	seconds := math.Max(42, float64(68)-float64(chapterLevel)*1.25-float64(chapterIndex)*2.5)
	if challenge {
		seconds = math.Max(42, float64(95)-float64(chapterLevel)*1.25-float64(chapterIndex)*2.5)
	}
	return questionCount, int(math.Round(seconds * 1000)), chapterLevel < 16
}

func generateCampaignRunQuestions(levelID int, seed int64, count, timeLimitMS int) []CampaignPuzzle {
	random := newFriendSeededRandom(seed + int64(levelID+1)*7919)
	used := make(map[string]struct{}, count)
	result := make([]CampaignPuzzle, 0, count)
	for attempts := 0; len(result) < count && attempts < count*1000; attempts++ {
		numbers := []int{random.int(1, 13), random.int(1, 13), random.int(1, 13), random.int(1, 13)}
		key := friendNumberKey(numbers)
		if _, exists := used[key]; exists {
			continue
		}
		solutions := friendSolveDetailed(numbers, 1)
		allSolutions := friendSolveDetailed(numbers, 40)
		if len(solutions) == 0 {
			continue
		}
		used[key] = struct{}{}
		rules := friendPuzzleRules()
		result = append(result, CampaignPuzzle{
			PuzzleID:      fmt.Sprintf("C%03d-Q%02d", levelID+1, len(result)+1),
			Numbers:       append([]int(nil), numbers...),
			Rules:         rules,
			QuestionHash:  puzzleQuestionHash(numbers, rules, seed, len(result)),
			SourceSeed:    fmt.Sprintf("%d", seed),
			SolutionCount: len(allSolutions),
			ShortestSteps: shortestSolutionSteps(allSolutions),
			TimeLimitMS:   timeLimitMS,
			SolutionSteps: append([]FriendMatchSolutionStep(nil), solutions[0].steps...),
		})
	}
	return result
}

func publicCampaignRun(run CampaignRun) CampaignRunResponse {
	puzzles := make([]CampaignPuzzlePublic, len(run.Questions))
	for index, question := range run.Questions {
		puzzles[index] = CampaignPuzzlePublic{
			PuzzleID:     question.PuzzleID,
			Numbers:      append([]int(nil), question.Numbers...),
			Rules:        question.Rules,
			QuestionHash: question.QuestionHash,
			TimeLimitMS:  question.TimeLimitMS,
		}
	}
	attempts := run.Attempts
	if attempts == nil {
		attempts = make([]CampaignRunAttemptInput, 0)
	}
	return CampaignRunResponse{
		Version: run.Version, RunID: run.RunID, LevelID: run.LevelID,
		QuestionCount: run.QuestionCount, TimeLimitMS: run.TimeLimitMS,
		AllowHint: run.AllowHint, CreatedAt: run.CreatedAt, ExpiresAt: run.ExpiresAt,
		Puzzles: puzzles, Questions: puzzles, Attempts: attempts, Status: runStatusOrRunning(run.Status), QuestionIndex: run.QuestionIndex,
		Score: run.Score, ElapsedMS: run.ElapsedMS, Mistakes: run.Mistakes, HintsUsed: run.HintsUsed, BestCombo: run.BestCombo,
		FinishedAt: run.FinishedAt,
	}
}

func runStatusOrRunning(status string) string {
	if status == "" {
		return RunRunning
	}
	return status
}

func (s *Service) ensureCampaignLevelUnlocked(ctx context.Context, userID uint64, levelID int) error {
	profile, err := s.store.GetPlayerProfile(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		if levelID == 0 {
			return nil
		}
		return apperror.New(10008, 403, "当前关卡尚未解锁", nil)
	}
	if err != nil {
		return err
	}
	state := decodeProgress(profile.ProgressJSON)
	if levelID > readInt(state["unlocked_level"]) {
		return apperror.New(10008, 403, "当前关卡尚未解锁", nil)
	}
	return nil
}

func randomCampaignSeed() (int64, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(2147483647))
	if err != nil {
		return 0, fmt.Errorf("generate campaign seed: %w", err)
	}
	return value.Int64(), nil
}

func randomCampaignRunID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate campaign run id: %w", err)
	}
	return fmt.Sprintf("cr_%x", bytes), nil
}
