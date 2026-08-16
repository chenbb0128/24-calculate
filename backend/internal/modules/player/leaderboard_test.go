package player

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/example/go-service/internal/modules/user"
	db "github.com/example/go-service/internal/store/sqlc"
)

type leaderboardProfileReader struct {
	profile user.ProfileResponse
}

func (f leaderboardProfileReader) GetProfile(context.Context, uint64) (user.ProfileResponse, error) {
	return f.profile, nil
}

type leaderboardStore struct {
	campaignRows []db.CampaignLeaderboardRow
	dailyRows    []db.DailyLeaderboardRow
	dateKey      string
}

func (f *leaderboardStore) GetPlayerProfile(context.Context, uint64) (db.PlayerProfile, error) {
	return db.PlayerProfile{}, nil
}

func (f *leaderboardStore) CreatePlayerProfile(context.Context, db.CreatePlayerProfileParams) error {
	return nil
}

func (f *leaderboardStore) MutatePlayerProgress(context.Context, uint64, ProgressMutation) (json.RawMessage, error) {
	return json.RawMessage(DefaultProgressJSON), nil
}

func (f *leaderboardStore) GetLeaderboardSubmissionByKey(context.Context, db.GetLeaderboardSubmissionByKeyParams) (db.PlayerLeaderboardSubmission, error) {
	return db.PlayerLeaderboardSubmission{}, sql.ErrNoRows
}

func (f *leaderboardStore) CreateLeaderboardSubmission(context.Context, db.CreateLeaderboardSubmissionParams) error {
	return nil
}

func (f *leaderboardStore) ListEndlessLeaderboard(context.Context) ([]db.LeaderboardScoreRow, error) {
	return nil, nil
}

func (f *leaderboardStore) ListFriendLeaderboard(context.Context) ([]db.LeaderboardScoreRow, error) {
	return nil, nil
}

func (f *leaderboardStore) ListFriendUserIDs(_ context.Context, userID uint64) ([]uint64, error) {
	return []uint64{userID}, nil
}

func (f *leaderboardStore) ListCampaignLeaderboard(context.Context) ([]db.CampaignLeaderboardRow, error) {
	return f.campaignRows, nil
}

func (f *leaderboardStore) ListDailyLeaderboard(_ context.Context, dateKey string) ([]db.DailyLeaderboardRow, error) {
	f.dateKey = dateKey
	return f.dailyRows, nil
}

func (f *leaderboardStore) CompleteLevel(context.Context, CompleteLevelParams) (CompleteLevelResult, error) {
	return CompleteLevelResult{}, nil
}

func (f *leaderboardStore) CompleteDaily(context.Context, CompleteDailyParams) (CompleteDailyResult, error) {
	return CompleteDailyResult{}, nil
}

func TestLeaderboardSortsAndAddsCurrentPlayer(t *testing.T) {
	store := &leaderboardStore{
		campaignRows: []db.CampaignLeaderboardRow{
			{UserID: 8, Nickname: "top", Score: 100},
			{UserID: 3, Nickname: "me", Score: 40},
		},
	}
	service := NewService(leaderboardProfileReader{profile: user.ProfileResponse{ID: 3, Nickname: "me"}}, store)

	result, err := service.Leaderboard(context.Background(), 3, LeaderboardCampaign)
	if err != nil {
		t.Fatalf("Leaderboard() error = %v", err)
	}
	if result.MyRank != 2 || result.MyScore != 40 {
		t.Fatalf("current player = rank %d score %d, want rank 2 score 40", result.MyRank, result.MyScore)
	}
	if len(result.Entries) != 2 || result.Entries[0].UserID != 8 || result.Entries[0].Rank != 1 {
		t.Fatalf("entries = %#v, want sorted top entry", result.Entries)
	}
}

func TestDailyLeaderboardAddsZeroScoreForNewPlayer(t *testing.T) {
	store := &leaderboardStore{}
	service := NewService(leaderboardProfileReader{profile: user.ProfileResponse{ID: 3, Nickname: "new"}}, store)

	result, err := service.Leaderboard(context.Background(), 3, LeaderboardDaily)
	if err != nil {
		t.Fatalf("Leaderboard() error = %v", err)
	}
	if result.MyRank != 1 || result.MyScore != 0 || len(result.Entries) != 1 {
		t.Fatalf("result = %#v, want one zero-score current-player entry", result)
	}
	if store.dateKey == "" {
		t.Fatal("daily leaderboard date key was not passed to store")
	}
}

func TestAllLeaderboardModesAreSupported(t *testing.T) {
	service := NewService(leaderboardProfileReader{profile: user.ProfileResponse{ID: 3}}, &leaderboardStore{})
	for _, mode := range []string{LeaderboardCampaign, LeaderboardDaily, LeaderboardEndless, LeaderboardFriend} {
		result, err := service.LeaderboardScoped(context.Background(), 3, mode, LeaderboardGlobal)
		if err != nil {
			t.Fatalf("LeaderboardScoped(%q) error = %v", mode, err)
		}
		if result.Mode != mode || result.Scope != LeaderboardGlobal || result.MyRank != 1 {
			t.Fatalf("result for %q = %#v", mode, result)
		}
	}
}
