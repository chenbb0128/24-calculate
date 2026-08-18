package player

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (r *FriendRoomRepository) createEndlessRunMySQL(ctx context.Context, run EndlessRun) error {
	if r == nil || r.database == nil {
		return fmt.Errorf("endless run mysql repository is not initialized")
	}
	normalizeEndlessRunTimes(&run)
	state, err := json.Marshal(run)
	if err != nil {
		return fmt.Errorf("encode endless run state: %w", err)
	}
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var startKey any
	if strings.TrimSpace(run.StartKey) != "" {
		startKey = strings.TrimSpace(run.StartKey)
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO endless_runs (
    run_id, user_id, run_seed, status, current_question_index, score, mistakes, best_combo,
    time_limit_ms, started_at, last_activity_at, deadline_at, expires_at, submitted_at,
    reward_claimed, version, state_version, start_idempotency_key, state_json, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.RunID, run.UserID, run.RunSeed, run.Status, run.QuestionIndex, run.Score, run.Mistakes, run.BestCombo,
		run.TimeLimitMS, run.StartedAt, run.LastActivityAt, run.DeadlineAt, run.ExpiresAt, nullableTime(run.SubmittedAt),
		run.Status == RunSubmitted, run.Version, run.StateVersion, startKey, string(state), run.CreatedAt)
	if err != nil {
		return err
	}
	if err := insertEndlessQuestions(ctx, tx, run); err != nil {
		return err
	}
	if err := insertEndlessAttempts(ctx, tx, run); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *FriendRoomRepository) getEndlessRunMySQL(ctx context.Context, runID string) (EndlessRun, error) {
	if r == nil || r.database == nil {
		return EndlessRun{}, fmt.Errorf("endless run mysql repository is not initialized")
	}
	var state string
	err := r.database.QueryRowContext(ctx, `SELECT state_json FROM endless_runs WHERE run_id = ? LIMIT 1`, strings.TrimSpace(runID)).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return EndlessRun{}, ErrEndlessRunNotFound
	}
	if err != nil {
		return EndlessRun{}, err
	}
	var run EndlessRun
	if err := json.Unmarshal([]byte(state), &run); err != nil {
		return EndlessRun{}, fmt.Errorf("decode endless run state: %w", err)
	}
	return run, nil
}

func (r *FriendRoomRepository) GetEndlessRunByStartKey(ctx context.Context, userID uint64, key string) (EndlessRun, error) {
	if r == nil || r.database == nil {
		return EndlessRun{}, fmt.Errorf("endless run mysql repository is not initialized")
	}
	var state string
	err := r.database.QueryRowContext(ctx, `
SELECT state_json FROM endless_runs
WHERE user_id = ? AND start_idempotency_key = ?
LIMIT 1`, userID, strings.TrimSpace(key)).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return EndlessRun{}, ErrEndlessRunNotFound
	}
	if err != nil {
		return EndlessRun{}, err
	}
	var run EndlessRun
	if err := json.Unmarshal([]byte(state), &run); err != nil {
		return EndlessRun{}, fmt.Errorf("decode endless run state: %w", err)
	}
	return run, nil
}

func (r *FriendRoomRepository) updateEndlessRunMySQL(ctx context.Context, run EndlessRun) error {
	if r == nil || r.database == nil {
		return fmt.Errorf("endless run mysql repository is not initialized")
	}
	normalizeEndlessRunTimes(&run)
	state, err := json.Marshal(run)
	if err != nil {
		return fmt.Errorf("encode endless run state: %w", err)
	}
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `
UPDATE endless_runs SET
    status = ?, current_question_index = ?, score = ?, mistakes = ?, best_combo = ?,
    time_limit_ms = ?, started_at = ?, last_activity_at = ?, deadline_at = ?, expires_at = ?,
    submitted_at = ?, reward_claimed = ?, version = ?, state_version = ?, state_json = ?
WHERE run_id = ?`,
		run.Status, run.QuestionIndex, run.Score, run.Mistakes, run.BestCombo, run.TimeLimitMS,
		run.StartedAt, run.LastActivityAt, run.DeadlineAt, run.ExpiresAt, nullableTime(run.SubmittedAt),
		run.Status == RunSubmitted, run.Version, run.StateVersion, string(state), run.RunID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrEndlessRunNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM endless_run_questions WHERE run_id = ?`, run.RunID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM endless_run_attempts WHERE run_id = ?`, run.RunID); err != nil {
		return err
	}
	if err := insertEndlessQuestions(ctx, tx, run); err != nil {
		return err
	}
	if err := insertEndlessAttempts(ctx, tx, run); err != nil {
		return err
	}
	return tx.Commit()
}

func insertEndlessQuestions(ctx context.Context, tx *sql.Tx, run EndlessRun) error {
	answered := map[int]bool{}
	for _, attempt := range run.Attempts {
		if attempt.Solved {
			answered[attempt.QuestionIndex] = true
		}
	}
	for index, puzzle := range run.Questions {
		numbers, _ := json.Marshal(puzzle.Numbers)
		rules, _ := json.Marshal(puzzle.Rules)
		steps, _ := json.Marshal(puzzle.SolutionSteps)
		var answeredAt any
		if answered[index] {
			answeredAt = run.LastActivityAt
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO endless_run_questions (
    run_id, question_index, puzzle_id, question_hash, numbers_json, target, rules_json,
    solution_hash, difficulty, served_at, answered_at, solution_steps_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			run.RunID, index, puzzle.PuzzleID, puzzle.QuestionHash, string(numbers), 24, string(rules),
			puzzle.SolutionHash, puzzle.Difficulty, puzzle.ServedAt, answeredAt, string(steps)); err != nil {
			return err
		}
	}
	return nil
}

func insertEndlessAttempts(ctx context.Context, tx *sql.Tx, run EndlessRun) error {
	for index, attempt := range run.Attempts {
		steps, _ := json.Marshal(attempt.SolutionSteps)
		key := fmt.Sprintf("%s-attempt-%d", run.RunID, index)
		if _, err := tx.ExecContext(ctx, `
INSERT INTO endless_run_attempts (
    run_id, question_index, idempotency_key, solved, validated, elapsed_ms, mistakes,
    score, score_delta, combo, solution_steps_json, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			run.RunID, attempt.QuestionIndex, key, attempt.Solved, true, attempt.ElapsedMS, attempt.Mistakes,
			attempt.Score, attempt.ScoreDelta, attempt.Combo, string(steps), run.LastActivityAt); err != nil {
			return err
		}
	}
	return nil
}

func normalizeEndlessRunTimes(run *EndlessRun) {
	now := time.Now().UTC()
	if run.Status == "" {
		run.Status = RunRunning
	}
	if run.Version == 0 {
		run.Version = endlessRunProtocolVersion
	}
	if run.StateVersion == 0 {
		run.StateVersion = endlessRunStateVersion
	}
	if run.CreatedAt.IsZero() {
		run.CreatedAt = now
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = run.CreatedAt
	}
	if run.TimeLimitMS <= 0 {
		run.TimeLimitMS = int(endlessRunTimeLimit / time.Millisecond)
	}
	if run.DeadlineAt.IsZero() {
		run.DeadlineAt = run.StartedAt.Add(time.Duration(run.TimeLimitMS) * time.Millisecond)
	}
	if run.LastActivityAt.IsZero() {
		run.LastActivityAt = run.CreatedAt
	}
	if run.ExpiresAt.IsZero() {
		run.ExpiresAt = run.CreatedAt.Add(endlessRunTTL)
	}
}

func nullableTime(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return *value
}
