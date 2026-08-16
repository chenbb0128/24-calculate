package player

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/example/go-service/internal/apperror"
	"github.com/example/go-service/internal/store"
	db "github.com/example/go-service/internal/store/sqlc"
)

type Repository struct {
	queries *db.Queries
	tx      *store.TxManager
}

func (r *Repository) MutatePlayerProgress(ctx context.Context, userID uint64, mutate ProgressMutation) (result json.RawMessage, err error) {
	if r == nil || r.tx == nil {
		return nil, fmt.Errorf("player repository transaction manager is not initialized")
	}
	err = r.tx.Exec(ctx, func(queries *db.Queries) error {
		now := time.Now().UTC()
		if err := queries.EnsurePlayerProfile(ctx, db.EnsurePlayerProfileParams{
			UserID:       userID,
			ProgressJSON: DefaultProgressJSON,
			CreatedAt:    now,
			UpdatedAt:    now,
		}); err != nil {
			return err
		}
		profile, err := queries.GetPlayerProfileForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		state := decodeProgress(profile.ProgressJSON)
		if mutate != nil {
			if err := mutate(state); err != nil {
				return err
			}
		}
		encoded, err := json.Marshal(state)
		if err != nil {
			return fmt.Errorf("encode player progress: %w", err)
		}
		if err := queries.UpdatePlayerProfileProgress(ctx, db.UpdatePlayerProfileProgressParams{
			ProgressJSON: string(encoded),
			UpdatedAt:    now,
			UserID:       userID,
		}); err != nil {
			return err
		}
		result = json.RawMessage(encoded)
		return nil
	})
	return result, err
}

func NewRepository(queries *db.Queries, tx *store.TxManager) *Repository {
	return &Repository{queries: queries, tx: tx}
}

func (r *Repository) GetPlayerProfile(ctx context.Context, userID uint64) (db.PlayerProfile, error) {
	if r == nil || r.queries == nil {
		return db.PlayerProfile{}, fmt.Errorf("player repository is not initialized")
	}
	return r.queries.GetPlayerProfile(ctx, userID)
}

func (r *Repository) CreatePlayerProfile(ctx context.Context, arg db.CreatePlayerProfileParams) error {
	if r == nil || r.queries == nil {
		return fmt.Errorf("player repository is not initialized")
	}
	return r.queries.CreatePlayerProfile(ctx, arg)
}

func (r *Repository) ListCampaignLeaderboard(ctx context.Context) ([]db.CampaignLeaderboardRow, error) {
	if r == nil || r.queries == nil {
		return nil, fmt.Errorf("player repository is not initialized")
	}
	return r.queries.ListCampaignLeaderboard(ctx)
}

func (r *Repository) ListDailyLeaderboard(ctx context.Context, dateKey string) ([]db.DailyLeaderboardRow, error) {
	if r == nil || r.queries == nil {
		return nil, fmt.Errorf("player repository is not initialized")
	}
	return r.queries.ListDailyLeaderboard(ctx, db.ListDailyLeaderboardParams{DateKey: dateKey})
}

func (r *Repository) GetLeaderboardSubmissionByKey(ctx context.Context, arg db.GetLeaderboardSubmissionByKeyParams) (db.PlayerLeaderboardSubmission, error) {
	if r == nil || r.queries == nil {
		return db.PlayerLeaderboardSubmission{}, fmt.Errorf("player repository is not initialized")
	}
	return r.queries.GetLeaderboardSubmissionByKey(ctx, arg)
}

func (r *Repository) CreateLeaderboardSubmission(ctx context.Context, arg db.CreateLeaderboardSubmissionParams) error {
	if r == nil || r.queries == nil {
		return fmt.Errorf("player repository is not initialized")
	}
	return r.queries.CreateLeaderboardSubmission(ctx, arg)
}

func (r *Repository) ListEndlessLeaderboard(ctx context.Context) ([]db.LeaderboardScoreRow, error) {
	if r == nil || r.queries == nil {
		return nil, fmt.Errorf("player repository is not initialized")
	}
	return r.queries.ListEndlessLeaderboard(ctx)
}

func (r *Repository) ListFriendLeaderboard(ctx context.Context) ([]db.LeaderboardScoreRow, error) {
	if r == nil || r.queries == nil {
		return nil, fmt.Errorf("player repository is not initialized")
	}
	return r.queries.ListFriendLeaderboard(ctx)
}

func (r *Repository) ListFriendUserIDs(ctx context.Context, userID uint64) ([]uint64, error) {
	if r == nil || r.queries == nil {
		return nil, fmt.Errorf("player repository is not initialized")
	}
	return r.queries.ListFriendUserIDs(ctx, userID)
}

func (r *Repository) CompleteLevel(ctx context.Context, params CompleteLevelParams) (result CompleteLevelResult, err error) {
	if r == nil || r.tx == nil {
		return CompleteLevelResult{}, fmt.Errorf("player repository transaction manager is not initialized")
	}
	err = r.tx.Exec(ctx, func(queries *db.Queries) error {
		now := time.Now().UTC()
		if err := queries.EnsurePlayerProfile(ctx, db.EnsurePlayerProfileParams{
			UserID:       params.UserID,
			ProgressJSON: DefaultProgressJSON,
			CreatedAt:    now,
			UpdatedAt:    now,
		}); err != nil {
			return err
		}

		profile, err := queries.GetPlayerProfileForUpdate(ctx, params.UserID)
		if err != nil {
			return err
		}
		previous, err := queries.GetPlayerLevelCompletionByKey(ctx, db.GetPlayerLevelCompletionByKeyParams{
			UserID:         params.UserID,
			IdempotencyKey: params.IdempotencyKey,
		})
		if err == nil {
			result = CompleteLevelResult{
				LevelID:             int(previous.LevelID),
				Stars:               int(previous.Stars),
				BestScore:           int(previous.BestScore),
				RewardCoins:         int(previous.RewardCoins),
				Coins:               progressCoins(profile.ProgressJSON),
				UnlockedLevel:       int(previous.UnlockedLevel),
				Progress:            json.RawMessage([]byte(profile.ProgressJSON)),
				IdempotencyReplayed: true,
			}
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		state := decodeProgress(profile.ProgressJSON)
		if params.LevelID > readInt(state["unlocked_level"]) {
			return apperror.New(10008, 403, "当前关卡尚未解锁", nil)
		}
		levels := ensureObject(state, "levels")
		levelKey := fmt.Sprintf("%d", params.LevelID)
		level := ensureObject(levels, levelKey)
		bestScore := maxInt(readInt(level["best_score"]), params.Score)
		bestStars := maxInt(readInt(level["stars"]), params.Stars)
		level["best_score"] = bestScore
		level["stars"] = bestStars
		levels[levelKey] = level
		state["levels"] = levels

		unlockedLevel := maxInt(readInt(state["unlocked_level"]), params.LevelID+1)
		state["unlocked_level"] = unlockedLevel
		state["last_level"] = maxInt(readInt(state["last_level"]), params.LevelID)

		levelRewards := ensureObject(state, "level_rewards")
		rewardCoins := 0
		if !readBool(levelRewards[levelKey]) {
			rewardCoins = 8 + params.Stars*3
			levelRewards[levelKey] = true
		}
		state["level_rewards"] = levelRewards
		coins := maxInt(0, readInt(state["coins"])+rewardCoins)
		state["coins"] = minInt(999999, coins)
		dateKey := now.In(shanghaiLocation).Format("2006-01-02")
		rewardCoins += recordServerDailyTask(state, "campaign_clear", 1, dateKey, 1, 15)
		rewardCoins += recordServerDailyTaskMax(state, "combo", params.BestCombo, dateKey, 5, 20)
		rewardCoins += recordServerWeeklyTask(state, "weekly_campaign", 1, dateKey, 5, 45)
		rewardCoins += unlockServerAchievement(state, "first_clear", params.LevelID == 0, 30)
		rewardCoins += unlockServerAchievement(state, "three_star", bestStars >= 3, 50)
		rewardCoins += unlockServerAchievement(state, "perfect_clear", params.Mistakes == 0 && params.Hints == 0, 80)
		rewardCoins += unlockServerAchievement(state, "combo_5", params.BestCombo >= 5, 30)
		rewardCoins += unlockServerAchievement(state, "combo_10", params.BestCombo >= 10, 80)
		recordServerSolveStats(state, "campaign", maxInt(1, params.Questions), params.Score, params.ElapsedMS, params.FastestMS, params.BestCombo, params.LevelID, params.Operators)
		state["coins"] = minInt(999999, readInt(state["coins"])+rewardCoins-(8+params.Stars*3))

		encoded, err := json.Marshal(state)
		if err != nil {
			return fmt.Errorf("encode player progress: %w", err)
		}
		if err := queries.UpdatePlayerProfileProgress(ctx, db.UpdatePlayerProfileProgressParams{
			ProgressJSON: string(encoded),
			UpdatedAt:    now,
			UserID:       params.UserID,
		}); err != nil {
			return err
		}
		if err := queries.CreatePlayerLevelCompletion(ctx, db.CreatePlayerLevelCompletionParams{
			UserID:         params.UserID,
			LevelID:        uint32(params.LevelID),
			IdempotencyKey: params.IdempotencyKey,
			Score:          uint16(params.Score),
			Stars:          uint8(params.Stars),
			RewardCoins:    uint32(rewardCoins),
			BestScore:      uint16(bestScore),
			UnlockedLevel:  uint32(unlockedLevel),
			CreatedAt:      now,
		}); err != nil {
			return err
		}
		result = CompleteLevelResult{
			LevelID:       params.LevelID,
			Stars:         bestStars,
			BestScore:     bestScore,
			RewardCoins:   rewardCoins,
			Coins:         int(state["coins"].(int)),
			UnlockedLevel: unlockedLevel,
			Progress:      json.RawMessage(encoded),
		}
		return nil
	})
	return result, err
}

func (r *Repository) CompleteDaily(ctx context.Context, params CompleteDailyParams) (result CompleteDailyResult, err error) {
	if r == nil || r.tx == nil {
		return CompleteDailyResult{}, fmt.Errorf("player repository transaction manager is not initialized")
	}
	err = r.tx.Exec(ctx, func(queries *db.Queries) error {
		now := time.Now().UTC()
		if err := queries.EnsurePlayerProfile(ctx, db.EnsurePlayerProfileParams{
			UserID:       params.UserID,
			ProgressJSON: DefaultProgressJSON,
			CreatedAt:    now,
			UpdatedAt:    now,
		}); err != nil {
			return err
		}
		profile, err := queries.GetPlayerProfileForUpdate(ctx, params.UserID)
		if err != nil {
			return err
		}

		dateKey := now.In(shanghaiLocation).Format("2006-01-02")
		existing, err := queries.GetPlayerDailyCompletionByDate(ctx, db.GetPlayerDailyCompletionByDateParams{
			UserID:  params.UserID,
			DateKey: dateKey,
		})
		if err == nil {
			result = CompleteDailyResult{
				DateKey:             existing.DateKey,
				Score:               int(existing.Score),
				BestScore:           int(existing.BestScore),
				Streak:              int(existing.Streak),
				RewardCoins:         int(existing.RewardCoins),
				Coins:               progressCoins(profile.ProgressJSON),
				Progress:            json.RawMessage([]byte(profile.ProgressJSON)),
				IdempotencyReplayed: true,
			}
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		previousDate := previousShanghaiDate(dateKey)
		previous, previousErr := queries.GetPlayerDailyCompletionByDate(ctx, db.GetPlayerDailyCompletionByDateParams{
			UserID:  params.UserID,
			DateKey: previousDate,
		})
		streak := 1
		if previousErr == nil {
			streak = int(previous.Streak) + 1
		} else if !errors.Is(previousErr, sql.ErrNoRows) {
			return previousErr
		}

		state := decodeProgress(profile.ProgressJSON)
		daily := ensureObject(state, "daily")
		completed := ensureObject(daily, "completed")
		rewardClaimed := ensureObject(daily, "reward_claimed")
		currentBest := readInt(daily["best_score"])
		bestScore := maxInt(currentBest, params.Score)
		completed[dateKey] = true
		rewardClaimed[dateKey] = true
		daily["completed"] = completed
		daily["reward_claimed"] = rewardClaimed
		daily["last_date"] = dateKey
		daily["streak"] = streak
		daily["best_score"] = bestScore
		state["daily"] = daily

		rewardCoins := 15
		coins := minInt(999999, maxInt(0, readInt(state["coins"])+rewardCoins))
		state["coins"] = coins
		rewardCoins += recordServerDailyTaskMax(state, "combo", params.BestCombo, dateKey, 5, 20)
		rewardCoins += recordServerWeeklyTask(state, "weekly_daily", 1, dateKey, 3, 35)
		rewardCoins += unlockServerAchievement(state, "daily_3", streak >= 3, 60)
		rewardCoins += unlockServerAchievement(state, "daily_7", streak >= 7, 120)
		recordServerSolveStats(state, "daily", maxInt(1, params.Questions), params.Score, params.ElapsedMS, params.FastestMS, params.BestCombo, -1, params.Operators)
		state["coins"] = minInt(999999, readInt(state["coins"])+rewardCoins-15)
		encoded, err := json.Marshal(state)
		if err != nil {
			return fmt.Errorf("encode player progress: %w", err)
		}
		if err := queries.UpdatePlayerProfileProgress(ctx, db.UpdatePlayerProfileProgressParams{
			ProgressJSON: string(encoded),
			UpdatedAt:    now,
			UserID:       params.UserID,
		}); err != nil {
			return err
		}
		if err := queries.CreatePlayerDailyCompletion(ctx, db.CreatePlayerDailyCompletionParams{
			UserID:         params.UserID,
			DateKey:        dateKey,
			IdempotencyKey: params.IdempotencyKey,
			Score:          uint32(params.Score),
			BestScore:      uint32(bestScore),
			Streak:         uint32(streak),
			RewardCoins:    uint32(rewardCoins),
			CreatedAt:      now,
		}); err != nil {
			return err
		}
		result = CompleteDailyResult{
			DateKey:     dateKey,
			Score:       params.Score,
			BestScore:   bestScore,
			Streak:      streak,
			RewardCoins: rewardCoins,
			Coins:       readInt(state["coins"]),
			Progress:    json.RawMessage(encoded),
		}
		return nil
	})
	return result, err
}

func decodeProgress(raw string) map[string]any {
	state := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &state); err != nil || state == nil {
		_ = json.Unmarshal([]byte(DefaultProgressJSON), &state)
	}
	return state
}

func progressCoins(raw string) int {
	return readInt(decodeProgress(raw)["coins"])
}

func ensureObject(parent map[string]any, key string) map[string]any {
	if value, ok := parent[key].(map[string]any); ok && value != nil {
		return value
	}
	value := map[string]any{}
	parent[key] = value
	return value
}

func readInt(value any) int {
	switch number := value.(type) {
	case int:
		return number
	case int64:
		return int(number)
	case float64:
		return int(number)
	case json.Number:
		parsed, _ := number.Int64()
		return int(parsed)
	default:
		return 0
	}
}

func readBool(value any) bool {
	result, _ := value.(bool)
	return result
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func previousShanghaiDate(dateKey string) string {
	date, err := time.ParseInLocation("2006-01-02", dateKey, shanghaiLocation)
	if err != nil {
		return dateKey
	}
	return date.AddDate(0, 0, -1).Format("2006-01-02")
}

func recordServerDailyTask(state map[string]any, taskID string, amount int, dateKey string, target, reward int) int {
	return recordServerDailyTaskInternal(state, taskID, amount, dateKey, target, reward, false)
}

func recordServerDailyTaskMax(state map[string]any, taskID string, amount int, dateKey string, target, reward int) int {
	return recordServerDailyTaskInternal(state, taskID, amount, dateKey, target, reward, true)
}

func recordServerDailyTaskInternal(state map[string]any, taskID string, amount int, dateKey string, target, reward int, useMax bool) int {
	tasks := ensureObject(state, "tasks")
	if fmt.Sprint(tasks["date"]) != dateKey {
		tasks = map[string]any{"date": dateKey, "values": map[string]any{}, "claimed": map[string]any{}}
		state["tasks"] = tasks
	}
	values := ensureObject(tasks, "values")
	claimed := ensureObject(tasks, "claimed")
	old := readInt(values[taskID])
	next := minInt(target, old+amount)
	if useMax {
		next = minInt(target, maxInt(old, amount))
	}
	values[taskID] = next
	if next < target || readBool(claimed[taskID]) {
		return 0
	}
	claimed[taskID] = true
	return reward
}

func recordServerWeeklyTask(state map[string]any, taskID string, amount int, dateKey string, target, reward int) int {
	week := fmt.Sprintf("%d", time.Date(
		mustParseShanghaiDate(dateKey).Year(),
		mustParseShanghaiDate(dateKey).Month(),
		mustParseShanghaiDate(dateKey).Day(), 0, 0, 0, 0, time.UTC,
	).Unix()/604800)
	weekly := ensureObject(state, "weekly_tasks")
	if fmt.Sprint(weekly["week"]) != week {
		weekly = map[string]any{"week": week, "values": map[string]any{}, "claimed": map[string]any{}}
		state["weekly_tasks"] = weekly
	}
	values := ensureObject(weekly, "values")
	claimed := ensureObject(weekly, "claimed")
	next := minInt(target, readInt(values[taskID])+amount)
	values[taskID] = next
	if next < target || readBool(claimed[taskID]) {
		return 0
	}
	claimed[taskID] = true
	return reward
}

func unlockServerAchievement(state map[string]any, achievementID string, eligible bool, reward int) int {
	if !eligible {
		return 0
	}
	achievements := ensureObject(state, "achievements")
	unlocked := ensureObject(achievements, "unlocked")
	claimed := ensureObject(achievements, "claimed")
	if readBool(unlocked[achievementID]) {
		return 0
	}
	unlocked[achievementID] = true
	claimed[achievementID] = true
	return reward
}

func mustParseShanghaiDate(dateKey string) time.Time {
	date, err := time.ParseInLocation("2006-01-02", dateKey, shanghaiLocation)
	if err != nil {
		return time.Now().In(shanghaiLocation)
	}
	return date
}
