package player

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/example/go-service/internal/apperror"
)

func nullableUserID(value uint64) any {
	if value == 0 {
		return nil
	}
	return value
}

func (s *Service) GetRankedSummary(ctx context.Context, userID uint64, seasonID string) (RankedSummary, error) {
	if s == nil || s.rankHistory == nil {
		return RankedSummary{}, apperror.ServiceUnavailable("ranked history service is unavailable", nil)
	}
	seasonID, err := s.resolveRankSeasonID(seasonID)
	if err != nil {
		return RankedSummary{}, err
	}
	return s.rankHistory.GetRankedSummary(ctx, userID, seasonID)
}

func (s *Service) ListRankedMatches(ctx context.Context, userID uint64, seasonID, cursorValue string, limit int) (RankedMatchPage, error) {
	if s == nil || s.rankHistory == nil {
		return RankedMatchPage{}, apperror.ServiceUnavailable("ranked history service is unavailable", nil)
	}
	seasonID, err := s.resolveRankSeasonID(seasonID)
	if err != nil {
		return RankedMatchPage{}, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	cursor, err := decodeRankHistoryCursor(cursorValue)
	if err != nil {
		return RankedMatchPage{}, apperror.BadRequest("ranked history cursor is invalid", err)
	}
	return s.rankHistory.ListRankedMatches(ctx, userID, seasonID, cursor, limit)
}

func (s *Service) GetRankedMatch(ctx context.Context, userID uint64, matchID string) (RankedMatchRecord, error) {
	if s == nil || s.rankHistory == nil {
		return RankedMatchRecord{}, apperror.ServiceUnavailable("ranked history service is unavailable", nil)
	}
	matchID = strings.TrimSpace(matchID)
	if matchID == "" || len(matchID) > 128 {
		return RankedMatchRecord{}, apperror.BadRequest("match_id is invalid", nil)
	}
	result, err := s.rankHistory.GetRankedMatch(ctx, userID, matchID)
	if err != nil {
		if err == sql.ErrNoRows {
			return RankedMatchRecord{}, apperror.NotFound("ranked match was not found", err)
		}
		return RankedMatchRecord{}, err
	}
	return result, nil
}

func (s *Service) resolveRankSeasonID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return s.currentRankSeasonID(), nil
	}
	return normalizeRankSeasonID(value)
}

func encodeRankHistoryCursor(cursor RankHistoryCursor) string {
	payload, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeRankHistoryCursor(value string) (*RankHistoryCursor, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}
	var cursor RankHistoryCursor
	if err := json.Unmarshal(decoded, &cursor); err != nil || cursor.ID == 0 || cursor.CreatedAt.IsZero() {
		return nil, fmt.Errorf("cursor payload is invalid")
	}
	return &cursor, nil
}

func (r *SQLRankRepository) GetRankedSummary(ctx context.Context, userID uint64, seasonID string) (RankedSummary, error) {
	if r == nil || r.db == nil {
		return RankedSummary{}, fmt.Errorf("rank repository database is not initialized")
	}
	seasonID, err := normalizeRankSeasonID(seasonID)
	if err != nil {
		return RankedSummary{}, err
	}
	profile, err := r.GetOrCreateRankProfile(ctx, userID, seasonID)
	if err != nil {
		return RankedSummary{}, err
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT outcome
FROM ranked_match_results
WHERE user_id = ? AND season_id = ?
ORDER BY created_at DESC, id DESC`, userID, seasonID)
	if err != nil {
		return RankedSummary{}, fmt.Errorf("list ranked outcomes: %w", err)
	}
	defer rows.Close()
	currentStreak := 0
	for rows.Next() {
		var outcome string
		if err := rows.Scan(&outcome); err != nil {
			return RankedSummary{}, fmt.Errorf("scan ranked outcome: %w", err)
		}
		if outcome != "win" {
			break
		}
		currentStreak++
	}
	if err := rows.Err(); err != nil {
		return RankedSummary{}, fmt.Errorf("iterate ranked outcomes: %w", err)
	}

	rows, err = r.db.QueryContext(ctx, `
SELECT outcome
FROM ranked_match_results
WHERE user_id = ? AND season_id = ?
ORDER BY created_at ASC, id ASC`, userID, seasonID)
	if err != nil {
		return RankedSummary{}, fmt.Errorf("list ranked streaks: %w", err)
	}
	defer rows.Close()
	streak, bestStreak := 0, 0
	for rows.Next() {
		var outcome string
		if err := rows.Scan(&outcome); err != nil {
			return RankedSummary{}, fmt.Errorf("scan ranked streak: %w", err)
		}
		if outcome == "win" {
			streak++
			if streak > bestStreak {
				bestStreak = streak
			}
		} else {
			streak = 0
		}
	}
	if err := rows.Err(); err != nil {
		return RankedSummary{}, fmt.Errorf("iterate ranked streaks: %w", err)
	}
	return RankedSummary{
		SeasonID: seasonID, Label: rankLabel(profile.Tier, profile.Division), Rating: profile.Rating,
		StarsLabel: fmt.Sprintf("%d 星", profile.Stars), RankedMatches: profile.RankedMatches,
		Wins: profile.Wins, Losses: profile.Losses, Draws: profile.Draws,
		WinRate: rankWinRate(profile.Wins, profile.RankedMatches), CurrentStreak: currentStreak, BestStreak: bestStreak,
	}, nil
}

func (r *SQLRankRepository) ListRankedMatches(ctx context.Context, userID uint64, seasonID string, cursor *RankHistoryCursor, limit int) (RankedMatchPage, error) {
	if r == nil || r.db == nil {
		return RankedMatchPage{}, fmt.Errorf("rank repository database is not initialized")
	}
	seasonID, err := normalizeRankSeasonID(seasonID)
	if err != nil {
		return RankedMatchPage{}, err
	}
	if limit < 1 || limit > 100 {
		return RankedMatchPage{}, fmt.Errorf("ranked match limit is invalid")
	}
	args := []any{userID, seasonID}
	condition := ""
	if cursor != nil {
		condition = " AND (r.created_at < ? OR (r.created_at = ? AND r.id < ?))"
		args = append(args, cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}
	args = append(args, limit+1)
	rows, err := r.db.QueryContext(ctx, `
SELECT r.id, r.match_id, r.outcome, COALESCE(u.nickname, ''),
       r.solved, r.question_count, r.elapsed_ms, r.mistakes, r.rating_delta, r.created_at
FROM ranked_match_results r
LEFT JOIN users u ON u.id = r.opponent_user_id
WHERE r.user_id = ? AND r.season_id = ?`+condition+`
ORDER BY r.created_at DESC, r.id DESC
LIMIT ?`, args...)
	if err != nil {
		return RankedMatchPage{}, fmt.Errorf("list ranked matches: %w", err)
	}
	defer rows.Close()
	items := make([]RankedMatchRecord, 0, limit)
	var next RankHistoryCursor
	for rows.Next() {
		var id uint64
		var item RankedMatchRecord
		var outcome, opponentName string
		if err := rows.Scan(&id, &item.MatchID, &outcome, &opponentName, &item.Solved,
			&item.QuestionCount, &item.ElapsedMS, &item.Mistakes, &item.RatingDelta, &item.CreatedAt); err != nil {
			return RankedMatchPage{}, fmt.Errorf("scan ranked match: %w", err)
		}
		item.Mode, item.Outcome = "ranked", publicRankOutcome(outcome)
		item.OpponentName = normalizePublicNickname(opponentName)
		if item.OpponentName == "" {
			item.OpponentName = "玩家"
		}
		if len(items) < limit {
			items = append(items, item)
		} else {
			next = RankHistoryCursor{CreatedAt: item.CreatedAt, ID: id}
		}
	}
	if err := rows.Err(); err != nil {
		return RankedMatchPage{}, fmt.Errorf("iterate ranked matches: %w", err)
	}
	page := RankedMatchPage{Matches: items}
	if next.ID != 0 {
		page.NextCursor = encodeRankHistoryCursor(next)
	}
	return page, nil
}

func (r *SQLRankRepository) GetRankedMatch(ctx context.Context, userID uint64, matchID string) (RankedMatchRecord, error) {
	if r == nil || r.db == nil {
		return RankedMatchRecord{}, fmt.Errorf("rank repository database is not initialized")
	}
	row := r.db.QueryRowContext(ctx, `
SELECT r.outcome, COALESCE(u.nickname, ''), r.solved, r.question_count,
       r.elapsed_ms, r.mistakes, r.rating_delta, r.created_at
FROM ranked_match_results r
LEFT JOIN users u ON u.id = r.opponent_user_id
WHERE r.user_id = ? AND r.match_id = ?
ORDER BY r.id DESC LIMIT 1`, userID, strings.TrimSpace(matchID))
	var outcome, opponentName string
	var item RankedMatchRecord
	if err := row.Scan(&outcome, &opponentName, &item.Solved, &item.QuestionCount, &item.ElapsedMS,
		&item.Mistakes, &item.RatingDelta, &item.CreatedAt); err != nil {
		return RankedMatchRecord{}, err
	}
	item.MatchID, item.Mode, item.Outcome = strings.TrimSpace(matchID), "ranked", publicRankOutcome(outcome)
	item.OpponentName = normalizePublicNickname(opponentName)
	if item.OpponentName == "" {
		item.OpponentName = "玩家"
	}
	return item, nil
}

func rankWinRate(wins, matches int) float64 {
	if matches <= 0 {
		return 0
	}
	return float64(wins) / float64(matches)
}

func publicRankOutcome(outcome string) string {
	return outcome
}

func rankLabel(tier string, division int) string {
	labels := map[string]string{RankTierBronze: "青铜", RankTierSilver: "白银", RankTierGold: "黄金", RankTierPlatinum: "铂金", RankTierDiamond: "钻石", RankTierMaster: "大师", RankTierKing: "王者"}
	label := labels[tier]
	if label == "" {
		label = labels[RankTierBronze]
	}
	divisions := []string{"I", "II", "III"}
	return fmt.Sprintf("%s %s", label, divisions[clampRankInt(division, 1, 3)-1])
}
