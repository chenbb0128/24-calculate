package player

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestDeriveRankVisibleMatchesClientThresholds(t *testing.T) {
	tests := []struct {
		rating   int
		tier     string
		division int
		stars    int
	}{
		{rating: 0, tier: RankTierBronze, division: 3, stars: 0},
		{rating: 100, tier: RankTierBronze, division: 3, stars: 1},
		{rating: 1000, tier: RankTierBronze, division: 1, stars: 3},
		{rating: 1025, tier: RankTierBronze, division: 1, stars: 4},
		{rating: 1100, tier: RankTierSilver, division: 3, stars: 0},
		{rating: 1300, tier: RankTierGold, division: 3, stars: 0},
		{rating: 2100, tier: RankTierKing, division: 3, stars: 0},
	}
	for _, test := range tests {
		got := deriveRankVisible(test.rating)
		if got.Tier != test.tier || got.Division != test.division || got.Stars != test.stars {
			t.Fatalf("rating %d => %+v, want tier=%s division=%d stars=%d", test.rating, got, test.tier, test.division, test.stars)
		}
	}
}

func TestNormalizeRankProfileKeepsNewPlayerPlacementDefaults(t *testing.T) {
	got := normalizeRankProfile(RankProfile{
		SeasonID: "2026-S3",
		Rating:   RankDefault,
		Tier:     RankTierBronze,
		Division: 3,
		Stars:    0,
	})
	if got.Rating != RankDefault || got.Tier != RankTierBronze || got.Division != 3 || got.Stars != 0 {
		t.Fatalf("new-player rank defaults = %+v, want rating 1000 bronze III 0 stars", got)
	}
}

func TestRankSeasonIDUsesShanghaiQuarter(t *testing.T) {
	zone := time.FixedZone("Asia/Shanghai", 8*60*60)
	if got := rankSeasonID(time.Date(2026, time.August, 18, 12, 0, 0, 0, zone)); got != "2026-S3" {
		t.Fatalf("rankSeasonID() = %q, want 2026-S3", got)
	}
}

func TestProgressWithRankAddsServerSnapshot(t *testing.T) {
	progress, err := progressWithRank(DefaultProgressJSON, RankView{
		SeasonID: "2026-S3", Rating: 1025, Tier: RankTierBronze, Division: 3, Stars: 3,
	})
	if err != nil {
		t.Fatalf("progressWithRank() error = %v", err)
	}
	var state map[string]any
	if err := json.Unmarshal([]byte(progress), &state); err != nil {
		t.Fatalf("decode progress: %v", err)
	}
	rank, ok := state["rank"].(map[string]any)
	if !ok || rank["season_id"] != "2026-S3" || int(rank["rating"].(float64)) != 1025 {
		t.Fatalf("rank snapshot = %#v", state["rank"])
	}
}

func TestValidateRankedMatchSettlementRejectsClientInconsistency(t *testing.T) {
	base := RankedMatchSettlement{
		MatchID: "friend-123456", SeasonID: "2026-S3",
		Players: []RankMatchPlayer{
			{UserID: 1, Outcome: "win", IdempotencyKey: "idempotency-1"},
			{UserID: 2, Outcome: "win", IdempotencyKey: "idempotency-2"},
		},
	}
	if err := validateRankedMatchSettlement(base); err == nil {
		t.Fatal("validateRankedMatchSettlement() error = nil for inconsistent outcomes")
	}
}

func TestValidateRankedMatchSettlementAllowsServerGeneratedBotResult(t *testing.T) {
	settlement := RankedMatchSettlement{
		MatchID: "match-bot", SeasonID: "2026-S3",
		Players: []RankMatchPlayer{{UserID: 7, Outcome: "win", IdempotencyKey: "human-result"}},
	}
	if err := validateRankedMatchSettlement(settlement); err != nil {
		t.Fatalf("server-generated bot settlement rejected: %v", err)
	}
}

type rankStoreFake struct {
	profile        RankProfile
	result         map[uint64]RankSettlementResult
	rows           []RankLeaderboardRow
	seasonRequests []string
}

func (f *rankStoreFake) GetOrCreateRankProfile(_ context.Context, _ uint64, seasonID string) (RankProfile, error) {
	f.seasonRequests = append(f.seasonRequests, seasonID)
	profile := f.profile
	profile.SeasonID = seasonID
	return profile, nil
}

func (f *rankStoreFake) SettleRankedMatch(context.Context, RankedMatchSettlement) (map[uint64]RankSettlementResult, error) {
	return f.result, nil
}

func (f *rankStoreFake) ListRankLeaderboard(context.Context, string) ([]RankLeaderboardRow, error) {
	return f.rows, nil
}

func TestGetRankUsesConfiguredRankStore(t *testing.T) {
	service := NewService(nil, nil)
	service.SetRankStore(&rankStoreFake{profile: RankProfile{
		UserID: 3, SeasonID: "2026-S3", Rating: 1000, Tier: RankTierBronze,
		Division: 3, Stars: 0, BestTier: RankTierBronze,
	}})
	got, err := service.GetRank(context.Background(), 3)
	if err != nil {
		t.Fatalf("GetRank() error = %v", err)
	}
	if got.SeasonID != "2026-S3" || got.Rating != 1000 || got.Tier != RankTierBronze {
		t.Fatalf("GetRank() = %+v", got)
	}
}
