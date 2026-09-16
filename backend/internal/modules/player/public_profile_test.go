package player

import (
	"context"
	"errors"
	"testing"

	"github.com/example/go-service/internal/modules/moderation"
	"github.com/example/go-service/internal/modules/user"
	db "github.com/example/go-service/internal/store/sqlc"
)

func TestLeaderboardHidesRejectedAndUnreviewedProfiles(t *testing.T) {
	store := &leaderboardStore{campaignRows: []db.CampaignLeaderboardRow{{
		UserID: 8, Nickname: "违规昵称", Avatar: "https://calc-api.pdurl.cn/avatars/8/bad.webp",
		NicknameModerationStatus: string(moderation.StatusRejected), AvatarModerationStatus: string(moderation.StatusUnreviewed), Score: 100,
	}}}
	service := NewService(leaderboardProfileReader{profile: user.ProfileResponse{ID: 3, Nickname: "我", NicknameModerationStatus: string(moderation.StatusApproved), AvatarModerationStatus: string(moderation.StatusApproved)}}, store)

	result, err := service.Leaderboard(context.Background(), 3, LeaderboardCampaign)
	if err != nil {
		t.Fatalf("Leaderboard() error = %v", err)
	}
	if result.Entries[0].Nickname != user.DefaultNickname || result.Entries[0].Avatar != user.DefaultAvatar {
		t.Fatalf("unsafe leaderboard entry = %#v", result.Entries[0])
	}
}

func TestRankLeaderboardHidesPendingAvatarAndNickname(t *testing.T) {
	rankStore := &rankStoreFake{
		profile: RankProfile{UserID: 3, Rating: RankDefault, Tier: RankTierBronze, Division: 3},
		rows: []RankLeaderboardRow{{
			UserID: 8, Nickname: "pending-name", Avatar: "https://calc-api.pdurl.cn/avatars/8/pending.webp",
			NicknameModerationStatus: string(moderation.StatusPending), AvatarModerationStatus: string(moderation.StatusRejected), Rating: 1200,
		}},
	}
	service := NewService(leaderboardProfileReader{profile: user.ProfileResponse{ID: 3, Nickname: "我", NicknameModerationStatus: string(moderation.StatusApproved), AvatarModerationStatus: string(moderation.StatusApproved)}}, &leaderboardStore{})
	service.SetRankStore(rankStore)

	result, err := service.LeaderboardScopedPage(context.Background(), 3, LeaderboardRanked, LeaderboardQuery{Period: "season", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("rank leaderboard error = %v", err)
	}
	if result.Entries[0].Nickname != user.DefaultNickname || result.Entries[0].Avatar != user.DefaultAvatar {
		t.Fatalf("unsafe rank entry = %#v", result.Entries[0])
	}
}

func TestFriendRoomResponseReFiltersPersistedUnsafeSnapshot(t *testing.T) {
	rooms := &matchmakingRoomStoreFake{room: FriendRoom{
		RoomCode: "123456", RoomID: "friend-123456", OwnerID: 8, Status: FriendRoomWaiting,
		Players: []FriendRoomPlayer{{UserID: 8, Nickname: "违规昵称", Avatar: "https://evil.example/avatar.webp"}},
	}}
	service := NewServiceWithRoomsAndEndless(failingProfileReader{}, &leaderboardStore{}, rooms, &matchmakingStoreFake{})

	result, err := service.GetFriendRoom(context.Background(), "123456")
	if err != nil {
		t.Fatalf("GetFriendRoom() error = %v", err)
	}
	if result.Players[0].Nickname != user.DefaultNickname || result.Players[0].Avatar != user.DefaultAvatar {
		t.Fatalf("unsafe room player = %#v", result.Players[0])
	}
}

func TestRankAndFriendHistoryHideUnsafeOpponentNames(t *testing.T) {
	if got := safePublicOpponentName("违规昵称", string(moderation.StatusRejected)); got != user.DefaultNickname {
		t.Fatalf("rejected opponent name = %q", got)
	}
	if got := safePublicOpponentName("待审核昵称", string(moderation.StatusPending)); got != user.DefaultNickname {
		t.Fatalf("pending opponent name = %q", got)
	}
}

type failingProfileReader struct{}

func (failingProfileReader) GetProfile(context.Context, uint64) (user.ProfileResponse, error) {
	return user.ProfileResponse{}, errors.New("profile unavailable")
}
