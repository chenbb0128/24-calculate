package player

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/example/go-service/internal/modules/user"
	db "github.com/example/go-service/internal/store/sqlc"
)

type bootstrapProgressStore struct {
	*leaderboardStore
	state map[string]any
}

func newBootstrapProgressStore(t *testing.T) *bootstrapProgressStore {
	t.Helper()
	state := map[string]any{}
	if err := json.Unmarshal([]byte(DefaultProgressJSON), &state); err != nil {
		t.Fatal(err)
	}
	return &bootstrapProgressStore{leaderboardStore: &leaderboardStore{}, state: state}
}

func (s *bootstrapProgressStore) GetPlayerProfile(context.Context, uint64) (db.PlayerProfile, error) {
	encoded, err := json.Marshal(s.state)
	if err != nil {
		return db.PlayerProfile{}, err
	}
	return db.PlayerProfile{ProgressJSON: string(encoded)}, nil
}

func (s *bootstrapProgressStore) MutatePlayerProgress(_ context.Context, _ uint64, mutate ProgressMutation) (json.RawMessage, error) {
	if err := mutate(s.state); err != nil {
		return nil, err
	}
	return json.Marshal(s.state)
}

type isolatedProgressStore struct {
	*leaderboardStore
	states map[uint64]map[string]any
}

func newIsolatedProgressStore(t *testing.T) *isolatedProgressStore {
	t.Helper()
	return &isolatedProgressStore{leaderboardStore: &leaderboardStore{}, states: map[uint64]map[string]any{}}
}

func (s *isolatedProgressStore) GetPlayerProfile(_ context.Context, userID uint64) (db.PlayerProfile, error) {
	state, ok := s.states[userID]
	if !ok {
		return db.PlayerProfile{}, sql.ErrNoRows
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		return db.PlayerProfile{}, err
	}
	return db.PlayerProfile{UserID: userID, ProgressJSON: string(encoded)}, nil
}

func (s *isolatedProgressStore) CreatePlayerProfile(_ context.Context, arg db.CreatePlayerProfileParams) error {
	if _, exists := s.states[arg.UserID]; exists {
		return sql.ErrNoRows
	}
	state := map[string]any{}
	if err := json.Unmarshal([]byte(arg.ProgressJSON), &state); err != nil {
		return err
	}
	s.states[arg.UserID] = state
	return nil
}

func (s *isolatedProgressStore) MutatePlayerProgress(_ context.Context, userID uint64, mutate ProgressMutation) (json.RawMessage, error) {
	state, ok := s.states[userID]
	if !ok {
		state = map[string]any{}
		if err := json.Unmarshal([]byte(DefaultProgressJSON), &state); err != nil {
			return nil, err
		}
		s.states[userID] = state
	}
	if err := mutate(state); err != nil {
		return nil, err
	}
	return json.Marshal(state)
}

func TestPlayerProgressIsolatedByBackendUserID(t *testing.T) {
	store := newIsolatedProgressStore(t)
	service := NewService(leaderboardProfileReader{profile: user.ProfileResponse{Nickname: "玩家"}}, store)

	if _, err := service.Bootstrap(context.Background(), 3); err != nil {
		t.Fatalf("Bootstrap(user 3) error = %v", err)
	}
	if _, err := store.MutatePlayerProgress(context.Background(), 3, func(state map[string]any) error {
		state["coins"] = 777
		return nil
	}); err != nil {
		t.Fatalf("mutate user 3 error = %v", err)
	}
	if _, err := service.Bootstrap(context.Background(), 4); err != nil {
		t.Fatalf("Bootstrap(user 4) error = %v", err)
	}

	user3, _ := store.GetPlayerProfile(context.Background(), 3)
	user4, _ := store.GetPlayerProfile(context.Background(), 4)
	if progressCoins(user3.ProgressJSON) != 777 {
		t.Fatalf("user 3 coins = %d, want 777", progressCoins(user3.ProgressJSON))
	}
	if progressCoins(user4.ProgressJSON) == 777 {
		t.Fatalf("user 4 inherited user 3 progress: %s", user4.ProgressJSON)
	}
}

func TestBootstrapAwardsLoginRewardOnlyOncePerShanghaiDate(t *testing.T) {
	store := newBootstrapProgressStore(t)
	service := NewService(leaderboardProfileReader{profile: user.ProfileResponse{ID: 3}}, store)

	first, err := service.Bootstrap(context.Background(), 3)
	if err != nil {
		t.Fatalf("first Bootstrap() error = %v", err)
	}
	second, err := service.Bootstrap(context.Background(), 3)
	if err != nil {
		t.Fatalf("second Bootstrap() error = %v", err)
	}
	if first.LoginReward != 5 || second.LoginReward != 0 {
		t.Fatalf("login rewards = %d, %d; want 5, 0", first.LoginReward, second.LoginReward)
	}
	if readInt(store.state["coins"]) != 5 {
		t.Fatalf("coins = %v, want one reward", store.state["coins"])
	}
	login := ensureObject(store.state, "login")
	if readInt(login["streak"]) != 1 || login["last_reward"] != 5 {
		t.Fatalf("login state = %#v, want first-day reward", login)
	}
}

func TestBootstrapContinuesLoginStreakAndCapsAtSevenDays(t *testing.T) {
	store := newBootstrapProgressStore(t)
	today := time.Now().In(shanghaiLocation).Format("2006-01-02")
	login := ensureObject(store.state, "login")
	login["last_date"] = previousShanghaiDate(today)
	login["streak"] = 6
	service := NewService(leaderboardProfileReader{profile: user.ProfileResponse{ID: 3}}, store)

	result, err := service.Bootstrap(context.Background(), 3)
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	if result.LoginReward != 27 || readInt(login["streak"]) != 7 {
		t.Fatalf("result = %#v, login = %#v; want day-seven reward and streak", result, login)
	}
}

func TestRecordServerSolveStatsTracksModesOperatorsAndFastestSolve(t *testing.T) {
	state := map[string]any{}
	if err := json.Unmarshal([]byte(DefaultProgressJSON), &state); err != nil {
		t.Fatal(err)
	}
	recordServerSolveStats(state, "campaign", 3, 100, 3600, 900, 3, 21, []string{"+", "+", "-"})
	recordServerSolveStats(state, "daily", 3, 200, 2400, 700, 4, -1, []string{"*"})

	stats := ensureObject(state, "player_stats")
	if readInt(stats["total_solved"]) != 6 || readInt(stats["total_score"]) != 300 || readInt(stats["fastest_ms"]) != 700 {
		t.Fatalf("stats totals = %#v", stats)
	}
	if readInt(stats["best_level"]) != 22 || readInt(stats["best_chapter"]) != 2 || readInt(stats["best_combo"]) != 4 {
		t.Fatalf("stats milestones = %#v", stats)
	}
	operators := ensureObject(stats, "operator_counts")
	if readInt(operators["+"]) != 2 || readInt(operators["-"]) != 1 || readInt(operators["*"]) != 1 {
		t.Fatalf("operator counts = %#v", operators)
	}
}
