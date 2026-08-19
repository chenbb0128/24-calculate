package player

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/example/go-service/internal/apperror"
)

const (
	RankTierBronze    = "bronze"
	RankTierSilver    = "silver"
	RankTierGold      = "gold"
	RankTierPlatinum  = "platinum"
	RankTierDiamond   = "diamond"
	RankTierMaster    = "master"
	RankTierKing      = "king"
	RankDefault       = 1000
	RankMaxRating     = 9999
	RankStarsPerDiv   = 5
	RankDivisionCount = 3
	RankPlacementMax  = 5
)

var rankSeasonPattern = regexp.MustCompile(`^\d{4}-S[1-4]$`)

type RankProfile struct {
	UserID           uint64
	SeasonID         string
	Rating           int
	Tier             string
	Division         int
	Stars            int
	PlacementMatches int
	RankedMatches    int
	Wins             int
	Losses           int
	Draws            int
	BestTier         string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type RankView struct {
	SeasonID         string    `json:"season_id"`
	Rating           int       `json:"rating"`
	Tier             string    `json:"tier"`
	Division         int       `json:"division"`
	Stars            int       `json:"stars"`
	PlacementMatches int       `json:"placement_matches"`
	RankedMatches    int       `json:"ranked_matches"`
	Wins             int       `json:"wins"`
	Losses           int       `json:"losses"`
	Draws            int       `json:"draws"`
	BestTier         string    `json:"best_tier"`
	UpdatedAt        time.Time `json:"updated_at,omitempty"`
}

type RankResult struct {
	Eligible         bool   `json:"eligible"`
	Reason           string `json:"reason,omitempty"`
	SeasonID         string `json:"season_id,omitempty"`
	MatchID          string `json:"match_id,omitempty"`
	Outcome          string `json:"outcome,omitempty"`
	Rating           int    `json:"rating"`
	Tier             string `json:"tier"`
	Division         int    `json:"division"`
	Stars            int    `json:"stars"`
	BestTier         string `json:"best_tier,omitempty"`
	RatingBefore     int    `json:"rating_before,omitempty"`
	RatingDelta      int    `json:"rating_delta,omitempty"`
	RatingAfter      int    `json:"rating_after,omitempty"`
	TierBefore       string `json:"tier_before,omitempty"`
	TierAfter        string `json:"tier_after,omitempty"`
	DivisionBefore   int    `json:"division_before,omitempty"`
	DivisionAfter    int    `json:"division_after,omitempty"`
	StarsBefore      int    `json:"stars_before,omitempty"`
	StarsAfter       int    `json:"stars_after,omitempty"`
	PlacementMatches int    `json:"placement_matches,omitempty"`
	RankedMatches    int    `json:"ranked_matches,omitempty"`
	Wins             int    `json:"wins,omitempty"`
	Losses           int    `json:"losses,omitempty"`
	Draws            int    `json:"draws,omitempty"`
}

type RankSettlementResult struct {
	Profile RankProfile
	Result  RankResult
}

type RankMatchPlayer struct {
	UserID         uint64
	OpponentUserID uint64
	Outcome        string
	IdempotencyKey string
	Solved         int
	QuestionCount  int
	ElapsedMS      int
	Mistakes       int
}

type RankedMatchSettlement struct {
	MatchID  string
	SeasonID string
	Players  []RankMatchPlayer
}

type RankLeaderboardRow struct {
	UserID        uint64
	Nickname      string
	Avatar        string
	SeasonID      string
	Rating        int
	Tier          string
	Division      int
	Stars         int
	RankedMatches int
	Wins          int
	UpdatedAt     time.Time
}

type RankedSummary struct {
	SeasonID      string  `json:"season_id"`
	Label         string  `json:"label"`
	Rating        int     `json:"rating"`
	StarsLabel    string  `json:"stars_label"`
	RankedMatches int     `json:"ranked_matches"`
	Wins          int     `json:"wins"`
	Losses        int     `json:"losses"`
	Draws         int     `json:"draws"`
	WinRate       float64 `json:"win_rate"`
	CurrentStreak int     `json:"current_streak"`
	BestStreak    int     `json:"best_streak"`
}

type RankedMatchRecord struct {
	MatchID       string    `json:"match_id"`
	Mode          string    `json:"mode"`
	Outcome       string    `json:"outcome"`
	OpponentName  string    `json:"opponent_name"`
	Solved        int       `json:"solved"`
	QuestionCount int       `json:"question_count"`
	ElapsedMS     int       `json:"elapsed_ms"`
	Mistakes      int       `json:"mistakes"`
	RatingDelta   int       `json:"rating_delta"`
	CreatedAt     time.Time `json:"created_at"`
}

type RankedMatchPage struct {
	Matches    []RankedMatchRecord `json:"matches"`
	NextCursor string              `json:"next_cursor,omitempty"`
}

type RankHistoryCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uint64    `json:"id"`
}

func (s *Service) GetRank(ctx context.Context, userID uint64) (RankView, error) {
	if s == nil || s.rankStore == nil {
		return RankView{}, apperror.ServiceUnavailable("段位服务暂不可用", nil)
	}
	profile, err := s.rankStore.GetOrCreateRankProfile(ctx, userID, s.currentRankSeasonID())
	if err != nil {
		return RankView{}, err
	}
	return rankView(profile), nil
}

// RankStore is intentionally optional so existing in-memory stores used by
// older gameplay tests do not need to know about the MySQL rank tables.
type RankStore interface {
	GetOrCreateRankProfile(context.Context, uint64, string) (RankProfile, error)
	SettleRankedMatch(context.Context, RankedMatchSettlement) (map[uint64]RankSettlementResult, error)
	ListRankLeaderboard(context.Context, string) ([]RankLeaderboardRow, error)
}

type RankHistoryStore interface {
	GetRankedSummary(context.Context, uint64, string) (RankedSummary, error)
	ListRankedMatches(context.Context, uint64, string, *RankHistoryCursor, int) (RankedMatchPage, error)
	GetRankedMatch(context.Context, uint64, string) (RankedMatchRecord, error)
}

type SQLRankRepository struct {
	db *sql.DB
}

func NewSQLRankRepository(database *sql.DB) *SQLRankRepository {
	return &SQLRankRepository{db: database}
}

func (r *SQLRankRepository) GetOrCreateRankProfile(ctx context.Context, userID uint64, seasonID string) (RankProfile, error) {
	if r == nil || r.db == nil {
		return RankProfile{}, fmt.Errorf("rank repository database is not initialized")
	}
	seasonID, err := normalizeRankSeasonID(seasonID)
	if err != nil {
		return RankProfile{}, err
	}
	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `
INSERT IGNORE INTO player_rank_profiles
    (user_id, season_id, rating, tier, division, stars, placement_matches,
     ranked_matches, wins, losses, draws, best_tier, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, 0, 0, 0, 0, 0, ?, ?, ?)`,
		userID, seasonID, RankDefault, RankTierBronze, 3, 0, RankTierBronze, now, now)
	if err != nil {
		return RankProfile{}, fmt.Errorf("create rank profile: %w", err)
	}
	return r.getRankProfile(ctx, r.db, userID, seasonID, false)
}

func (r *SQLRankRepository) getRankProfile(ctx context.Context, query interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, userID uint64, seasonID string, forUpdate bool) (RankProfile, error) {
	lock := ""
	if forUpdate {
		lock = " FOR UPDATE"
	}
	row := query.QueryRowContext(ctx, `
SELECT user_id, season_id, rating, tier, division, stars, placement_matches,
       ranked_matches, wins, losses, draws, best_tier, created_at, updated_at
FROM player_rank_profiles
WHERE user_id = ? AND season_id = ?`+lock, userID, seasonID)
	var profile RankProfile
	if err := row.Scan(
		&profile.UserID, &profile.SeasonID, &profile.Rating, &profile.Tier,
		&profile.Division, &profile.Stars, &profile.PlacementMatches,
		&profile.RankedMatches, &profile.Wins, &profile.Losses, &profile.Draws,
		&profile.BestTier, &profile.CreatedAt, &profile.UpdatedAt,
	); err != nil {
		return RankProfile{}, err
	}
	return normalizeRankProfile(profile), nil
}

func (r *SQLRankRepository) SettleRankedMatch(ctx context.Context, settlement RankedMatchSettlement) (map[uint64]RankSettlementResult, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("rank repository database is not initialized")
	}
	if err := validateRankedMatchSettlement(settlement); err != nil {
		return nil, err
	}
	seasonID, err := normalizeRankSeasonID(settlement.SeasonID)
	if err != nil {
		return nil, err
	}
	players := append([]RankMatchPlayer(nil), settlement.Players...)
	sort.Slice(players, func(i, j int) bool { return players[i].UserID < players[j].UserID })

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin rank settlement transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	existing, err := queryRankedMatchResults(ctx, tx, settlement.MatchID, seasonID)
	if err != nil {
		return nil, err
	}
	if len(existing) == len(players) {
		for _, player := range players {
			profile, profileErr := r.getRankProfile(ctx, tx, player.UserID, seasonID, false)
			if profileErr != nil {
				return nil, fmt.Errorf("load replayed rank profile: %w", profileErr)
			}
			entry := existing[player.UserID]
			// Profile refreshes progress with the current server state. Result
			// remains the immutable first settlement stored for this match.
			entry.Profile = profile
			existing[player.UserID] = entry
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit replayed rank settlement: %w", err)
		}
		return existing, nil
	}
	if len(existing) != 0 {
		return nil, fmt.Errorf("ranked match settlement is partially recorded")
	}

	profiles := make(map[uint64]RankProfile, len(players))
	for _, player := range players {
		if _, err := tx.ExecContext(ctx, `
INSERT IGNORE INTO player_rank_profiles
    (user_id, season_id, rating, tier, division, stars, placement_matches,
     ranked_matches, wins, losses, draws, best_tier, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, 0, 0, 0, 0, 0, ?, ?, ?)`,
			player.UserID, seasonID, RankDefault, RankTierBronze, 3, 0, RankTierBronze, time.Now().UTC(), time.Now().UTC()); err != nil {
			return nil, fmt.Errorf("ensure rank profile: %w", err)
		}
		profile, err := r.getRankProfile(ctx, tx, player.UserID, seasonID, true)
		if err != nil {
			return nil, fmt.Errorf("lock rank profile: %w", err)
		}
		profiles[player.UserID] = profile
	}

	results := make(map[uint64]RankSettlementResult, len(players))
	for _, player := range players {
		profile := profiles[player.UserID]
		before := rankView(profile)
		delta := rankDeltaForOutcome(player.Outcome)
		next := profile
		next.Rating = clampRankRating(profile.Rating + delta)
		next.PlacementMatches = minRankInt(RankPlacementMax, profile.PlacementMatches+1)
		next.RankedMatches++
		switch player.Outcome {
		case "win":
			next.Wins++
		case "lose":
			next.Losses++
		case "draw":
			next.Draws++
		}
		visible := deriveRankVisible(next.Rating)
		next.Tier, next.Division, next.Stars = visible.Tier, visible.Division, visible.Stars
		if rankTierIndex(next.Tier) > rankTierIndex(profile.BestTier) {
			next.BestTier = next.Tier
		}
		next.UpdatedAt = time.Now().UTC()
		if _, err := tx.ExecContext(ctx, `
UPDATE player_rank_profiles
SET rating = ?, tier = ?, division = ?, stars = ?, placement_matches = ?,
    ranked_matches = ?, wins = ?, losses = ?, draws = ?, best_tier = ?, updated_at = ?
WHERE user_id = ? AND season_id = ?`,
			next.Rating, next.Tier, next.Division, next.Stars, next.PlacementMatches,
			next.RankedMatches, next.Wins, next.Losses, next.Draws, next.BestTier,
			next.UpdatedAt, next.UserID, seasonID); err != nil {
			return nil, fmt.Errorf("update rank profile: %w", err)
		}
		after := rankView(next)
		if _, err := tx.ExecContext(ctx, `
INSERT INTO ranked_match_results
    (match_id, user_id, opponent_user_id, season_id, outcome, solved, question_count,
     elapsed_ms, mistakes, rating_before, rating_delta, rating_after,
     tier_before, tier_after, division_before, division_after,
     stars_before, stars_after, placement_matches, ranked_matches, wins,
     losses, draws, best_tier, idempotency_key, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			settlement.MatchID, player.UserID, nullableUserID(player.OpponentUserID), seasonID, player.Outcome,
			player.Solved, player.QuestionCount, player.ElapsedMS, player.Mistakes, profile.Rating,
			delta, next.Rating, before.Tier, after.Tier, before.Division, after.Division,
			before.Stars, after.Stars, next.PlacementMatches, next.RankedMatches,
			next.Wins, next.Losses, next.Draws, next.BestTier, player.IdempotencyKey,
			next.UpdatedAt); err != nil {
			return nil, fmt.Errorf("record rank result: %w", err)
		}
		results[player.UserID] = RankSettlementResult{Profile: next, Result: RankResult{
			Eligible: true, SeasonID: seasonID, MatchID: settlement.MatchID, Outcome: player.Outcome,
			Rating: next.Rating, Tier: next.Tier, Division: next.Division, Stars: next.Stars, BestTier: next.BestTier,
			RatingBefore: profile.Rating, RatingDelta: delta, RatingAfter: next.Rating,
			TierBefore: before.Tier, TierAfter: after.Tier,
			DivisionBefore: before.Division, DivisionAfter: after.Division,
			StarsBefore: before.Stars, StarsAfter: after.Stars,
			PlacementMatches: next.PlacementMatches, RankedMatches: next.RankedMatches,
			Wins: next.Wins, Losses: next.Losses, Draws: next.Draws,
		}}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit rank settlement: %w", err)
	}
	return results, nil
}

func queryRankedMatchResults(ctx context.Context, query interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, matchID, seasonID string) (map[uint64]RankSettlementResult, error) {
	rows, err := query.QueryContext(ctx, `
SELECT user_id, outcome, rating_before, rating_delta, rating_after,
       tier_before, tier_after, division_before, division_after,
       stars_before, stars_after, placement_matches, ranked_matches,
       wins, losses, draws, best_tier, created_at
FROM ranked_match_results
WHERE match_id = ? AND season_id = ?
ORDER BY user_id`, matchID, seasonID)
	if err != nil {
		return nil, fmt.Errorf("query rank settlement: %w", err)
	}
	defer rows.Close()
	result := make(map[uint64]RankSettlementResult)
	for rows.Next() {
		var userID uint64
		var outcome string
		var before, delta, after int
		var tierBefore, tierAfter, bestTier string
		var divisionBefore, divisionAfter, starsBefore, starsAfter int
		var placementMatches, rankedMatches, wins, losses, draws int
		var createdAt time.Time
		if err := rows.Scan(&userID, &outcome, &before, &delta, &after,
			&tierBefore, &tierAfter, &divisionBefore, &divisionAfter,
			&starsBefore, &starsAfter, &placementMatches, &rankedMatches,
			&wins, &losses, &draws, &bestTier, &createdAt); err != nil {
			return nil, fmt.Errorf("scan rank settlement: %w", err)
		}
		result[userID] = RankSettlementResult{Result: RankResult{
			Eligible: true, SeasonID: seasonID, MatchID: matchID, Outcome: outcome,
			Rating: after, Tier: tierAfter, Division: divisionAfter, Stars: starsAfter,
			BestTier: bestTier, RatingBefore: before, RatingDelta: delta, RatingAfter: after,
			TierBefore: tierBefore, TierAfter: tierAfter,
			DivisionBefore: divisionBefore, DivisionAfter: divisionAfter,
			StarsBefore: starsBefore, StarsAfter: starsAfter,
			PlacementMatches: placementMatches, RankedMatches: rankedMatches,
			Wins: wins, Losses: losses, Draws: draws,
		}}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rank settlement: %w", err)
	}
	return result, nil
}

func (r *SQLRankRepository) ListRankLeaderboard(ctx context.Context, seasonID string) ([]RankLeaderboardRow, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("rank repository database is not initialized")
	}
	seasonID, err := normalizeRankSeasonID(seasonID)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT p.user_id, u.nickname, u.avatar, p.season_id, p.rating, p.tier,
       p.division, p.stars, p.ranked_matches, p.wins, p.updated_at
FROM player_rank_profiles p
INNER JOIN users u ON u.id = p.user_id
WHERE p.season_id = ? AND u.status = 1
ORDER BY p.rating DESC, p.wins DESC, p.ranked_matches DESC, p.user_id ASC`, seasonID)
	if err != nil {
		return nil, fmt.Errorf("list rank leaderboard: %w", err)
	}
	defer rows.Close()
	result := make([]RankLeaderboardRow, 0)
	for rows.Next() {
		var item RankLeaderboardRow
		if err := rows.Scan(&item.UserID, &item.Nickname, &item.Avatar, &item.SeasonID, &item.Rating,
			&item.Tier, &item.Division, &item.Stars, &item.RankedMatches, &item.Wins, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan rank leaderboard: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rank leaderboard: %w", err)
	}
	return result, nil
}

func (s *Service) settleRankedFriendMatch(ctx context.Context, userID uint64, room FriendRoom, submissions map[uint64]FriendMatchSubmissionRecord) (*RankSettlementResult, error) {
	if !room.Ranked || !room.RankedEligible {
		return nil, nil
	}
	if s.rankStore == nil {
		return nil, apperror.ServiceUnavailable("排位结算服务暂不可用", nil)
	}
	if len(submissions) != 2 {
		return nil, nil
	}
	players := make([]uint64, 0, 2)
	for playerID := range submissions {
		players = append(players, playerID)
	}
	sort.Slice(players, func(i, j int) bool { return players[i] < players[j] })
	botIndex := -1
	for index, playerID := range players {
		if playerID == 0 {
			botIndex = index
		}
	}
	if botIndex >= 0 {
		humanIndex := 1 - botIndex
		humanID := players[humanIndex]
		if humanID == 0 || humanID != userID {
			return nil, nil
		}
		human, bot := submissions[humanID], submissions[0]
		outcome := compareFriendResults(human, bot)
		matchID := room.MatchID
		if strings.TrimSpace(matchID) == "" {
			matchID = room.RoomID
		}
		seasonID := room.SeasonID
		if strings.TrimSpace(seasonID) == "" {
			seasonID = s.currentRankSeasonID()
		}
		release, err := s.acquireSettlementLock(ctx, "ranked:"+matchID)
		if err != nil {
			return nil, err
		}
		defer release()
		settled, err := s.rankStore.SettleRankedMatch(ctx, RankedMatchSettlement{
			MatchID: matchID, SeasonID: seasonID,
			Players: []RankMatchPlayer{{UserID: humanID, Outcome: outcome, IdempotencyKey: human.IdempotencyKey,
				Solved: human.Solved, QuestionCount: room.Rules.QuestionCount, ElapsedMS: human.ElapsedMS, Mistakes: human.Mistakes}},
		})
		if err != nil {
			return nil, err
		}
		result, ok := settled[userID]
		if !ok {
			return nil, fmt.Errorf("rank settlement did not return current player")
		}
		return &result, nil
	}
	left, right := submissions[players[0]], submissions[players[1]]
	leftOutcome := compareFriendResults(left, right)
	rightOutcome := compareFriendResults(right, left)
	matchID := room.MatchID
	if strings.TrimSpace(matchID) == "" {
		matchID = room.RoomID
	}
	seasonID := room.SeasonID
	if strings.TrimSpace(seasonID) == "" {
		seasonID = s.currentRankSeasonID()
	}
	release, err := s.acquireSettlementLock(ctx, "ranked:"+matchID)
	if err != nil {
		return nil, err
	}
	defer release()
	settled, err := s.rankStore.SettleRankedMatch(ctx, RankedMatchSettlement{
		MatchID:  matchID,
		SeasonID: seasonID,
		Players: []RankMatchPlayer{
			{UserID: players[0], OpponentUserID: players[1], Outcome: leftOutcome, IdempotencyKey: left.IdempotencyKey,
				Solved: left.Solved, QuestionCount: room.Rules.QuestionCount, ElapsedMS: left.ElapsedMS, Mistakes: left.Mistakes},
			{UserID: players[1], OpponentUserID: players[0], Outcome: rightOutcome, IdempotencyKey: right.IdempotencyKey,
				Solved: right.Solved, QuestionCount: room.Rules.QuestionCount, ElapsedMS: right.ElapsedMS, Mistakes: right.Mistakes},
		},
	})
	if err != nil {
		return nil, err
	}
	result, ok := settled[userID]
	if !ok {
		return nil, fmt.Errorf("rank settlement did not return current player")
	}
	return &result, nil
}

type rankVisible struct {
	Tier     string
	Division int
	Stars    int
}

type rankTierDefinition struct {
	ID        string
	MinRating int
}

var rankTiers = []rankTierDefinition{
	{ID: RankTierBronze, MinRating: 0},
	{ID: RankTierSilver, MinRating: 1100},
	{ID: RankTierGold, MinRating: 1300},
	{ID: RankTierPlatinum, MinRating: 1500},
	{ID: RankTierDiamond, MinRating: 1700},
	{ID: RankTierMaster, MinRating: 1900},
	{ID: RankTierKing, MinRating: 2100},
}

func deriveRankVisible(rating int) rankVisible {
	rating = clampRankRating(rating)
	tierIndex := 0
	for index, tier := range rankTiers {
		if rating >= tier.MinRating {
			tierIndex = index
		}
	}
	tier := rankTiers[tierIndex]
	nextMin := tier.MinRating + 200
	if tierIndex+1 < len(rankTiers) {
		nextMin = rankTiers[tierIndex+1].MinRating
	}
	width := maxRankInt(1, nextMin-tier.MinRating)
	progress := maxRankInt(0, rating-tier.MinRating)
	divisionWidth := maxRankInt(1, width/RankDivisionCount)
	division := maxRankInt(1, RankDivisionCount-minRankInt(RankDivisionCount-1, progress/divisionWidth))
	divisionProgress := progress % divisionWidth
	stars := clampRankInt((divisionProgress*RankStarsPerDiv)/divisionWidth, 0, RankStarsPerDiv-1)
	return rankVisible{Tier: tier.ID, Division: division, Stars: stars}
}

func rankTierIndex(tier string) int {
	for index, item := range rankTiers {
		if item.ID == strings.ToLower(strings.TrimSpace(tier)) {
			return index
		}
	}
	return 0
}

func rankView(profile RankProfile) RankView {
	profile = normalizeRankProfile(profile)
	return RankView{SeasonID: profile.SeasonID, Rating: profile.Rating, Tier: profile.Tier,
		Division: profile.Division, Stars: profile.Stars, PlacementMatches: profile.PlacementMatches,
		RankedMatches: profile.RankedMatches, Wins: profile.Wins, Losses: profile.Losses,
		Draws: profile.Draws, BestTier: profile.BestTier, UpdatedAt: profile.UpdatedAt}
}

func progressWithRank(raw string, rank RankView) (string, error) {
	state := decodeProgress(raw)
	encoded, err := json.Marshal(rank)
	if err != nil {
		return "", fmt.Errorf("encode rank progress: %w", err)
	}
	var value map[string]any
	if err := json.Unmarshal(encoded, &value); err != nil {
		return "", fmt.Errorf("decode rank progress: %w", err)
	}
	state["rank"] = value
	result, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("encode progress with rank: %w", err)
	}
	return string(result), nil
}

func normalizeRankProfile(profile RankProfile) RankProfile {
	profile.Rating = clampRankRating(profile.Rating)
	if profile.SeasonID == "" {
		profile.SeasonID = rankSeasonID(time.Now())
	}
	visible := deriveRankVisible(profile.Rating)
	if rankTierIndex(profile.Tier) == 0 && profile.Tier != RankTierBronze {
		profile.Tier = visible.Tier
	}
	if profile.Tier == "" {
		profile.Tier = visible.Tier
	}
	profile.Division = clampRankInt(profile.Division, 1, RankDivisionCount)
	profile.Stars = clampRankInt(profile.Stars, 0, RankStarsPerDiv-1)
	profile.PlacementMatches = clampRankInt(profile.PlacementMatches, 0, RankPlacementMax)
	profile.RankedMatches = maxRankInt(0, profile.RankedMatches)
	profile.Wins = maxRankInt(0, profile.Wins)
	profile.Losses = maxRankInt(0, profile.Losses)
	profile.Draws = maxRankInt(0, profile.Draws)
	if profile.BestTier == "" {
		profile.BestTier = profile.Tier
	}
	return profile
}

func normalizeRankSeasonID(value string) (string, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if !rankSeasonPattern.MatchString(value) {
		return "", fmt.Errorf("rank season_id is invalid")
	}
	return value, nil
}

func rankSeasonID(now time.Time) string {
	local := now.In(shanghaiLocation)
	return fmt.Sprintf("%d-S%d", local.Year(), (int(local.Month())-1)/3+1)
}

func rankDeltaForOutcome(outcome string) int {
	switch outcome {
	case "win":
		return 25
	case "lose":
		return -25
	default:
		return 0
	}
}

func validateRankedMatchSettlement(settlement RankedMatchSettlement) error {
	if strings.TrimSpace(settlement.MatchID) == "" || len(settlement.MatchID) > 128 {
		return fmt.Errorf("ranked match_id is invalid")
	}
	if _, err := normalizeRankSeasonID(settlement.SeasonID); err != nil {
		return err
	}
	if len(settlement.Players) != 1 && len(settlement.Players) != 2 {
		return fmt.Errorf("ranked match must contain one or two players")
	}
	if settlement.Players[0].UserID == 0 || (len(settlement.Players) == 2 &&
		(settlement.Players[1].UserID == 0 || settlement.Players[0].UserID == settlement.Players[1].UserID)) {
		return fmt.Errorf("ranked match players are invalid")
	}
	seen := map[uint64]struct{}{}
	for _, player := range settlement.Players {
		if _, ok := seen[player.UserID]; ok {
			return fmt.Errorf("ranked match players are duplicated")
		}
		seen[player.UserID] = struct{}{}
		if player.Outcome != "win" && player.Outcome != "lose" && player.Outcome != "draw" {
			return fmt.Errorf("ranked match outcome is invalid")
		}
		if strings.TrimSpace(player.IdempotencyKey) == "" || len(player.IdempotencyKey) > 128 {
			return fmt.Errorf("ranked idempotency_key is invalid")
		}
	}
	if len(settlement.Players) == 2 && ((settlement.Players[0].Outcome == "win") != (settlement.Players[1].Outcome == "lose") ||
		(settlement.Players[0].Outcome == "lose") != (settlement.Players[1].Outcome == "win") ||
		(settlement.Players[0].Outcome == "draw") != (settlement.Players[1].Outcome == "draw")) {
		return fmt.Errorf("ranked match outcomes are inconsistent")
	}
	return nil
}

func clampRankRating(value int) int { return clampRankInt(value, 0, RankMaxRating) }
func clampRankInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
func maxRankInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
func minRankInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

var _ RankStore = (*SQLRankRepository)(nil)
