package player

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/example/go-service/internal/apperror"
)

type friendHistoryStoreFake struct {
	rows       map[uint64][]FriendMatchHistoryRecord
	details    map[uint64]map[string]FriendMatchHistoryRecord
	seenCursor []*RankHistoryCursor
}

func (f *friendHistoryStoreFake) ListFriendMatchHistory(_ context.Context, userID uint64, cursor *RankHistoryCursor, limit int) (FriendMatchHistoryPage, error) {
	f.seenCursor = append(f.seenCursor, cursor)
	rows := f.rows[userID]
	if cursor != nil {
		for index, row := range rows {
			if row.CreatedAt.Equal(cursor.CreatedAt) {
				if index+1 >= len(rows) {
					return FriendMatchHistoryPage{Matches: nil}, nil
				}
				rows = rows[index+1:]
				break
			}
		}
	}
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return FriendMatchHistoryPage{Matches: rows}, nil
}

func (f *friendHistoryStoreFake) GetFriendMatchHistory(_ context.Context, userID uint64, matchID string) (FriendMatchHistoryRecord, error) {
	if item, ok := f.details[userID][matchID]; ok {
		return item, nil
	}
	return FriendMatchHistoryRecord{}, sql.ErrNoRows
}

func TestFriendHistoryIsScopedToAuthenticatedUser(t *testing.T) {
	created := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	store := &friendHistoryStoreFake{rows: map[uint64][]FriendMatchHistoryRecord{
		1: {{MatchID: "match-user-1", RoundID: "round-a", CreatedAt: created}},
		2: {{MatchID: "match-user-2", RoundID: "round-b", CreatedAt: created}},
	}}
	service := NewService(nil, nil)
	service.SetFriendHistoryStore(store)

	first, err := service.ListFriendMatchHistory(context.Background(), 1, "", 20)
	if err != nil || len(first.Matches) != 1 || first.Matches[0].MatchID != "match-user-1" || first.Matches[0].RoundID != "round-a" {
		t.Fatalf("user 1 history = %+v, error = %v", first, err)
	}
	second, err := service.ListFriendMatchHistory(context.Background(), 2, "", 20)
	if err != nil || len(second.Matches) != 1 || second.Matches[0].MatchID != "match-user-2" || second.Matches[0].RoundID != "round-b" {
		t.Fatalf("user 2 history = %+v, error = %v", second, err)
	}
}

func TestFriendHistoryCursorIsDecodedAndForwarded(t *testing.T) {
	created := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	store := &friendHistoryStoreFake{rows: map[uint64][]FriendMatchHistoryRecord{
		1: {
			{MatchID: "match-1", CreatedAt: created},
			{MatchID: "match-2", CreatedAt: created.Add(-time.Minute)},
		},
	}}
	service := NewService(nil, nil)
	service.SetFriendHistoryStore(store)

	first, err := service.ListFriendMatchHistory(context.Background(), 1, "", 1)
	if err != nil || len(first.Matches) != 1 || first.Matches[0].MatchID != "match-1" {
		t.Fatalf("first page = %+v, error = %v", first, err)
	}
	if first.NextCursor != "" {
		t.Fatalf("fake store should not manufacture a cursor: %q", first.NextCursor)
	}

	cursor := encodeFriendHistoryCursor(RankHistoryCursor{CreatedAt: created, ID: 1})
	second, err := service.ListFriendMatchHistory(context.Background(), 1, cursor, 1)
	if err != nil || len(second.Matches) != 1 || second.Matches[0].MatchID != "match-2" {
		t.Fatalf("second page = %+v, error = %v", second, err)
	}
	if len(store.seenCursor) != 2 || store.seenCursor[0] != nil || store.seenCursor[1] == nil || store.seenCursor[1].ID != 1 {
		t.Fatalf("forwarded cursors = %#v, want decoded second-page cursor", store.seenCursor)
	}
}

func TestFriendHistoryDetailCannotCrossUserOrRound(t *testing.T) {
	created := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	store := &friendHistoryStoreFake{
		details: map[uint64]map[string]FriendMatchHistoryRecord{
			1: {"match-1": {MatchID: "match-1", RoundID: "round-a", CreatedAt: created}},
			2: {"match-1": {MatchID: "match-1", RoundID: "round-b", CreatedAt: created}},
		},
	}
	service := NewService(nil, nil)
	service.SetFriendHistoryStore(store)

	owned, err := service.GetFriendMatchHistory(context.Background(), 1, "match-1")
	if err != nil || owned.RoundID != "round-a" {
		t.Fatalf("owned detail = %+v, error = %v", owned, err)
	}
	other, err := service.GetFriendMatchHistory(context.Background(), 3, "match-1")
	if err == nil {
		t.Fatal("GetFriendMatchHistory() allowed a non-owner to read the match")
	}
	appErr, ok := err.(*apperror.AppError)
	if !ok || appErr.HTTPStatus != 404 {
		t.Fatalf("non-owner error = %#v, want HTTP 404", err)
	}

	// A rematch may reuse the room code, but its round must remain the exact
	// round stored with the caller's submission. The service never infers it
	// from another user's record.
	other, err = service.GetFriendMatchHistory(context.Background(), 2, "match-1")
	if err != nil || other.RoundID != "round-b" {
		t.Fatalf("second user's round = %+v, error = %v", other, err)
	}
}
