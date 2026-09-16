package player

import (
	"github.com/example/go-service/internal/modules/user"
)

func safePublicOpponentName(value, status string) string {
	return user.SafePublicNickname(value, status)
}

func safePublicFriendRoomPlayer(player FriendRoomPlayer) FriendRoomPlayer {
	if player.UserID == 0 {
		player.Nickname = "对手"
		player.Avatar = user.DefaultAvatar
		player.NicknameModerationStatus = ""
		player.AvatarModerationStatus = ""
		return player
	}
	player.Nickname = user.SafePublicNickname(player.Nickname, player.NicknameModerationStatus)
	player.Avatar = user.SafePublicAvatar(player.Avatar, player.AvatarModerationStatus)
	return player
}

func sanitizePublicFriendRoom(room *FriendRoom) *FriendRoom {
	if room == nil {
		return nil
	}
	copyRoom := *room
	copyRoom.Players = append([]FriendRoomPlayer(nil), room.Players...)
	for index := range copyRoom.Players {
		copyRoom.Players[index] = safePublicFriendRoomPlayer(copyRoom.Players[index])
	}
	return &copyRoom
}
