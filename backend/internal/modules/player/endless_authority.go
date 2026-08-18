package player

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/example/go-service/internal/apperror"
	"github.com/example/go-service/internal/store"
)

type EndlessRunStartInput struct {
	ProtocolVersion int    `json:"protocol_version"`
	IdempotencyKey  string `json:"idempotency_key"`
}

type EndlessNextQuestionInput struct {
	ProtocolVersion int                       `json:"protocol_version"`
	IdempotencyKey  string                    `json:"idempotency_key"`
	QuestionIndex   int                       `json:"question_index"`
	PuzzleID        string                    `json:"puzzle_id"`
	ElapsedMS       int                       `json:"elapsed_ms"`
	Solved          bool                      `json:"solved"`
	Mistakes        int                       `json:"mistakes"`
	ScoreDelta      int                       `json:"score_delta"`
	Combo           int                       `json:"combo"`
	SolutionSteps   []FriendMatchSolutionStep `json:"solution_steps"`
}

type EndlessNextQuestionResponse struct {
	RunID               string               `json:"run_id"`
	Status              string               `json:"status"`
	Accepted            bool                 `json:"accepted"`
	Validated           bool                 `json:"validated"`
	IdempotencyReplayed bool                 `json:"idempotency_replayed"`
	QuestionIndex       int                  `json:"question_index"`
	Score               int                  `json:"score"`
	Mistakes            int                  `json:"mistakes"`
	BestCombo           int                  `json:"best_combo"`
	QuestionsSolved     int                  `json:"questions_solved"`
	ElapsedMS           int                  `json:"elapsed_ms"`
	ScoreDelta          int                  `json:"score_delta"`
	NextPuzzle          *EndlessPuzzlePublic `json:"next_puzzle,omitempty"`
	RewardCoins         int                  `json:"reward_coins"`
}

func (s *Service) NextEndlessQuestion(ctx context.Context, userID uint64, runID string, input EndlessNextQuestionInput) (EndlessNextQuestionResponse, error) {
	if s.endlessRuns == nil {
		return EndlessNextQuestionResponse{}, apperror.ServiceUnavailable("endless run service is unavailable", nil)
	}
	runID = strings.TrimSpace(runID)
	if runID == "" || input.ProtocolVersion != endlessRunProtocolVersion {
		return EndlessNextQuestionResponse{}, apperror.BadRequest("endless next-question protocol is invalid", nil)
	}
	key := strings.TrimSpace(input.IdempotencyKey)
	if len(key) < 8 || len(key) > 128 {
		return EndlessNextQuestionResponse{}, apperror.BadRequest("idempotency_key length is invalid", nil)
	}
	release, err := s.acquireSettlementLock(ctx, "endless:"+runID)
	if err != nil {
		return EndlessNextQuestionResponse{}, err
	}
	defer release()

	run, err := s.endlessRuns.GetEndlessRun(ctx, runID)
	if errors.Is(err, ErrEndlessRunNotFound) {
		return EndlessNextQuestionResponse{}, apperror.NotFound("endless run was not found", err)
	}
	if err != nil {
		return EndlessNextQuestionResponse{}, err
	}
	if run.UserID != userID {
		return EndlessNextQuestionResponse{}, apperror.New(10004, 403, "you cannot access this run", nil)
	}
	now := time.Now().UTC()
	if run.Status == RunExpired || (!run.ExpiresAt.IsZero() && now.After(run.ExpiresAt)) {
		run.Status = RunExpired
		_ = persistEndlessRun(ctx, s.endlessRuns, run)
		return EndlessNextQuestionResponse{}, apperror.New(10005, 410, "endless run has expired", nil)
	}
	if run.NextResults == nil {
		run.NextResults = map[string]EndlessNextQuestionResponse{}
	}
	if run.NextRequestHashes == nil {
		run.NextRequestHashes = map[string]string{}
	}
	requestHash := hashEndlessNextRequest(input)
	if previous, exists := run.NextResults[key]; exists {
		if run.NextRequestHashes[key] != requestHash {
			return EndlessNextQuestionResponse{}, apperror.Conflict("idempotency_key was already used with another request", nil)
		}
		previous.IdempotencyReplayed = true
		return previous, nil
	}

	if run.Status == RunSubmitted || run.Status == RunCancelled {
		return EndlessNextQuestionResponse{}, apperror.Conflict("endless run is no longer active", nil)
	}
	if run.Status == RunFinished {
		result := endlessNextResult(run, false, false, 0, nil)
		run.NextResults[key] = result
		run.NextRequestHashes[key] = requestHash
		if err := persistEndlessRun(ctx, s.endlessRuns, run); err != nil {
			return EndlessNextQuestionResponse{}, err
		}
		return result, nil
	}

	if run.QuestionIndex < 0 || run.QuestionIndex >= len(run.Questions) {
		return EndlessNextQuestionResponse{}, apperror.Conflict("current endless puzzle is unavailable", nil)
	}
	current := run.Questions[run.QuestionIndex]
	if input.QuestionIndex != run.QuestionIndex || strings.TrimSpace(input.PuzzleID) != current.PuzzleID {
		return EndlessNextQuestionResponse{}, apperror.BadRequest("puzzle_id or question_index does not match the server state", nil)
	}
	if input.ElapsedMS < 0 {
		return EndlessNextQuestionResponse{}, apperror.BadRequest("elapsed_ms must not be negative", nil)
	}
	if input.Mistakes < 0 || input.ScoreDelta < 0 || input.Combo < 0 {
		return EndlessNextQuestionResponse{}, apperror.BadRequest("client metrics are invalid", nil)
	}

	if !run.DeadlineAt.IsZero() && !now.Before(run.DeadlineAt) {
		serverElapsed := endlessPuzzleElapsed(current, now, run.StartedAt)
		run.Mistakes++
		run.Combo = 0
		run.Attempts = append(run.Attempts, EndlessRunAttemptInput{
			PuzzleID: current.PuzzleID, QuestionHash: current.QuestionHash, QuestionIndex: run.QuestionIndex,
			ElapsedMS: serverElapsed, Solved: false, Mistakes: run.Mistakes, Score: run.Score,
			ScoreDelta: 0, Combo: 0, SolutionSteps: []FriendMatchSolutionStep{},
		})
		finishEndlessRun(&run, now)
		result := endlessNextResult(run, false, true, 0, nil)
		run.NextResults[key] = result
		run.NextRequestHashes[key] = requestHash
		if err := persistEndlessRun(ctx, s.endlessRuns, run); err != nil {
			return EndlessNextQuestionResponse{}, err
		}
		return result, nil
	}

	serverElapsed := endlessPuzzleElapsed(current, now, run.StartedAt)
	if input.Solved {
		if err := validateEndlessSolution(current, input.SolutionSteps); err != nil {
			return EndlessNextQuestionResponse{}, err
		}
		combo := run.Combo + 1
		scoreDelta := calculateEndlessScoreDelta(current, serverElapsed, combo, run.QuestionIndex)
		run.Score += scoreDelta
		run.Combo = combo
		run.BestCombo = maxInt(run.BestCombo, combo)
		attempt := EndlessRunAttemptInput{
			PuzzleID: current.PuzzleID, QuestionHash: current.QuestionHash, QuestionIndex: run.QuestionIndex,
			ElapsedMS: serverElapsed, Solved: true, Mistakes: run.Mistakes,
			Score: run.Score, ScoreDelta: scoreDelta, Combo: combo,
			SolutionSteps: cloneSolutionSteps(input.SolutionSteps),
		}
		run.Attempts = append(run.Attempts, attempt)
		run.QuestionIndex++
		if run.QuestionIndex < 1 {
			run.QuestionIndex = 1
		}
		if !run.DeadlineAt.IsZero() && now.Before(run.DeadlineAt) {
			used := endlessRunUsedSet(run)
			next, ok := generateEndlessPuzzle(run.RunSeed, run.QuestionIndex, used, now)
			if !ok {
				used = recentEndlessUsedSet(run)
				next, ok = generateEndlessPuzzle(run.RunSeed, run.QuestionIndex, used, now)
			}
			if !ok {
				next, ok = generateEndlessPuzzle(run.RunSeed, run.QuestionIndex, nil, now)
			}
			if !ok {
				return EndlessNextQuestionResponse{}, apperror.ServiceUnavailable("endless puzzle generation failed", nil)
			}
			run.Questions = append(run.Questions, next)
			run.UsedFingerprints = append(run.UsedFingerprints, endlessQuestionFingerprint(next))
			run.UsedNumberKeys = append(run.UsedNumberKeys, friendNumberKey(next.Numbers))
		}
	} else {
		if len(input.SolutionSteps) != 0 {
			return EndlessNextQuestionResponse{}, apperror.BadRequest("solution_steps must be empty when solved is false", nil)
		}
		run.Mistakes++
		run.Combo = 0
		run.Attempts = append(run.Attempts, EndlessRunAttemptInput{
			PuzzleID: current.PuzzleID, QuestionHash: current.QuestionHash, QuestionIndex: run.QuestionIndex,
			ElapsedMS: serverElapsed, Solved: false, Mistakes: run.Mistakes, Score: run.Score,
			ScoreDelta: 0, Combo: 0, SolutionSteps: []FriendMatchSolutionStep{},
		})
	}
	run.ElapsedMS = endlessRunElapsed(run, now)
	run.LastActivityAt = now
	if !run.DeadlineAt.IsZero() && !now.Before(run.DeadlineAt) {
		finishEndlessRun(&run, now)
	}
	var nextPuzzle *EndlessPuzzlePublic
	if run.Status == RunRunning && run.QuestionIndex < len(run.Questions) {
		public := publicEndlessPuzzle(run.Questions[run.QuestionIndex])
		nextPuzzle = &public
	}
	result := endlessNextResult(run, true, true, func() int {
		if input.Solved {
			return run.Attempts[len(run.Attempts)-1].ScoreDelta
		}
		return 0
	}(), nextPuzzle)
	run.NextResults[key] = result
	run.NextRequestHashes[key] = requestHash
	if err := persistEndlessRun(ctx, s.endlessRuns, run); err != nil {
		return EndlessNextQuestionResponse{}, err
	}
	return result, nil
}

func persistEndlessRun(ctx context.Context, runs EndlessRunStore, run EndlessRun) error {
	stateStore, ok := runs.(EndlessRunStateStore)
	if !ok {
		return apperror.ServiceUnavailable("endless run state store is unavailable", nil)
	}
	return stateStore.UpdateEndlessRun(ctx, run)
}

func endlessNextResult(run EndlessRun, accepted, validated bool, scoreDelta int, next *EndlessPuzzlePublic) EndlessNextQuestionResponse {
	return EndlessNextQuestionResponse{
		RunID: run.RunID, Status: runStatusOrRunning(run.Status), Accepted: accepted, Validated: validated,
		QuestionIndex: run.QuestionIndex, Score: run.Score, Mistakes: run.Mistakes, BestCombo: run.BestCombo,
		QuestionsSolved: endlessQuestionsSolved(run.Attempts), ElapsedMS: run.ElapsedMS, ScoreDelta: scoreDelta,
		NextPuzzle: next, RewardCoins: 0,
	}
}

func endlessQuestionsSolved(attempts []EndlessRunAttemptInput) int {
	count := 0
	for _, attempt := range attempts {
		if attempt.Solved {
			count++
		}
	}
	return count
}

func (s *Service) submitEndlessRunAuthoritative(ctx context.Context, userID uint64, runID string, input EndlessRunSubmissionInput) (EndlessRunSubmissionResponse, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" || input.RunID != runID || input.ProtocolVersion != endlessRunProtocolVersion || input.ClientAuthoritative {
		return EndlessRunSubmissionResponse{}, apperror.BadRequest("endless settlement protocol is invalid", nil)
	}
	key := strings.TrimSpace(input.IdempotencyKey)
	if len(key) < 8 || len(key) > 128 {
		return EndlessRunSubmissionResponse{}, apperror.BadRequest("idempotency_key length is invalid", nil)
	}
	release, err := s.acquireSettlementLock(ctx, "endless:"+runID)
	if err != nil {
		return EndlessRunSubmissionResponse{}, err
	}
	defer release()
	run, err := s.endlessRuns.GetEndlessRun(ctx, runID)
	if errors.Is(err, ErrEndlessRunNotFound) {
		return EndlessRunSubmissionResponse{}, apperror.NotFound("endless run was not found", err)
	}
	if err != nil {
		return EndlessRunSubmissionResponse{}, err
	}
	if run.UserID != userID {
		return EndlessRunSubmissionResponse{}, apperror.New(10004, 403, "you cannot submit this run", nil)
	}
	now := time.Now().UTC()
	if run.Status == RunExpired || (!run.ExpiresAt.IsZero() && now.After(run.ExpiresAt)) {
		return EndlessRunSubmissionResponse{}, apperror.New(10005, 410, "endless run has expired", nil)
	}
	if run.SubmitResults == nil {
		run.SubmitResults = map[string]EndlessRunSubmissionResponse{}
	}
	if previous, exists := run.SubmitResults[key]; exists {
		previous.IdempotencyReplayed = true
		return previous, nil
	}
	if run.Status == RunSubmitted {
		return EndlessRunSubmissionResponse{}, apperror.Conflict("endless run was already submitted", nil)
	}
	if run.Status == RunCancelled {
		return EndlessRunSubmissionResponse{}, apperror.Conflict("endless run was cancelled", nil)
	}
	if run.Status == RunRunning && !run.DeadlineAt.IsZero() && !now.Before(run.DeadlineAt) {
		finishEndlessRun(&run, now)
	}
	if run.Status == RunRunning {
		run.ElapsedMS = endlessRunElapsed(run, now)
	}
	calculated := endlessRunSummaryFromState(run, now)
	dateKey := now.In(shanghaiLocation).Format("2006-01-02")
	rewardCoins := 0
	if s.store == nil {
		return EndlessRunSubmissionResponse{}, apperror.ServiceUnavailable("player store is unavailable", nil)
	}
	progress, err := s.store.MutatePlayerProgress(ctx, userID, func(state map[string]any) error {
		var applyErr error
		rewardCoins, applyErr = applyEndlessServerResult(state, calculated, dateKey, run.RunID)
		return applyErr
	})
	if err != nil {
		return EndlessRunSubmissionResponse{}, err
	}
	metadata := map[string]any{"protocol_version": input.ProtocolVersion, "run_id": run.RunID, "reward_coins": rewardCoins, "server_authoritative": true}
	encoded, _ := json.Marshal(metadata)
	createErr := s.store.CreateLeaderboardSubmission(ctx, createLeaderboardSubmissionParams(userID, LeaderboardEndless, key, calculated.Score, calculated.Questions, calculated.ElapsedMS, "", "", string(encoded)))
	if createErr != nil && !store.IsDuplicateEntry(createErr) {
		return EndlessRunSubmissionResponse{}, createErr
	}
	result := EndlessRunSubmissionResponse{
		Mode: LeaderboardEndless, RunID: run.RunID, Status: RunSubmitted, Score: calculated.Score,
		Questions: calculated.Questions, ElapsedMS: calculated.ElapsedMS, Mistakes: calculated.Mistakes,
		BestCombo: calculated.BestCombo, RewardCoins: rewardCoins, Coins: progressCoins(string(progress)),
		Validated: true, LeaderboardUpdated: true, Progress: progress,
	}
	finishedAt := now
	run.Status = RunSubmitted
	run.IdempotencyKey = key
	run.SubmittedAt = &finishedAt
	run.FinishedAt = &finishedAt
	run.ElapsedMS = calculated.ElapsedMS
	run.Score = calculated.Score
	run.Mistakes = calculated.Mistakes
	run.BestCombo = calculated.BestCombo
	run.SubmitResults[key] = result
	if err := persistEndlessRun(ctx, s.endlessRuns, run); err != nil {
		return EndlessRunSubmissionResponse{}, err
	}
	return result, nil
}

func endlessRunSummaryFromState(run EndlessRun, now time.Time) endlessSummary {
	return endlessSummary{Score: maxInt(0, run.Score), Questions: endlessQuestionsSolved(run.Attempts), ElapsedMS: maxInt(0, minInt(run.ElapsedMS, int(endlessRunTimeLimit/time.Millisecond))), Mistakes: maxInt(0, run.Mistakes), BestCombo: maxInt(0, run.BestCombo)}
}

func finishEndlessRun(run *EndlessRun, now time.Time) {
	if run.Status == RunRunning {
		run.Status = RunFinished
		finished := now
		run.FinishedAt = &finished
	}
	run.ElapsedMS = endlessRunElapsed(*run, now)
	run.LastActivityAt = now
}

func endlessRunElapsed(run EndlessRun, now time.Time) int {
	if run.StartedAt.IsZero() {
		return maxInt(0, run.ElapsedMS)
	}
	elapsed := now.Sub(run.StartedAt)
	if elapsed < 0 {
		return 0
	}
	limit := run.TimeLimitMS
	if limit <= 0 {
		limit = int(endlessRunTimeLimit / time.Millisecond)
	}
	return minInt(limit, int(elapsed/time.Millisecond))
}

func endlessPuzzleElapsed(puzzle EndlessPuzzle, now, startedAt time.Time) int {
	base := puzzle.ServedAt
	if base.IsZero() {
		base = startedAt
	}
	if base.IsZero() || now.Before(base) {
		return 0
	}
	elapsed := maxInt(0, int(now.Sub(base)/time.Millisecond))
	if puzzle.TimeLimitMS > 0 {
		elapsed = minInt(puzzle.TimeLimitMS, elapsed)
	}
	return elapsed
}

func calculateEndlessScoreDelta(puzzle EndlessPuzzle, elapsedMS, combo, questionIndex int) int {
	limit := puzzle.TimeLimitMS
	if limit <= 0 {
		limit = int(endlessRunTimeLimit / time.Millisecond)
	}
	remaining := maxInt(0, limit-elapsedMS)
	delta := 40 + remaining/1000 + minInt(60, questionIndex*2) + minInt(100, combo*10)
	if puzzle.Difficulty == "hard" || puzzle.Difficulty == "expert" {
		delta += 20
	}
	return minInt(500, maxInt(10, delta))
}

func validateEndlessSolution(puzzle EndlessPuzzle, steps []FriendMatchSolutionStep) error {
	if len(puzzle.Numbers) != 4 || len(steps) != 3 {
		return apperror.BadRequest("solution_steps are invalid", nil)
	}
	for _, number := range puzzle.Numbers {
		if number < 1 || number > 9 {
			return apperror.BadRequest("endless puzzle contains an invalid digit", nil)
		}
	}
	normalized, err := normalizeEndlessSolutionSteps(puzzle, steps)
	if err != nil {
		return err
	}
	steps = normalized
	for _, step := range steps {
		if !endlessOperatorAllowed(step.Operator) || len(step.FirstIndices) == 0 || len(step.SecondIndices) == 0 {
			return apperror.BadRequest("solution_steps contain an invalid operation", nil)
		}
		seen := map[int]bool{}
		for _, index := range append(append([]int(nil), step.FirstIndices...), step.SecondIndices...) {
			if index < 0 || index >= len(puzzle.Numbers) || seen[index] {
				return apperror.BadRequest("solution_steps contain an invalid number reference", nil)
			}
			seen[index] = true
		}
	}
	if !replayFriendSolution(puzzle.Numbers, steps, puzzle.Rules) {
		return apperror.BadRequest("solution_steps do not solve the server puzzle", nil)
	}
	return nil
}

func normalizeEndlessSolutionSteps(puzzle EndlessPuzzle, steps []FriendMatchSolutionStep) ([]FriendMatchSolutionStep, error) {
	type item struct {
		value   int
		indices []int
	}
	items := make([]item, len(puzzle.Numbers))
	for index, number := range puzzle.Numbers {
		items[index] = item{value: number, indices: []int{index}}
	}
	sameGroup := func(left, right []int) bool {
		if len(left) != len(right) {
			return false
		}
		seen := map[int]int{}
		for _, value := range left {
			seen[value]++
		}
		for _, value := range right {
			if seen[value] == 0 {
				return false
			}
			seen[value]--
		}
		return true
	}
	findGroup := func(group []int) int {
		for index, value := range items {
			if sameGroup(value.indices, group) {
				return index
			}
		}
		return -1
	}
	result := make([]FriendMatchSolutionStep, len(steps))
	for stepIndex, original := range steps {
		step := original
		if len(step.FirstIndices) == 0 || len(step.SecondIndices) == 0 {
			if step.Left == nil || step.Right == nil {
				return nil, apperror.BadRequest("solution_steps do not identify operands", nil)
			}
			firstIndex, secondIndex := -1, -1
			for index, value := range items {
				if value.value == *step.Left && firstIndex < 0 {
					firstIndex = index
				}
			}
			for index, value := range items {
				if index != firstIndex && value.value == *step.Right {
					secondIndex = index
					break
				}
			}
			if firstIndex < 0 || secondIndex < 0 {
				return nil, apperror.BadRequest("solution_steps reference unavailable operands", nil)
			}
			step.FirstIndices = append([]int(nil), items[firstIndex].indices...)
			step.SecondIndices = append([]int(nil), items[secondIndex].indices...)
		}
		firstIndex := findGroup(step.FirstIndices)
		secondIndex := findGroup(step.SecondIndices)
		if firstIndex < 0 || secondIndex < 0 || firstIndex == secondIndex {
			return nil, apperror.BadRequest("solution_steps contain an invalid operation order", nil)
		}
		first, second := items[firstIndex], items[secondIndex]
		if (step.First != 0 && step.First != first.value) || (step.Second != 0 && step.Second != second.value) {
			return nil, apperror.BadRequest("solution_steps operand values do not match the puzzle", nil)
		}
		if step.Left != nil && *step.Left != first.value || step.Right != nil && *step.Right != second.value {
			return nil, apperror.BadRequest("solution_steps operand values do not match the puzzle", nil)
		}
		value, ok := applyFriendOperator(first.value, second.value, step.Operator)
		if !ok || (puzzle.Rules.IntegerIntermediateResults && value != int(value)) || (!puzzle.Rules.AllowNegativeIntermediate && value < 0) {
			return nil, apperror.BadRequest("solution_steps contain an invalid intermediate result", nil)
		}
		if step.Result != nil && *step.Result != value {
			return nil, apperror.BadRequest("solution_steps result does not match the puzzle", nil)
		}
		step.First = first.value
		step.Second = second.value
		result[stepIndex] = step
		next := make([]item, 0, len(items)-1)
		for index, value := range items {
			if index != firstIndex && index != secondIndex {
				next = append(next, value)
			}
		}
		indices := append([]int(nil), first.indices...)
		indices = append(indices, second.indices...)
		next = append(next, item{value: value, indices: indices})
		items = next
	}
	target := puzzle.Target
	if target == 0 {
		target = 24
	}
	if len(items) != 1 || items[0].value != target || len(steps) != 3 {
		return nil, apperror.BadRequest("solution_steps do not solve the puzzle", nil)
	}
	return result, nil
}

func endlessOperatorAllowed(operator string) bool {
	switch strings.TrimSpace(operator) {
	case "+", "-", "*", "/", "×", "÷", "脳", "梅":
		return true
	default:
		return false
	}
}

func cloneSolutionSteps(steps []FriendMatchSolutionStep) []FriendMatchSolutionStep {
	result := make([]FriendMatchSolutionStep, len(steps))
	for index, step := range steps {
		result[index] = step
		result[index].FirstIndices = append([]int(nil), step.FirstIndices...)
		result[index].SecondIndices = append([]int(nil), step.SecondIndices...)
	}
	return result
}

func hashEndlessNextRequest(input EndlessNextQuestionInput) string {
	payload, _ := json.Marshal(struct {
		ProtocolVersion int                       `json:"protocol_version"`
		QuestionIndex   int                       `json:"question_index"`
		PuzzleID        string                    `json:"puzzle_id"`
		ElapsedMS       int                       `json:"elapsed_ms"`
		Solved          bool                      `json:"solved"`
		Mistakes        int                       `json:"mistakes"`
		ScoreDelta      int                       `json:"score_delta"`
		Combo           int                       `json:"combo"`
		SolutionSteps   []FriendMatchSolutionStep `json:"solution_steps"`
	}{input.ProtocolVersion, input.QuestionIndex, input.PuzzleID, input.ElapsedMS, input.Solved, input.Mistakes, input.ScoreDelta, input.Combo, input.SolutionSteps})
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func endlessRunUsedSet(run EndlessRun) map[string]struct{} {
	result := stringSet(run.UsedFingerprints)
	numbers := run.UsedNumberKeys
	if len(numbers) == 0 {
		for _, puzzle := range run.Questions {
			numbers = append(numbers, friendNumberKey(puzzle.Numbers))
		}
	}
	for _, value := range numbers {
		result["numbers:"+value] = struct{}{}
	}
	return result
}

func recentEndlessUsedSet(run EndlessRun) map[string]struct{} {
	result := map[string]struct{}{}
	fingerprints := run.UsedFingerprints
	numbers := run.UsedNumberKeys
	if len(numbers) == 0 {
		for _, puzzle := range run.Questions {
			numbers = append(numbers, friendNumberKey(puzzle.Numbers))
		}
	}
	start := maxInt(0, len(fingerprints)-50)
	for _, value := range fingerprints[start:] {
		result[value] = struct{}{}
	}
	start = maxInt(0, len(numbers)-50)
	for _, value := range numbers[start:] {
		result["numbers:"+value] = struct{}{}
	}
	return result
}

func endlessQuestionFingerprint(puzzle EndlessPuzzle) string {
	payload, _ := json.Marshal(struct {
		Numbers     []int  `json:"numbers"`
		Difficulty  string `json:"difficulty"`
		RuleVersion int    `json:"rule_version"`
	}{puzzle.Numbers, puzzle.Difficulty, 1})
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func generateEndlessPuzzle(seed int64, index int, used map[string]struct{}, servedAt time.Time) (EndlessPuzzle, bool) {
	if used == nil {
		used = map[string]struct{}{}
	}
	friendCandidatePool()
	candidates := append([]friendCandidate(nil), friendCandidateCache.values...)
	random := newFriendSeededRandom(seed + int64(index+1)*0x9e3779b9)
	for position := len(candidates) - 1; position > 0; position-- {
		swap := random.int(0, position)
		candidates[position], candidates[swap] = candidates[swap], candidates[position]
	}
	desired := endlessDifficulty(index)
	for pass := 0; pass < 2; pass++ {
		for _, candidate := range candidates {
			if pass == 0 && candidate.difficulty != desired && !(desired == "expert" && candidate.difficulty == "hard") {
				continue
			}
			rules := endlessPuzzleRules()
			solutions := verifiedFriendSolutions(candidate.numbers, rules, 80)
			if len(solutions) == 0 {
				continue
			}
			puzzle := EndlessPuzzle{
				PuzzleID: fmt.Sprintf("ENDLESS-Q%04d", index+1), Numbers: append([]int(nil), candidate.numbers...), Target: 24, Rules: rules,
				SourceSeed: fmt.Sprintf("%d", seed), SolutionCount: len(solutions), ShortestSteps: shortestSolutionSteps(solutions),
				Difficulty: desired, TimeLimitMS: int(endlessRunTimeLimit / time.Millisecond), ServedAt: servedAt,
				SolutionSteps: cloneSolutionSteps(solutions[0].steps),
			}
			puzzle.QuestionHash = puzzleQuestionHash(puzzle.Numbers, puzzle.Rules, seed, index)
			proof, _ := json.Marshal(puzzle.SolutionSteps)
			hash := sha256.Sum256(proof)
			puzzle.SolutionHash = hex.EncodeToString(hash[:])
			if _, exists := used[endlessQuestionFingerprint(puzzle)]; exists {
				continue
			}
			if _, exists := used["numbers:"+friendNumberKey(puzzle.Numbers)]; exists {
				continue
			}
			return puzzle, true
		}
	}
	return EndlessPuzzle{}, false
}

func endlessDifficulty(index int) string {
	switch {
	case index < 5:
		return "easy"
	case index < 10:
		return "standard"
	case index < 20:
		return "hard"
	default:
		return "expert"
	}
}

func endlessPuzzleRules() FriendPuzzleRules {
	return FriendPuzzleRules{UseEachNumberOnce: true, IntegerIntermediateResults: true, AllowedOperators: []string{"+", "-", "×", "÷"}, AllowNegativeIntermediate: true}
}
