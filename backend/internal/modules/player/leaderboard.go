package player

import (
	"context"
	"sort"
	"time"

	"github.com/example/go-service/internal/apperror"
)

const (
	LeaderboardCampaign = "campaign"
	LeaderboardDaily    = "daily"
	LeaderboardEndless  = "endless"
	LeaderboardFriend   = "friend"
	LeaderboardGlobal   = "global"
	LeaderboardFriends  = "friends"
)

func (s *Service) Leaderboard(ctx context.Context, userID uint64, mode string) (LeaderboardResponse, error) {
	return s.LeaderboardScoped(ctx, userID, mode, LeaderboardGlobal)
}

func (s *Service) LeaderboardScoped(ctx context.Context, userID uint64, mode, scope string) (LeaderboardResponse, error) {
	profile, err := s.profiles.GetProfile(ctx, userID)
	if err != nil {
		return LeaderboardResponse{}, err
	}

	mode = normalizeLeaderboardMode(mode)
	scope = normalizeLeaderboardScope(scope)
	if mode != LeaderboardCampaign && mode != LeaderboardDaily && mode != LeaderboardEndless && mode != LeaderboardFriend {
		return LeaderboardResponse{}, apperror.BadRequest("leaderboard mode is invalid", nil)
	}
	if scope != LeaderboardGlobal && scope != LeaderboardFriends {
		return LeaderboardResponse{}, apperror.BadRequest("leaderboard scope is invalid", nil)
	}

	var friendIDs map[uint64]struct{}
	if scope == LeaderboardFriends {
		ids, err := s.store.ListFriendUserIDs(ctx, userID)
		if err != nil {
			return LeaderboardResponse{}, err
		}
		friendIDs = make(map[uint64]struct{}, len(ids))
		for _, id := range ids {
			friendIDs[id] = struct{}{}
		}
	}
	include := func(id uint64) bool {
		if scope == LeaderboardGlobal {
			return true
		}
		_, ok := friendIDs[id]
		return ok
	}

	entries := make([]LeaderboardEntry, 0)
	dateKey := ""
	appendRow := func(id uint64, nickname, avatar string, score int64) {
		if !include(id) {
			return
		}
		entries = append(entries, LeaderboardEntry{
			UserID:   id,
			Nickname: nickname,
			Avatar:   avatar,
			Score:    clampLeaderboardScore(score),
		})
	}

	switch mode {
	case LeaderboardCampaign:
		rows, err := s.store.ListCampaignLeaderboard(ctx)
		if err != nil {
			return LeaderboardResponse{}, err
		}
		for _, row := range rows {
			appendRow(row.UserID, row.Nickname, row.Avatar, row.Score)
		}
	case LeaderboardDaily:
		dateKey = time.Now().In(shanghaiLocation).Format("2006-01-02")
		rows, err := s.store.ListDailyLeaderboard(ctx, dateKey)
		if err != nil {
			return LeaderboardResponse{}, err
		}
		for _, row := range rows {
			appendRow(row.UserID, row.Nickname, row.Avatar, int64(row.Score))
		}
	case LeaderboardEndless:
		rows, err := s.store.ListEndlessLeaderboard(ctx)
		if err != nil {
			return LeaderboardResponse{}, err
		}
		for _, row := range rows {
			appendRow(row.UserID, row.Nickname, row.Avatar, row.Score)
		}
	case LeaderboardFriend:
		rows, err := s.store.ListFriendLeaderboard(ctx)
		if err != nil {
			return LeaderboardResponse{}, err
		}
		for _, row := range rows {
			appendRow(row.UserID, row.Nickname, row.Avatar, row.Score)
		}
	}

	// Keep a zero-score row for a player who has not submitted this mode yet.
	found := false
	for _, entry := range entries {
		if entry.UserID == userID {
			found = true
			break
		}
	}
	if !found {
		entries = append(entries, LeaderboardEntry{
			UserID:   profile.ID,
			Nickname: profile.Nickname,
			Avatar:   profile.Avatar,
			Score:    0,
		})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Score != entries[j].Score {
			return entries[i].Score > entries[j].Score
		}
		return entries[i].UserID < entries[j].UserID
	})

	myRank := 0
	myScore := 0
	for index := range entries {
		entries[index].Rank = index + 1
		entries[index].IsMe = entries[index].UserID == userID
		if entries[index].IsMe {
			myRank = entries[index].Rank
			myScore = entries[index].Score
		}
	}

	return LeaderboardResponse{
		Mode:     mode,
		Scope:    scope,
		DateKey:  dateKey,
		MyUserID: userID,
		Entries:  entries,
		MyRank:   myRank,
		MyScore:  myScore,
	}, nil
}

func normalizeLeaderboardMode(mode string) string {
	return mode
}

func normalizeLeaderboardScope(scope string) string {
	if scope == LeaderboardFriends {
		return LeaderboardFriends
	}
	return LeaderboardGlobal
}

func clampLeaderboardScore(score int64) int {
	if score < 0 {
		return 0
	}
	if score > 999999999 {
		return 999999999
	}
	return int(score)
}
