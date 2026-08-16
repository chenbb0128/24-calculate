package player

import (
	"encoding/json"

	"github.com/example/go-service/internal/modules/user"
)

type BootstrapResponse struct {
	User        user.ProfileResponse `json:"user"`
	Progress    json.RawMessage      `json:"progress"`
	LoginReward int                  `json:"login_reward"`
	ServerDate  string               `json:"server_date"`
}

type CompleteLevelInput struct {
	IdempotencyKey string `json:"idempotency_key"`
	Score          int    `json:"score"`
	Stars          int    `json:"stars"`
}

type CompleteLevelParams struct {
	UserID         uint64
	LevelID        int
	IdempotencyKey string
	Score          int
	Stars          int
	Questions      int
	ElapsedMS      int
	FastestMS      int
	Mistakes       int
	Hints          int
	BestCombo      int
	Operators      []string
}

type CompleteLevelResult struct {
	LevelID             int
	Stars               int
	BestScore           int
	RewardCoins         int
	Coins               int
	UnlockedLevel       int
	Progress            json.RawMessage
	IdempotencyReplayed bool
}

type CompleteLevelResponse struct {
	LevelID             int             `json:"level_id"`
	Stars               int             `json:"stars"`
	BestScore           int             `json:"best_score"`
	RewardCoins         int             `json:"reward_coins"`
	Coins               int             `json:"coins"`
	UnlockedLevel       int             `json:"unlocked_level"`
	Progress            json.RawMessage `json:"progress,omitempty"`
	IdempotencyReplayed bool            `json:"idempotency_replayed"`
}

type CompleteDailyInput struct {
	IdempotencyKey string `json:"idempotency_key"`
	Score          int    `json:"score"`
}

type CompleteDailyParams struct {
	UserID         uint64
	IdempotencyKey string
	Score          int
	Questions      int
	ElapsedMS      int
	FastestMS      int
	Mistakes       int
	Hints          int
	BestCombo      int
	Operators      []string
}

type CompleteDailyResult struct {
	DateKey             string
	Score               int
	BestScore           int
	Streak              int
	RewardCoins         int
	Coins               int
	Progress            json.RawMessage
	IdempotencyReplayed bool
}

type CompleteDailyResponse struct {
	DateKey             string          `json:"date_key"`
	Score               int             `json:"score"`
	BestScore           int             `json:"best_score"`
	Streak              int             `json:"streak"`
	RewardCoins         int             `json:"reward_coins"`
	Coins               int             `json:"coins"`
	Progress            json.RawMessage `json:"progress,omitempty"`
	IdempotencyReplayed bool            `json:"idempotency_replayed"`
}

// CompletionMetrics are produced by a server-validated run. They are kept
// out of the public legacy completion request so a client cannot forge task,
// achievement, or statistics progress by posting arbitrary metrics.
type CompletionMetrics struct {
	Questions int
	ElapsedMS int
	FastestMS int
	Mistakes  int
	Hints     int
	BestCombo int
	Operators []string
}
