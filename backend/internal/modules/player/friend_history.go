package player

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/example/go-service/internal/apperror"
)

type FriendMatchHistoryRecord struct {
	MatchID       string    `json:"match_id"`
	RoundID       string    `json:"round_id,omitempty"`
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

type FriendMatchHistoryPage struct {
	Matches    []FriendMatchHistoryRecord `json:"matches"`
	NextCursor string                     `json:"next_cursor,omitempty"`
}

type FriendMatchHistoryStore interface {
	ListFriendMatchHistory(context.Context, uint64, *RankHistoryCursor, int) (FriendMatchHistoryPage, error)
	GetFriendMatchHistory(context.Context, uint64, string) (FriendMatchHistoryRecord, error)
}

type SQLFriendMatchHistoryRepository struct {
	db *sql.DB
}

func NewSQLFriendMatchHistoryRepository(db *sql.DB) *SQLFriendMatchHistoryRepository {
	return &SQLFriendMatchHistoryRepository{db: db}
}

func (r *SQLFriendMatchHistoryRepository) ListFriendMatchHistory(ctx context.Context, userID uint64, cursor *RankHistoryCursor, limit int) (FriendMatchHistoryPage, error) {
	if r == nil || r.db == nil {
		return FriendMatchHistoryPage{}, fmt.Errorf("friend history database is not initialized")
	}
	if limit < 1 || limit > 100 {
		return FriendMatchHistoryPage{}, fmt.Errorf("friend history limit is invalid")
	}
	args := []any{userID}
	condition := ""
	if cursor != nil {
		condition = " AND (mine.created_at < ? OR (mine.created_at = ? AND mine.id < ?))"
		args = append(args, cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}
	args = append(args, limit+1)
	rows, err := r.db.QueryContext(ctx, `
SELECT mine.id,
       COALESCE(JSON_UNQUOTE(JSON_EXTRACT(mine.metadata_json, '$.match_id')), mine.room_id),
       COALESCE(JSON_UNQUOTE(JSON_EXTRACT(mine.metadata_json, '$.round_id')), ''),
       mine.outcome,
       COALESCE(opponent.nickname, ''),
       mine.questions, mine.elapsed_ms,
       CAST(COALESCE(JSON_UNQUOTE(JSON_EXTRACT(mine.metadata_json, '$.player_mistakes')), '0') AS UNSIGNED),
       CAST(COALESCE(rank_result.rating_delta, 0) AS SIGNED), mine.created_at
FROM player_leaderboard_submissions mine
LEFT JOIN player_leaderboard_submissions opponent_submission
  ON opponent_submission.mode = 'friend'
 AND opponent_submission.room_id = mine.room_id
 AND opponent_submission.user_id <> mine.user_id
 AND COALESCE(JSON_UNQUOTE(JSON_EXTRACT(opponent_submission.metadata_json, '$.round_id')), '') =
     COALESCE(JSON_UNQUOTE(JSON_EXTRACT(mine.metadata_json, '$.round_id')), '')
LEFT JOIN users opponent ON opponent.id = opponent_submission.user_id AND opponent.status = 1
LEFT JOIN ranked_match_results rank_result
  ON rank_result.match_id = COALESCE(JSON_UNQUOTE(JSON_EXTRACT(mine.metadata_json, '$.match_id')), mine.room_id)
 AND rank_result.user_id = mine.user_id
WHERE mine.mode = 'friend' AND mine.user_id = ?`+condition+`
ORDER BY mine.created_at DESC, mine.id DESC
LIMIT ?`, args...)
	if err != nil {
		return FriendMatchHistoryPage{}, fmt.Errorf("list friend match history: %w", err)
	}
	defer rows.Close()
	items := make([]FriendMatchHistoryRecord, 0, limit)
	var next RankHistoryCursor
	for rows.Next() {
		var id uint64
		var item FriendMatchHistoryRecord
		if err := rows.Scan(&id, &item.MatchID, &item.RoundID, &item.Outcome, &item.OpponentName,
			&item.Solved, &item.ElapsedMS, &item.Mistakes, &item.RatingDelta, &item.CreatedAt); err != nil {
			return FriendMatchHistoryPage{}, fmt.Errorf("scan friend match history: %w", err)
		}
		item.Mode = LeaderboardFriend
		item.QuestionCount = friendQuestionCount
		item.OpponentName = normalizePublicNickname(item.OpponentName)
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
		return FriendMatchHistoryPage{}, fmt.Errorf("iterate friend match history: %w", err)
	}
	page := FriendMatchHistoryPage{Matches: items}
	if next.ID != 0 {
		page.NextCursor = encodeFriendHistoryCursor(next)
	}
	return page, nil
}

func (r *SQLFriendMatchHistoryRepository) GetFriendMatchHistory(ctx context.Context, userID uint64, matchID string) (FriendMatchHistoryRecord, error) {
	if r == nil || r.db == nil {
		return FriendMatchHistoryRecord{}, fmt.Errorf("friend history database is not initialized")
	}
	var item FriendMatchHistoryRecord
	err := r.db.QueryRowContext(ctx, `
SELECT COALESCE(JSON_UNQUOTE(JSON_EXTRACT(mine.metadata_json, '$.match_id')), mine.room_id),
       COALESCE(JSON_UNQUOTE(JSON_EXTRACT(mine.metadata_json, '$.round_id')), ''),
       mine.outcome, COALESCE(opponent.nickname, ''), mine.questions, mine.elapsed_ms,
       CAST(COALESCE(JSON_UNQUOTE(JSON_EXTRACT(mine.metadata_json, '$.player_mistakes')), '0') AS UNSIGNED),
       CAST(COALESCE(rank_result.rating_delta, 0) AS SIGNED), mine.created_at
FROM player_leaderboard_submissions mine
LEFT JOIN player_leaderboard_submissions opponent_submission
  ON opponent_submission.mode = 'friend'
 AND opponent_submission.room_id = mine.room_id
 AND opponent_submission.user_id <> mine.user_id
 AND COALESCE(JSON_UNQUOTE(JSON_EXTRACT(opponent_submission.metadata_json, '$.round_id')), '') =
     COALESCE(JSON_UNQUOTE(JSON_EXTRACT(mine.metadata_json, '$.round_id')), '')
LEFT JOIN users opponent ON opponent.id = opponent_submission.user_id AND opponent.status = 1
LEFT JOIN ranked_match_results rank_result
  ON rank_result.match_id = COALESCE(JSON_UNQUOTE(JSON_EXTRACT(mine.metadata_json, '$.match_id')), mine.room_id)
 AND rank_result.user_id = mine.user_id
WHERE mine.mode = 'friend' AND mine.user_id = ?
  AND COALESCE(JSON_UNQUOTE(JSON_EXTRACT(mine.metadata_json, '$.match_id')), mine.room_id) = ?
ORDER BY mine.id DESC LIMIT 1`, userID, strings.TrimSpace(matchID)).Scan(
		&item.MatchID, &item.RoundID, &item.Outcome, &item.OpponentName, &item.Solved,
		&item.ElapsedMS, &item.Mistakes, &item.RatingDelta, &item.CreatedAt)
	if err != nil {
		return FriendMatchHistoryRecord{}, err
	}
	item.Mode = LeaderboardFriend
	item.QuestionCount = friendQuestionCount
	item.OpponentName = normalizePublicNickname(item.OpponentName)
	if item.OpponentName == "" {
		item.OpponentName = "玩家"
	}
	return item, nil
}

func encodeFriendHistoryCursor(cursor RankHistoryCursor) string {
	payload, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeFriendHistoryCursor(value string) (*RankHistoryCursor, error) {
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

func (s *Service) ListFriendMatchHistory(ctx context.Context, userID uint64, cursorValue string, limit int) (FriendMatchHistoryPage, error) {
	if s == nil || s.friendHistory == nil {
		return FriendMatchHistoryPage{}, apperror.ServiceUnavailable("friend match history service is unavailable", nil)
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	cursor, err := decodeFriendHistoryCursor(cursorValue)
	if err != nil {
		return FriendMatchHistoryPage{}, apperror.BadRequest("friend history cursor is invalid", err)
	}
	return s.friendHistory.ListFriendMatchHistory(ctx, userID, cursor, limit)
}

func (s *Service) GetFriendMatchHistory(ctx context.Context, userID uint64, matchID string) (FriendMatchHistoryRecord, error) {
	if s == nil || s.friendHistory == nil {
		return FriendMatchHistoryRecord{}, apperror.ServiceUnavailable("friend match history service is unavailable", nil)
	}
	matchID = strings.TrimSpace(matchID)
	if matchID == "" || len(matchID) > 128 {
		return FriendMatchHistoryRecord{}, apperror.BadRequest("match_id is invalid", nil)
	}
	result, err := s.friendHistory.GetFriendMatchHistory(ctx, userID, matchID)
	if err == sql.ErrNoRows {
		return FriendMatchHistoryRecord{}, apperror.NotFound("friend match was not found", err)
	}
	return result, err
}
