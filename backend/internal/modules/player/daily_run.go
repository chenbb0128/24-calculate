package player

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/example/go-service/internal/apperror"
)

const (
	dailyRunProtocolVersion  = 1
	dailyRunTTL              = 2 * time.Hour
	dailyQuestionCount       = 5
	dailyLegacyQuestionCount = 3
	dailyRuleCount           = 8
)

var ErrDailyRunNotFound = errors.New("daily run not found")

type DailyRunStore interface {
	CreateDailyRun(context.Context, DailyRun) error
	GetDailyRun(context.Context, string) (DailyRun, error)
}

func dailyRunStoreFrom(value any) DailyRunStore {
	store, _ := value.(DailyRunStore)
	return store
}

type DailyRunRule struct {
	ID                  string `json:"id"`
	Title               string `json:"title"`
	Text                string `json:"text"`
	TimeBonus           bool   `json:"time_bonus"`
	RequiredOperator    string `json:"required_operator"`
	ForbiddenOperator   string `json:"forbidden_operator"`
	MaxDigit            int    `json:"max_digit"`
	TimeLimitSeconds    int    `json:"time_limit"`
	HintCount           int    `json:"hint_count"`
	AllowHint           bool   `json:"allow_hint"`
	AllowNegativeResult bool   `json:"allow_negative_intermediate"`
}

type DailyRun struct {
	Version        int                    `json:"version"`
	RunID          string                 `json:"run_id"`
	UserID         uint64                 `json:"user_id"`
	DateKey        string                 `json:"date_key"`
	Seed           int64                  `json:"seed"`
	Rule           DailyRunRule           `json:"rule"`
	QuestionCount  int                    `json:"question_count"`
	TimeLimitMS    int                    `json:"time_limit_ms"`
	Questions      []DailyPuzzle          `json:"questions"`
	Attempts       []DailyRunAttemptInput `json:"attempts,omitempty"`
	Status         string                 `json:"status"`
	IdempotencyKey string                 `json:"idempotency_key,omitempty"`
	QuestionIndex  int                    `json:"question_index"`
	Score          int                    `json:"score"`
	ElapsedMS      int                    `json:"elapsed_ms"`
	Mistakes       int                    `json:"mistakes"`
	HintsUsed      int                    `json:"hints_used"`
	BestCombo      int                    `json:"best_combo"`
	FinishedAt     *time.Time             `json:"finished_at,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	ExpiresAt      time.Time              `json:"expires_at"`
}

type DailyPuzzle struct {
	PuzzleID      string                    `json:"puzzle_id"`
	Numbers       []int                     `json:"numbers"`
	Rules         FriendPuzzleRules         `json:"rules"`
	Difficulty    string                    `json:"difficulty"`
	QuestionHash  string                    `json:"question_hash"`
	SourceSeed    string                    `json:"source_seed"`
	SolutionCount int                       `json:"solution_count"`
	ShortestSteps int                       `json:"shortest_steps"`
	TimeLimitMS   int                       `json:"time_limit_ms"`
	SolutionSteps []FriendMatchSolutionStep `json:"solution_steps"`
}

type DailyPuzzlePublic struct {
	PuzzleID     string            `json:"puzzle_id"`
	Numbers      []int             `json:"numbers"`
	Rules        FriendPuzzleRules `json:"rules"`
	Difficulty   string            `json:"difficulty"`
	QuestionHash string            `json:"question_hash"`
	TimeLimitMS  int               `json:"time_limit_ms"`
}

type DailyRunResponse struct {
	Version             int                    `json:"version"`
	RunID               string                 `json:"run_id"`
	DateKey             string                 `json:"date_key"`
	Seed                int64                  `json:"seed"`
	Title               string                 `json:"title"`
	RuleID              string                 `json:"rule_id"`
	RuleTitle           string                 `json:"rule_title"`
	RuleText            string                 `json:"rule_text"`
	RuleIndex           int                    `json:"rule_index"`
	TimeBonus           bool                   `json:"time_bonus"`
	RequiredOperator    string                 `json:"required_operator"`
	ForbiddenOperator   string                 `json:"forbidden_operator"`
	MaxDigit            int                    `json:"max_digit"`
	AllowNegativeResult bool                   `json:"allow_negative_intermediate"`
	QuestionCount       int                    `json:"question_count"`
	ElapsedMS           int                    `json:"elapsed_ms"`
	TimeLimitSeconds    int                    `json:"time_limit"`
	TimeLimitMS         int                    `json:"time_limit_ms"`
	HintCount           int                    `json:"hint_count"`
	AllowHint           bool                   `json:"allow_hint"`
	CreatedAt           time.Time              `json:"created_at"`
	ExpiresAt           time.Time              `json:"expires_at"`
	Puzzles             []DailyPuzzlePublic    `json:"puzzles"`
	Questions           []DailyPuzzlePublic    `json:"questions"`
	Attempts            []DailyRunAttemptInput `json:"attempts"`
	Status              string                 `json:"status"`
	QuestionIndex       int                    `json:"question_index"`
	Score               int                    `json:"score"`
	Mistakes            int                    `json:"mistakes"`
	HintsUsed           int                    `json:"hints_used"`
	BestCombo           int                    `json:"best_combo"`
	FinishedAt          *time.Time             `json:"finished_at,omitempty"`
	Completed           bool                   `json:"completed,omitempty"`
	Message             string                 `json:"message,omitempty"`
}

type DailyRunAttemptInput struct {
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

type DailyRunSummaryInput struct {
	DateKey   string `json:"date_key"`
	Questions int    `json:"questions"`
	Score     int    `json:"score"`
	ElapsedMS int    `json:"elapsed_ms"`
	Mistakes  int    `json:"mistakes"`
	Hints     int    `json:"hints"`
	BestCombo int    `json:"best_combo"`
}

type DailyRunSubmissionInput struct {
	ProtocolVersion     int                    `json:"protocol_version"`
	IdempotencyKey      string                 `json:"idempotency_key"`
	RunID               string                 `json:"run_id"`
	DateKey             string                 `json:"date_key"`
	Attempts            []DailyRunAttemptInput `json:"attempts"`
	Summary             DailyRunSummaryInput   `json:"summary"`
	ClientAuthoritative bool                   `json:"client_authoritative"`
}

type DailyRunSubmissionResponse struct {
	Mode                string          `json:"mode"`
	RunID               string          `json:"run_id"`
	DateKey             string          `json:"date_key"`
	Score               int             `json:"score"`
	BestScore           int             `json:"best_score"`
	Questions           int             `json:"questions"`
	ElapsedMS           int             `json:"elapsed_ms"`
	Streak              int             `json:"streak"`
	RewardCoins         int             `json:"reward_coins"`
	Coins               int             `json:"coins"`
	Progress            json.RawMessage `json:"progress,omitempty"`
	Validated           bool            `json:"validated"`
	IdempotencyReplayed bool            `json:"idempotency_replayed"`
}

func (s *Service) StartDailyRun(ctx context.Context, userID uint64) (DailyRunResponse, error) {
	if s.dailyRuns == nil {
		return DailyRunResponse{}, apperror.ServiceUnavailable("每日挑战服务暂不可用", nil)
	}
	if _, err := s.profiles.GetProfile(ctx, userID); err != nil {
		return DailyRunResponse{}, err
	}
	dateKey := time.Now().In(shanghaiLocation).Format("2006-01-02")
	if profile, err := s.store.GetPlayerProfile(ctx, userID); err == nil {
		state := decodeProgress(profile.ProgressJSON)
		daily := ensureObject(state, "daily")
		completed := ensureObject(daily, "completed")
		rewardClaimed := ensureObject(daily, "reward_claimed")
		if readBool(completed[dateKey]) || readBool(rewardClaimed[dateKey]) {
			return DailyRunResponse{Version: dailyRunProtocolVersion, DateKey: dateKey, Status: RunFinished, Completed: true, Message: "今日挑战已完成"}, nil
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return DailyRunResponse{}, err
	}
	seed, err := dailyDateSeedWithSecret(dateKey, s.dailySeedSecret)
	if err != nil {
		return DailyRunResponse{}, apperror.ServiceUnavailable("每日挑战日期无效", err)
	}
	rule := dailyRuleForIndex(int(seed % dailyRuleCount))
	questions := generateDailyRunQuestions(dateKey, seed, rule)
	if len(questions) != dailyQuestionCount {
		return DailyRunResponse{}, apperror.ServiceUnavailable("每日挑战题目暂时生成失败", nil)
	}
	runID, err := randomDailyRunID()
	if err != nil {
		return DailyRunResponse{}, err
	}
	now := time.Now().UTC()
	run := DailyRun{
		Version:       dailyRunProtocolVersion,
		RunID:         runID,
		UserID:        userID,
		DateKey:       dateKey,
		Seed:          seed,
		Rule:          rule,
		QuestionCount: dailyQuestionCount,
		Status:        RunRunning,
		TimeLimitMS:   rule.TimeLimitSeconds * 1000,
		Questions:     questions,
		CreatedAt:     now,
		ExpiresAt:     now.Add(dailyRunTTL),
	}
	if err := s.dailyRuns.CreateDailyRun(ctx, run); err != nil {
		return DailyRunResponse{}, err
	}
	return publicDailyRun(run), nil
}

func (s *Service) SubmitDailyRun(ctx context.Context, userID uint64, runID string, input DailyRunSubmissionInput) (DailyRunSubmissionResponse, error) {
	if s.dailyRuns == nil {
		return DailyRunSubmissionResponse{}, apperror.ServiceUnavailable("每日挑战服务暂不可用", nil)
	}
	runID = strings.TrimSpace(runID)
	if runID == "" || input.RunID != runID || input.ProtocolVersion != dailyRunProtocolVersion || input.ClientAuthoritative {
		return DailyRunSubmissionResponse{}, apperror.BadRequest("每日挑战结算协议无效", nil)
	}
	run, err := s.dailyRuns.GetDailyRun(ctx, runID)
	if errors.Is(err, ErrDailyRunNotFound) {
		return DailyRunSubmissionResponse{}, apperror.NotFound("每日挑战对局不存在或已过期", err)
	}
	if err != nil {
		return DailyRunSubmissionResponse{}, err
	}
	key := strings.TrimSpace(input.IdempotencyKey)
	if len(key) < 8 || len(key) > 128 || key != "daily_"+run.DateKey || input.DateKey != run.DateKey {
		return DailyRunSubmissionResponse{}, apperror.BadRequest("每日挑战幂等键或日期无效", nil)
	}
	release, lockErr := s.acquireSettlementLock(ctx, "daily:"+runID)
	if lockErr != nil {
		return DailyRunSubmissionResponse{}, lockErr
	}
	defer release()
	today := time.Now().In(shanghaiLocation).Format("2006-01-02")
	if run.UserID != userID || run.DateKey != today || time.Now().UTC().After(run.ExpiresAt) {
		return DailyRunSubmissionResponse{}, apperror.New(10004, 403, "无权提交该每日挑战对局", nil)
	}
	if run.Status == "" {
		run.Status = RunRunning
	}
	if runStatusTerminal(run.Status) && run.IdempotencyKey != key {
		return DailyRunSubmissionResponse{}, apperror.New(10003, 409, "姣忔棩鎸戞垬宸茬粡缁撴潫", nil)
	}
	if runStatusTerminal(run.Status) && run.IdempotencyKey == key {
		result, replayErr := s.completeDaily(ctx, userID, CompleteDailyInput{IdempotencyKey: key, Score: run.Score}, CompletionMetrics{Questions: run.QuestionCount, Mistakes: run.Mistakes, Hints: run.HintsUsed, BestCombo: run.BestCombo})
		if replayErr != nil {
			return DailyRunSubmissionResponse{}, replayErr
		}
		return DailyRunSubmissionResponse{Mode: LeaderboardDaily, RunID: runID, DateKey: result.DateKey, Score: result.Score, BestScore: result.BestScore, Questions: run.QuestionCount, ElapsedMS: run.ElapsedMS, Streak: result.Streak, RewardCoins: result.RewardCoins, Coins: result.Coins, Progress: result.Progress, Validated: true, IdempotencyReplayed: true}, nil
	}
	calculated, err := validateDailyRunSubmission(run, input)
	if err != nil {
		return DailyRunSubmissionResponse{}, err
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
	run.Attempts = append([]DailyRunAttemptInput(nil), input.Attempts...)
	if stateStore, ok := s.dailyRuns.(DailyRunStateStore); ok {
		if err := stateStore.UpdateDailyRun(ctx, run); err != nil {
			return DailyRunSubmissionResponse{}, err
		}
	}
	result, err := s.completeDaily(ctx, userID, CompleteDailyInput{IdempotencyKey: key, Score: calculated.Score}, CompletionMetrics{
		Questions: calculated.Questions,
		ElapsedMS: calculated.ElapsedMS,
		FastestMS: calculated.FastestMS,
		Mistakes:  calculated.Mistakes,
		Hints:     calculated.Hints,
		BestCombo: calculated.BestCombo,
		Operators: dailyAttemptOperators(input.Attempts),
	})
	if err != nil {
		return DailyRunSubmissionResponse{}, err
	}
	return DailyRunSubmissionResponse{
		Mode:                LeaderboardDaily,
		RunID:               runID,
		DateKey:             result.DateKey,
		Score:               result.Score,
		BestScore:           result.BestScore,
		Questions:           calculated.Questions,
		ElapsedMS:           calculated.ElapsedMS,
		Streak:              result.Streak,
		RewardCoins:         result.RewardCoins,
		Coins:               result.Coins,
		Progress:            result.Progress,
		Validated:           true,
		IdempotencyReplayed: result.IdempotencyReplayed,
	}, nil
}

type dailyRunCalculation struct {
	Score     int
	Questions int
	ElapsedMS int
	Mistakes  int
	Hints     int
	BestCombo int
	FastestMS int
}

func validateDailyRunSubmission(run DailyRun, input DailyRunSubmissionInput) (dailyRunCalculation, error) {
	questionCount := run.QuestionCount
	if run.Version != dailyRunProtocolVersion || !isSupportedDailyQuestionCount(questionCount) || len(run.Questions) != questionCount {
		return dailyRunCalculation{}, apperror.BadRequest("每日挑战题目合同无效", nil)
	}
	if len(input.Attempts) != questionCount {
		return dailyRunCalculation{}, apperror.BadRequest("每日挑战必须完成全部题目", nil)
	}
	previousScore, previousCombo, previousMistakes, previousHints := 0, 0, 0, 0
	calculated := dailyRunCalculation{Questions: len(input.Attempts)}
	for index, attempt := range input.Attempts {
		question := run.Questions[index]
		if strings.TrimSpace(attempt.QuestionHash) != "" && attempt.QuestionHash != question.QuestionHash {
			return dailyRunCalculation{}, apperror.BadRequest("question_hash is invalid", nil)
		}
		if attempt.QuestionIndex != index || strings.TrimSpace(attempt.PuzzleID) != question.PuzzleID {
			return dailyRunCalculation{}, apperror.BadRequest("每日挑战题目顺序无效", nil)
		}
		if !attempt.Solved || !dailySolutionValid(question.Numbers, attempt.SolutionSteps, question.Rules) {
			return dailyRunCalculation{}, apperror.BadRequest("每日挑战答案步骤无效", nil)
		}
		if attempt.ElapsedMS < 0 || attempt.ElapsedMS > question.TimeLimitMS {
			return dailyRunCalculation{}, apperror.BadRequest("每日挑战答题时间无效", nil)
		}
		if attempt.Mistakes < previousMistakes || attempt.Mistakes > 10000 || attempt.Hints < previousHints || attempt.Hints < 0 || attempt.Hints > run.Rule.HintCount {
			return dailyRunCalculation{}, apperror.BadRequest("每日挑战答题记录无效", nil)
		}
		if attempt.Combo != previousCombo+1 || attempt.Combo < 1 || attempt.Combo > questionCount {
			return dailyRunCalculation{}, apperror.BadRequest("每日挑战连击记录无效", nil)
		}
		expectedDelta := dailyScoreDelta(question.TimeLimitMS, attempt.ElapsedMS, previousCombo, attempt.Mistakes)
		if attempt.ScoreDelta != expectedDelta || attempt.Score != previousScore+expectedDelta || attempt.Score < 0 || attempt.Score > 10000000 {
			return dailyRunCalculation{}, apperror.BadRequest("每日挑战分数计算无效", nil)
		}
		calculated.ElapsedMS += attempt.ElapsedMS
		if attempt.ElapsedMS > 0 && (calculated.FastestMS == 0 || attempt.ElapsedMS < calculated.FastestMS) {
			calculated.FastestMS = attempt.ElapsedMS
		}
		previousScore = attempt.Score
		previousCombo = attempt.Combo
		previousMistakes = attempt.Mistakes
		previousHints = attempt.Hints
		calculated.BestCombo = maxInt(calculated.BestCombo, attempt.Combo)
	}
	calculated.Score = previousScore
	calculated.Mistakes = previousMistakes
	calculated.Hints = previousHints
	if input.Summary.DateKey != run.DateKey || input.Summary.Questions != calculated.Questions || input.Summary.Score != calculated.Score || input.Summary.ElapsedMS != calculated.ElapsedMS || input.Summary.Mistakes != calculated.Mistakes || input.Summary.Hints != calculated.Hints || input.Summary.BestCombo != calculated.BestCombo {
		return dailyRunCalculation{}, apperror.BadRequest("每日挑战结算摘要与答题记录不一致", nil)
	}
	return calculated, nil
}

func isSupportedDailyQuestionCount(count int) bool {
	return count == dailyQuestionCount || count == dailyLegacyQuestionCount
}

func dailyAttemptOperators(attempts []DailyRunAttemptInput) []string {
	operators := make([]string, 0)
	for _, attempt := range attempts {
		operators = append(operators, solutionOperators(attempt.SolutionSteps)...)
	}
	return operators
}

func dailyScoreDelta(timeLimitMS, elapsedMS, previousCombo, mistakes int) int {
	remainingSeconds := float64(timeLimitMS-elapsedMS) / 1000
	delta := int(math.Round(remainingSeconds*6 + float64(previousCombo*30) - float64(mistakes*5)))
	return maxInt(10, delta)
}

func dailyRuleForIndex(index int) DailyRunRule {
	rules := []DailyRunRule{
		{ID: "no_division", Title: "今日规则：禁用除法", Text: "五题都不能使用 ÷，全部答对可领取完整奖励。", MaxDigit: 9, ForbiddenOperator: "÷"},
		{ID: "no_undo", Title: "今日规则：一步到底", Text: "今天不能撤销，每一步都要先想清楚。", MaxDigit: 9, HintCount: 2},
		{ID: "big_digits", Title: "今日规则：进阶数字", Text: "题目会出现 10～13，观察数字组合再开始。", MaxDigit: 13},
		{ID: "must_subtract", Title: "今日规则：必须减法", Text: "每题至少使用一次减法，才算完成挑战。", MaxDigit: 9, RequiredOperator: "-"},
		{ID: "must_multiply", Title: "今日规则：必须乘法", Text: "每题至少使用一次乘法，找出能快速合并的数字。", MaxDigit: 9, RequiredOperator: "×"},
		{ID: "no_multiply", Title: "今日规则：禁用乘法", Text: "今天不能使用 ×，尝试用加减除完成目标。", MaxDigit: 9, ForbiddenOperator: "×"},
		{ID: "no_add", Title: "今日规则：禁用加法", Text: "今天不能使用 +，先观察减法和除法的组合。", MaxDigit: 9, ForbiddenOperator: "+"},
		{ID: "quick_start", Title: "今日规则：快速出手", Text: "每题限时更短，连续答对可以获得额外分数。", MaxDigit: 13, TimeBonus: true},
	}
	rule := rules[((index%len(rules))+len(rules))%len(rules)]
	rule.TimeLimitSeconds = 75
	if rule.TimeBonus {
		rule.TimeLimitSeconds = 55
	}
	// Hints are a run-level budget. Keep one hint for every rule so the
	// challenge remains solvable without making the score depend on ads or
	// repeated client-side hint actions.
	rule.HintCount = 1
	rule.AllowHint = true
	rule.AllowNegativeResult = false
	return rule
}

func dailyPuzzleRules(rule DailyRunRule) FriendPuzzleRules {
	return FriendPuzzleRules{
		UseEachNumberOnce:          true,
		IntegerIntermediateResults: true,
		AllowedOperators:           []string{"+", "-", "×", "÷"},
		RequiredOperator:           rule.RequiredOperator,
		ForbiddenOperator:          rule.ForbiddenOperator,
		AllowNegativeIntermediate:  rule.AllowNegativeResult,
	}
}

func dailySolutionValid(numbers []int, steps []FriendMatchSolutionStep, rules FriendPuzzleRules) bool {
	if !replayFriendSolution(numbers, steps, rules) {
		return false
	}
	requiredFound := rules.RequiredOperator == ""
	for _, step := range steps {
		if step.Operator == rules.ForbiddenOperator && rules.ForbiddenOperator != "" {
			return false
		}
		if step.Operator == rules.RequiredOperator {
			requiredFound = true
		}
	}
	return requiredFound
}

func generateDailyRunQuestions(dateKey string, seed int64, rule DailyRunRule) []DailyPuzzle {
	random := newFriendSeededRandom(seed + int64(rule.MaxDigit)*7919)
	used := make(map[string]struct{}, dailyQuestionCount)
	result := make([]DailyPuzzle, 0, dailyQuestionCount)
	candidates := dailyCandidatePool()
	start := int(seed % int64(len(candidates)))
	rules := dailyPuzzleRules(rule)
	solutionCache := make(map[string][]friendSolution, len(candidates))
	verifiedSolutions := func(numbers []int) []friendSolution {
		key := friendNumberKey(numbers)
		if solutions, exists := solutionCache[key]; exists {
			return solutions
		}
		solutions := dailyVerifiedSolutions(numbers, rules, 500)
		solutionCache[key] = solutions
		return solutions
	}
	tryNumbers := func(numbers []int, stage int, strict bool) bool {
		if len(result) >= dailyQuestionCount {
			return true
		}
		key := friendNumberKey(numbers)
		if _, exists := used[key]; exists {
			return false
		}
		verified := verifiedSolutions(numbers)
		if len(verified) == 0 {
			return false
		}
		spec := dailyQuestionSpecForRule(rule, stage)
		if strict && !dailySolutionCountMatches(len(verified), spec) {
			return false
		}
		difficulty := spec.Difficulty
		if !strict {
			difficulty = dailyDifficultyForSolutionCount(len(verified))
		}
		used[key] = struct{}{}
		result = append(result, DailyPuzzle{
			PuzzleID: fmt.Sprintf("D%s-Q%02d", strings.ReplaceAll(dateKey, "-", ""), len(result)+1),
			Numbers:  append([]int(nil), numbers...), Rules: rules, Difficulty: difficulty,
			QuestionHash: puzzleQuestionHash(numbers, rules, seed, len(result)),
			SourceSeed:   fmt.Sprintf("%d", seed), SolutionCount: len(verified),
			ShortestSteps: shortestSolutionSteps(verified), TimeLimitMS: rule.TimeLimitSeconds * 1000,
			SolutionSteps: append([]FriendMatchSolutionStep(nil), verified[0].steps...),
		})
		return true
	}
	for stage := 0; stage < dailyQuestionCount; stage++ {
		for offset := 0; offset < len(candidates) && len(result) <= stage; offset++ {
			numbers := append([]int(nil), candidates[(start+offset)%len(candidates)]...)
			if maxNumber(numbers) <= rule.MaxDigit {
				tryNumbers(numbers, stage, true)
			}
		}
		for attempts := 0; len(result) <= stage && attempts < 5000; attempts++ {
			numbers := []int{random.int(1, rule.MaxDigit), random.int(1, rule.MaxDigit), random.int(1, rule.MaxDigit), random.int(1, rule.MaxDigit)}
			tryNumbers(numbers, stage, true)
		}
	}
	// A constrained operator rule can have fewer exact-difficulty candidates
	// than the full pool. Fill only the missing stages with already verified
	// questions, while retaining the deterministic order and uniqueness.
	for stage := len(result); stage < dailyQuestionCount; stage++ {
		for offset := 0; offset < len(candidates) && len(result) <= stage; offset++ {
			numbers := append([]int(nil), candidates[(start+offset)%len(candidates)]...)
			if maxNumber(numbers) <= rule.MaxDigit {
				tryNumbers(numbers, stage, false)
			}
		}
		for attempts := 0; len(result) <= stage && attempts < 5000; attempts++ {
			numbers := []int{random.int(1, rule.MaxDigit), random.int(1, rule.MaxDigit), random.int(1, rule.MaxDigit), random.int(1, rule.MaxDigit)}
			tryNumbers(numbers, stage, false)
		}
	}
	return result
}

type dailyQuestionSpec struct {
	Difficulty   string
	MinSolutions int
	MaxSolutions int
}

func dailyQuestionSpecFor(index int) dailyQuestionSpec {
	switch {
	case index <= 0:
		return dailyQuestionSpec{Difficulty: "warmup", MinSolutions: 8, MaxSolutions: 500}
	case index == 1:
		return dailyQuestionSpec{Difficulty: "advanced", MinSolutions: 4, MaxSolutions: 7}
	case index == 2:
		return dailyQuestionSpec{Difficulty: "challenge", MinSolutions: 2, MaxSolutions: 3}
	case index == 3:
		return dailyQuestionSpec{Difficulty: "hard", MinSolutions: 1, MaxSolutions: 2}
	default:
		return dailyQuestionSpec{Difficulty: "expert", MinSolutions: 1, MaxSolutions: 1}
	}
}

func dailyQuestionSpecForRule(rule DailyRunRule, index int) dailyQuestionSpec {
	spec := dailyQuestionSpecFor(index)
	// With multiplication forbidden, the valid integer solution space has a
	// different distribution. Reserve the narrower 4..5 solution band for the
	// final stage instead of silently falling back to an easy question.
	if rule.ID == "no_multiply" && index >= 3 {
		spec.MinSolutions = 4
		spec.MaxSolutions = 7
		if index >= 4 {
			spec.MaxSolutions = 5
		}
	}
	return spec
}

func dailySolutionCountMatches(count int, spec dailyQuestionSpec) bool {
	return count >= spec.MinSolutions && count <= spec.MaxSolutions
}

func dailyVerifiedSolutions(numbers []int, rules FriendPuzzleRules, maxSolutions int) []friendSolution {
	raw := friendSolveDetailed(numbers, maxSolutions)
	verified := make([]friendSolution, 0, len(raw))
	for _, solution := range raw {
		if dailySolutionValid(numbers, solution.steps, rules) {
			verified = append(verified, solution)
		}
	}
	return verified
}

func dailyDifficultyForSolutionCount(count int) string {
	switch {
	case count >= 8:
		return "warmup"
	case count >= 4:
		return "advanced"
	case count >= 2:
		return "challenge"
	default:
		return "hard"
	}
}

func dailyCandidatePool() [][]int {
	return append(friendCandidatePool(),
		[]int{4, 6, 10, 12}, []int{3, 7, 11, 13}, []int{2, 8, 10, 13},
		[]int{5, 6, 11, 13}, []int{1, 2, 6, 10}, []int{1, 2, 7, 12},
		[]int{1, 4, 10, 12}, []int{2, 4, 10, 13}, []int{2, 6, 10, 13},
		[]int{2, 6, 11, 12}, []int{3, 4, 6, 13},
	)
}

func maxNumber(numbers []int) int {
	result := 0
	for _, number := range numbers {
		result = maxInt(result, number)
	}
	return result
}

func publicDailyRun(run DailyRun) DailyRunResponse {
	puzzles := make([]DailyPuzzlePublic, len(run.Questions))
	for index, question := range run.Questions {
		puzzles[index] = DailyPuzzlePublic{
			PuzzleID: question.PuzzleID, Numbers: append([]int(nil), question.Numbers...),
			Rules: question.Rules, Difficulty: question.Difficulty,
			QuestionHash: question.QuestionHash, TimeLimitMS: question.TimeLimitMS,
		}
	}
	attempts := run.Attempts
	if attempts == nil {
		attempts = make([]DailyRunAttemptInput, 0)
	}
	return DailyRunResponse{
		Version: run.Version, RunID: run.RunID, DateKey: run.DateKey, Seed: run.Seed,
		Title: dailyChallengeTitle(run.QuestionCount), RuleID: run.Rule.ID, RuleTitle: run.Rule.Title,
		RuleText: run.Rule.Text, RuleIndex: int(run.Seed % dailyRuleCount), TimeBonus: run.Rule.TimeBonus,
		RequiredOperator: run.Rule.RequiredOperator, ForbiddenOperator: run.Rule.ForbiddenOperator,
		MaxDigit: run.Rule.MaxDigit, AllowNegativeResult: run.Rule.AllowNegativeResult,
		QuestionCount: run.QuestionCount, TimeLimitSeconds: run.Rule.TimeLimitSeconds,
		TimeLimitMS: run.TimeLimitMS, HintCount: run.Rule.HintCount, AllowHint: run.Rule.AllowHint,
		CreatedAt: run.CreatedAt, ExpiresAt: run.ExpiresAt, Puzzles: puzzles, Questions: puzzles, Attempts: attempts,
		Status: runStatusOrRunning(run.Status), QuestionIndex: run.QuestionIndex, Score: run.Score, ElapsedMS: run.ElapsedMS,
		Mistakes: run.Mistakes, HintsUsed: run.HintsUsed, BestCombo: run.BestCombo, FinishedAt: run.FinishedAt,
	}
}

func dailyChallengeTitle(questionCount int) string {
	if questionCount == dailyLegacyQuestionCount {
		return "每日挑战 · 三题连战"
	}
	return "每日挑战 · 五题连战"
}

func dailyDateSeed(dateKey string) (int64, error) {
	return dailyDateSeedWithSecret(dateKey, "development-daily-seed")
}

func dailyDateSeedWithSecret(dateKey, secret string) (int64, error) {
	date, err := time.ParseInLocation("2006-01-02", dateKey, shanghaiLocation)
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(secret) == "" {
		secret = "development-daily-seed"
	}
	material := fmt.Sprintf("%04d-%02d-%02d:%s", date.Year(), date.Month(), date.Day(), secret)
	digest := sha256.Sum256([]byte(material))
	seed := int64(binary.BigEndian.Uint64(digest[:8]) % 2147483647)
	if seed == 0 {
		seed = 1
	}
	return seed, nil
}

func randomDailyRunID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate daily run id: %w", err)
	}
	return fmt.Sprintf("dr_%x", bytes), nil
}
