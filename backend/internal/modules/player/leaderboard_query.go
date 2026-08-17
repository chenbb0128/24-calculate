package player

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/example/go-service/internal/apperror"
)

const LeaderboardOverall = "overall"

func (s *Service) LeaderboardScopedPage(ctx context.Context, userID uint64, mode string, query LeaderboardQuery) (LeaderboardResponse, error) {
	profile, err := s.profiles.GetProfile(ctx, userID)
	if err != nil {
		return LeaderboardResponse{}, err
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != LeaderboardCampaign && mode != LeaderboardDaily && mode != LeaderboardEndless && mode != LeaderboardFriend && mode != LeaderboardOverall {
		return LeaderboardResponse{}, apperror.BadRequest("leaderboard mode is invalid", nil)
	}
	scope := normalizeLeaderboardScope(strings.ToLower(strings.TrimSpace(query.Scope)))
	period := strings.ToLower(strings.TrimSpace(query.Period))
	if period == "" {
		period = "all"
	}
	if period != "all" && period != "weekly" && period != "monthly" {
		return LeaderboardResponse{}, apperror.BadRequest("leaderboard period is invalid", nil)
	}
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}
	if pageSize > 200 {
		pageSize = 200
	}

	friendIDs := map[uint64]struct{}{userID: {}}
	if scope == LeaderboardFriends {
		ids, err := s.store.ListFriendUserIDs(ctx, userID)
		if err != nil {
			return LeaderboardResponse{}, err
		}
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

	type row struct {
		id           uint64
		name, avatar string
		score        int64
		createdAt    time.Time
	}
	rows := make([]row, 0)
	now := time.Now().UTC()
	start := time.Time{}
	if period == "weekly" {
		start = now.AddDate(0, 0, -7)
	} else if period == "monthly" {
		start = now.AddDate(0, -1, 0)
	}
	accept := func(createdAt time.Time) bool {
		return start.IsZero() || createdAt.IsZero() || !createdAt.Before(start)
	}
	appendRow := func(id uint64, name, avatar string, score int64, createdAt time.Time) {
		if include(id) && accept(createdAt) {
			rows = append(rows, row{id: id, name: normalizePublicNickname(name), avatar: normalizePublicAvatar(avatar), score: int64(clampLeaderboardScore(score)), createdAt: createdAt})
		}
	}

	if mode == LeaderboardOverall {
		combined := make(map[uint64]row)
		add := func(id uint64, name, avatar string, score int64, createdAt time.Time) {
			if !include(id) || !accept(createdAt) {
				return
			}
			current := combined[id]
			current.id, current.name, current.avatar = id, normalizePublicNickname(name), normalizePublicAvatar(avatar)
			current.score += maxInt64(0, score)
			if createdAt.After(current.createdAt) {
				current.createdAt = createdAt
			}
			combined[id] = current
		}
		campaign, err := s.store.ListCampaignLeaderboard(ctx)
		if err != nil {
			return LeaderboardResponse{}, err
		}
		for _, item := range campaign {
			add(item.UserID, item.Nickname, item.Avatar, item.Score, item.LastCreatedAt)
		}
		dailyDate := now.In(shanghaiLocation).Format("2006-01-02")
		daily, err := s.store.ListDailyLeaderboard(ctx, dailyDate)
		if err != nil {
			return LeaderboardResponse{}, err
		}
		for _, item := range daily {
			add(item.UserID, item.Nickname, item.Avatar, int64(item.Score), item.CreatedAt)
		}
		endless, err := s.store.ListEndlessLeaderboard(ctx)
		if err != nil {
			return LeaderboardResponse{}, err
		}
		for _, item := range endless {
			add(item.UserID, item.Nickname, item.Avatar, item.Score, item.LastCreatedAt)
		}
		friend, err := s.store.ListFriendLeaderboard(ctx)
		if err != nil {
			return LeaderboardResponse{}, err
		}
		for _, item := range friend {
			add(item.UserID, item.Nickname, item.Avatar, item.Score, item.LastCreatedAt)
		}
		for _, item := range combined {
			rows = append(rows, item)
		}
	} else {
		switch mode {
		case LeaderboardCampaign:
			items, err := s.store.ListCampaignLeaderboard(ctx)
			if err != nil {
				return LeaderboardResponse{}, err
			}
			for _, item := range items {
				appendRow(item.UserID, item.Nickname, item.Avatar, item.Score, item.LastCreatedAt)
			}
		case LeaderboardDaily:
			dateKey := now.In(shanghaiLocation).Format("2006-01-02")
			items, err := s.store.ListDailyLeaderboard(ctx, dateKey)
			if err != nil {
				return LeaderboardResponse{}, err
			}
			for _, item := range items {
				appendRow(item.UserID, item.Nickname, item.Avatar, int64(item.Score), item.CreatedAt)
			}
		case LeaderboardEndless:
			items, err := s.store.ListEndlessLeaderboard(ctx)
			if err != nil {
				return LeaderboardResponse{}, err
			}
			for _, item := range items {
				appendRow(item.UserID, item.Nickname, item.Avatar, item.Score, item.LastCreatedAt)
			}
		case LeaderboardFriend:
			items, err := s.store.ListFriendLeaderboard(ctx)
			if err != nil {
				return LeaderboardResponse{}, err
			}
			for _, item := range items {
				appendRow(item.UserID, item.Nickname, item.Avatar, item.Score, item.LastCreatedAt)
			}
		}
	}

	seen := false
	for _, item := range rows {
		if item.id == userID {
			seen = true
			break
		}
	}
	if !seen {
		rows = append(rows, row{id: profile.ID, name: normalizePublicNickname(profile.Nickname), avatar: normalizePublicAvatar(profile.Avatar)})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].score != rows[j].score {
			return rows[i].score > rows[j].score
		}
		return rows[i].id < rows[j].id
	})
	entries := make([]LeaderboardEntry, len(rows))
	myRank, myScore := 0, 0
	for i, item := range rows {
		anomaly := item.score > 999999999 || (!item.createdAt.IsZero() && item.score > 100000 && now.Sub(item.createdAt) < time.Minute)
		entries[i] = LeaderboardEntry{Rank: i + 1, UserID: item.id, Nickname: item.name, Avatar: item.avatar, Score: clampLeaderboardScore(item.score), IsMe: item.id == userID, Anomaly: anomaly}
		if item.id == userID {
			myRank, myScore = i+1, entries[i].Score
		}
	}
	total := len(entries)
	from := (page - 1) * pageSize
	if from > total {
		from = total
	}
	to := from + pageSize
	if to > total {
		to = total
	}
	return LeaderboardResponse{Mode: mode, Scope: scope, DateKey: func() string {
		if mode == LeaderboardDaily {
			return now.In(shanghaiLocation).Format("2006-01-02")
		}
		return ""
	}(), MyUserID: userID, Entries: entries[from:to], MyRank: myRank, MyScore: myScore, Period: period, Page: page, PageSize: pageSize, Total: total}, nil
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
