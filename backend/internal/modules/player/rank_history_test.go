package player

import (
	"context"
	"testing"
)

type rankHistoryFake struct {
	profiles map[uint64]RankProfile
	rows     map[uint64][]RankedMatchRecord
}

func (f *rankHistoryFake) GetOrCreateRankProfile(_ context.Context, userID uint64, seasonID string) (RankProfile, error) {
	profile := f.profiles[userID]
	profile.UserID = userID
	profile.SeasonID = seasonID
	return profile, nil
}

func (*rankHistoryFake) SettleRankedMatch(context.Context, RankedMatchSettlement) (map[uint64]RankSettlementResult, error) {
	return nil, nil
}

func (*rankHistoryFake) ListRankLeaderboard(context.Context, string) ([]RankLeaderboardRow, error) {
	return nil, nil
}

func (f *rankHistoryFake) GetRankedSummary(_ context.Context, userID uint64, seasonID string) (RankedSummary, error) {
	profile := f.profiles[userID]
	return RankedSummary{SeasonID: seasonID, Rating: profile.Rating, RankedMatches: profile.RankedMatches, Wins: profile.Wins}, nil
}

func (f *rankHistoryFake) ListRankedMatches(_ context.Context, userID uint64, _ string, _ *RankHistoryCursor, _ int) (RankedMatchPage, error) {
	return RankedMatchPage{Matches: f.rows[userID]}, nil
}

func (f *rankHistoryFake) GetRankedMatch(_ context.Context, userID uint64, matchID string) (RankedMatchRecord, error) {
	for _, item := range f.rows[userID] {
		if item.MatchID == matchID {
			return item, nil
		}
	}
	return RankedMatchRecord{}, context.Canceled
}

func TestRankedHistoryIsScopedToAuthenticatedUser(t *testing.T) {
	fake := &rankHistoryFake{
		profiles: map[uint64]RankProfile{1: {Rating: 1200, RankedMatches: 1, Wins: 1}, 2: {Rating: 900, RankedMatches: 1}},
		rows: map[uint64][]RankedMatchRecord{
			1: {{MatchID: "m-user-1", Mode: "ranked"}},
			2: {{MatchID: "m-user-2", Mode: "ranked"}},
		},
	}
	service := NewService(nil, nil)
	service.SetRankStore(fake)

	first, err := service.ListRankedMatches(context.Background(), 1, "2026-S3", "", 20)
	if err != nil || len(first.Matches) != 1 || first.Matches[0].MatchID != "m-user-1" {
		t.Fatalf("user 1 history = %+v, error = %v", first, err)
	}
	second, err := service.ListRankedMatches(context.Background(), 2, "2026-S3", "", 20)
	if err != nil || len(second.Matches) != 1 || second.Matches[0].MatchID != "m-user-2" {
		t.Fatalf("user 2 history = %+v, error = %v", second, err)
	}
}

func TestRankedHistoryRejectsForgedCursor(t *testing.T) {
	service := NewService(nil, nil)
	service.SetRankStore(&rankHistoryFake{})
	if _, err := service.ListRankedMatches(context.Background(), 1, "2026-S3", "not-a-cursor", 20); err == nil {
		t.Fatal("ListRankedMatches() accepted a forged cursor")
	}
}
