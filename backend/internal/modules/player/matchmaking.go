package player

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/example/go-service/internal/apperror"
)

const (
	matchmakingModeFriend = "friend"
	matchmakingRulesV1    = 1
	matchmakingTTL        = 30 * time.Second
	matchmakingRetention  = 10 * time.Minute
)

var ErrMatchmakingAlreadyQueued = errors.New("matchmaking ticket already queued")

type MatchmakingStore interface {
	EnqueueMatchmaking(context.Context, MatchmakingTicket) (MatchmakingTicket, error)
	GetMatchmakingTicket(context.Context, string, string) (MatchmakingTicket, error)
	GetMatchmakingTicketByUser(context.Context, string, uint64) (MatchmakingTicket, error)
	SaveMatchmakingPair(context.Context, MatchmakingTicket, MatchmakingTicket) error
	CancelMatchmaking(context.Context, string, string, uint64) error
}

type MatchmakingExpiryStore interface {
	MarkMatchmakingExpired(context.Context, MatchmakingTicket) error
}

func matchmakingStoreFrom(value any) MatchmakingStore {
	store, _ := value.(MatchmakingStore)
	return store
}

type JoinMatchmakingInput struct {
	Mode         string `json:"mode"`
	RulesVersion int    `json:"rules_version"`
	ClientTicket string `json:"client_ticket"`
	Region       string `json:"region"`
}

type MatchmakingTicket struct {
	TicketID        string            `json:"ticket_id"`
	ClientTicket    string            `json:"client_ticket"`
	UserID          uint64            `json:"user_id"`
	Mode            string            `json:"mode"`
	RulesVersion    int               `json:"rules_version"`
	Region          string            `json:"region"`
	Player          FriendRoomPlayer  `json:"player"`
	Status          string            `json:"status"`
	MatchID         string            `json:"match_id,omitempty"`
	Room            *FriendRoom       `json:"room,omitempty"`
	Opponent        *FriendRoomPlayer `json:"opponent,omitempty"`
	MatchedTicketID string            `json:"-"`
	QueueKey        string            `json:"queue_key,omitempty"`
	IsBot           bool              `json:"-"`
	BotDifficulty   string            `json:"-"`
	CreatedAt       time.Time         `json:"created_at"`
	ExpiresAt       time.Time         `json:"expires_at"`
}

type MatchmakingResponse struct {
	TicketID       string            `json:"ticket_id"`
	Mode           string            `json:"mode"`
	Status         string            `json:"status"`
	MatchID        string            `json:"match_id,omitempty"`
	Room           *FriendRoom       `json:"room,omitempty"`
	Opponent       *FriendRoomPlayer `json:"opponent,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	ExpiresAt      time.Time         `json:"expires_at"`
	WaitingSeconds int               `json:"waiting_seconds,omitempty"`
}

func (s *Service) JoinMatchmaking(ctx context.Context, userID uint64, input JoinMatchmakingInput) (MatchmakingResponse, error) {
	if err := s.allowFriendRoomAction(ctx, userID, "matchmaking_join", 10, time.Minute); err != nil {
		return MatchmakingResponse{}, err
	}
	if s.matchmaking == nil || s.rooms == nil {
		return MatchmakingResponse{}, apperror.ServiceUnavailable("快速匹配服务暂不可用", nil)
	}
	if strings.TrimSpace(input.Mode) != matchmakingModeFriend || input.RulesVersion != matchmakingRulesV1 {
		return MatchmakingResponse{}, apperror.BadRequest("matchmaking mode or rules_version is invalid", nil)
	}
	clientTicket := strings.TrimSpace(input.ClientTicket)
	if len(clientTicket) < 8 || len(clientTicket) > 128 {
		return MatchmakingResponse{}, apperror.BadRequest("client_ticket length is invalid", nil)
	}
	region := strings.TrimSpace(input.Region)
	if len(region) > 32 {
		return MatchmakingResponse{}, apperror.BadRequest("region is invalid", nil)
	}
	for _, character := range region {
		if !(character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-' || character == '_' || character == '.') {
			return MatchmakingResponse{}, apperror.BadRequest("region is invalid", nil)
		}
	}
	profile, err := s.profiles.GetProfile(ctx, userID)
	if err != nil {
		return MatchmakingResponse{}, err
	}
	now := time.Now().UTC()
	ticket := MatchmakingTicket{
		TicketID: randomMatchmakingTicketID(), ClientTicket: clientTicket, UserID: userID,
		Mode: matchmakingModeFriend, RulesVersion: matchmakingRulesV1, Region: region,
		Player: FriendRoomPlayer{UserID: profile.ID, Nickname: profile.Nickname, Avatar: profile.Avatar, Ready: true},
		Status: "searching", CreatedAt: now, ExpiresAt: now.Add(matchmakingTTL),
	}
	ticket.QueueKey = matchmakingQueueDiscriminator(ticket)
	queued, err := s.matchmaking.EnqueueMatchmaking(ctx, ticket)
	if errors.Is(err, ErrMatchmakingAlreadyQueued) {
		existing, getErr := s.matchmaking.GetMatchmakingTicketByUser(ctx, matchmakingModeFriend, userID)
		if getErr != nil {
			return MatchmakingResponse{}, getErr
		}
		if existing.Status == "searching" && time.Now().UTC().After(existing.ExpiresAt) {
			if expiryStore, ok := s.matchmaking.(MatchmakingExpiryStore); ok {
				if expireErr := expiryStore.MarkMatchmakingExpired(ctx, existing); expireErr != nil {
					return MatchmakingResponse{}, expireErr
				}
			} else {
				_ = s.matchmaking.CancelMatchmaking(ctx, matchmakingModeFriend, existing.TicketID, userID)
			}
			queued, err = s.matchmaking.EnqueueMatchmaking(ctx, ticket)
		} else {
			return publicMatchmakingResponse(existing), nil
		}
	}
	if err != nil {
		return MatchmakingResponse{}, err
	}
	if queued.MatchedTicketID == "" {
		return publicMatchmakingResponse(queued), nil
	}
	opponent, err := s.matchmaking.GetMatchmakingTicket(ctx, matchmakingModeFriend, queued.MatchedTicketID)
	if err != nil {
		return MatchmakingResponse{}, err
	}
	room, err := s.CreateFriendRoom(ctx, userID)
	if err != nil {
		return MatchmakingResponse{}, err
	}
	if err := s.rooms.JoinFriendRoom(ctx, room.RoomCode, opponent.Player); err != nil {
		return MatchmakingResponse{}, err
	}
	room, err = s.GetFriendRoom(ctx, room.RoomCode)
	if err != nil {
		return MatchmakingResponse{}, err
	}
	matchID := room.RoomID
	room.MatchID = matchID
	room.StartAt = time.Now().UTC().Add(3 * time.Second).UnixMilli()
	room.Status = FriendRoomReady
	for index := range room.Players {
		room.Players[index].Ready = true
	}
	if lifecycle, ok := s.rooms.(FriendRoomLifecycleStore); ok {
		if _, err := lifecycle.SetFriendRoomReady(ctx, room.RoomCode, userID, true); err != nil {
			return MatchmakingResponse{}, err
		}
		if _, err := lifecycle.SetFriendRoomReady(ctx, room.RoomCode, opponent.UserID, true); err != nil {
			return MatchmakingResponse{}, err
		}
		started, err := lifecycle.StartFriendRoom(ctx, room.RoomCode, userID, room)
		if err != nil {
			return MatchmakingResponse{}, err
		}
		room = started
		matchID = room.MatchID
	}
	queued.Status, queued.MatchID, queued.Room = "matched", matchID, &room
	queued.Opponent = &opponent.Player
	opponent.Status, opponent.MatchID, opponent.Room = "matched", matchID, &room
	opponent.Opponent = &queued.Player
	if err := s.matchmaking.SaveMatchmakingPair(ctx, queued, opponent); err != nil {
		return MatchmakingResponse{}, err
	}
	return publicMatchmakingResponse(queued), nil
}

func (s *Service) GetMatchmakingStatus(ctx context.Context, userID uint64, ticketID string) (MatchmakingResponse, error) {
	if err := s.allowFriendRoomAction(ctx, userID, "matchmaking_status", 240, time.Minute); err != nil {
		return MatchmakingResponse{}, err
	}
	if s.matchmaking == nil {
		return MatchmakingResponse{}, apperror.ServiceUnavailable("快速匹配服务暂不可用", nil)
	}
	ticketID = strings.TrimSpace(ticketID)
	if ticketID == "" || len(ticketID) > 128 {
		return MatchmakingResponse{}, apperror.BadRequest("ticket_id is invalid", nil)
	}
	ticket, err := s.matchmaking.GetMatchmakingTicket(ctx, matchmakingModeFriend, ticketID)
	if errors.Is(err, ErrMatchmakingNotFound) {
		return MatchmakingResponse{}, apperror.NotFound("匹配票据不存在或已过期", err)
	}
	if err != nil {
		return MatchmakingResponse{}, err
	}
	if ticket.UserID != userID {
		return MatchmakingResponse{}, apperror.New(10004, 403, "无权查看该匹配票据", nil)
	}
	if ticket.Status == "searching" && time.Now().UTC().After(ticket.ExpiresAt) {
		if expiryStore, ok := s.matchmaking.(MatchmakingExpiryStore); ok {
			if err := expiryStore.MarkMatchmakingExpired(ctx, ticket); err != nil {
				return MatchmakingResponse{}, err
			}
		} else {
			_ = s.matchmaking.CancelMatchmaking(ctx, matchmakingModeFriend, ticket.TicketID, userID)
		}
		ticket.Status = "expired"
	} else if ticket.Status == "searching" && s.matchmakingWaitElapsed(ticket) {
		matched, matchErr := s.createBotMatch(ctx, ticket)
		if matchErr != nil {
			return MatchmakingResponse{}, matchErr
		}
		ticket = matched
	}
	if ticket.Status == "matched" && ticket.Room != nil && ticket.Room.RoomCode != "" {
		// The ticket contains a snapshot for atomic matchmaking writes. Refresh
		// the room before returning it so profile edits made after matching are
		// visible in the opponent card and the room player list.
		latestRoom, roomErr := s.GetFriendRoom(ctx, ticket.Room.RoomCode)
		if roomErr != nil {
			return MatchmakingResponse{}, roomErr
		}
		ticket.Room = &latestRoom
		ticket.Opponent = nil
		for index := range latestRoom.Players {
			if latestRoom.Players[index].UserID != userID {
				opponent := latestRoom.Players[index]
				ticket.Opponent = &opponent
				break
			}
		}
	}
	return publicMatchmakingResponse(ticket), nil
}

func (s *Service) CancelMatchmaking(ctx context.Context, userID uint64, ticketID string) (MatchmakingResponse, error) {
	if err := s.allowFriendRoomAction(ctx, userID, "matchmaking_cancel", 20, time.Minute); err != nil {
		return MatchmakingResponse{}, err
	}
	if s.matchmaking == nil {
		return MatchmakingResponse{}, apperror.ServiceUnavailable("快速匹配服务暂不可用", nil)
	}
	ticket, err := s.matchmaking.GetMatchmakingTicket(ctx, matchmakingModeFriend, strings.TrimSpace(ticketID))
	if errors.Is(err, ErrMatchmakingNotFound) {
		return MatchmakingResponse{}, apperror.NotFound("匹配票据不存在或已过期", err)
	}
	if err != nil {
		return MatchmakingResponse{}, err
	}
	if ticket.UserID != userID {
		return MatchmakingResponse{}, apperror.New(10004, 403, "无权取消该匹配票据", nil)
	}
	if ticket.Status == "searching" && time.Now().UTC().After(ticket.ExpiresAt) {
		if expiryStore, ok := s.matchmaking.(MatchmakingExpiryStore); ok {
			if err := expiryStore.MarkMatchmakingExpired(ctx, ticket); err != nil {
				return MatchmakingResponse{}, err
			}
		} else {
			_ = s.matchmaking.CancelMatchmaking(ctx, matchmakingModeFriend, ticket.TicketID, userID)
		}
		ticket.Status = "expired"
		return publicMatchmakingResponse(ticket), nil
	}
	if ticket.Status != "searching" {
		return publicMatchmakingResponse(ticket), nil
	}
	if err := s.matchmaking.CancelMatchmaking(ctx, matchmakingModeFriend, ticket.TicketID, userID); err != nil {
		return MatchmakingResponse{}, err
	}
	ticket.Status = "cancelled"
	return publicMatchmakingResponse(ticket), nil
}

func (s *Service) matchmakingWaitElapsed(ticket MatchmakingTicket) bool {
	wait := s.matchmakingWait
	if wait <= 0 {
		wait = defaultMatchmakingWait
	}
	return !ticket.CreatedAt.IsZero() && time.Since(ticket.CreatedAt) >= wait
}

func (s *Service) createBotMatch(ctx context.Context, ticket MatchmakingTicket) (MatchmakingTicket, error) {
	if s.rooms == nil {
		return MatchmakingTicket{}, apperror.ServiceUnavailable("match room service is unavailable", nil)
	}
	// Status polling is allowed from multiple devices and multiple API
	// instances. Only one request may create the fallback room and bot; the
	// other requests should observe the persisted ticket after the winner
	// finishes instead of creating duplicate rooms.
	release, lockErr := s.acquireSettlementLock(ctx, "matchmaking:bot:"+ticket.Mode+":"+ticket.TicketID)
	if lockErr != nil {
		var appErr *apperror.AppError
		if errors.As(lockErr, &appErr) && appErr.HTTPStatus == 409 {
			latest, getErr := s.matchmaking.GetMatchmakingTicket(ctx, ticket.Mode, ticket.TicketID)
			if getErr == nil {
				return latest, nil
			}
			return MatchmakingTicket{}, getErr
		}
		return MatchmakingTicket{}, lockErr
	}
	defer release()

	current, err := s.matchmaking.GetMatchmakingTicket(ctx, ticket.Mode, ticket.TicketID)
	if err != nil {
		return MatchmakingTicket{}, err
	}
	if current.Status != "searching" {
		return current, nil
	}
	room, err := s.CreateFriendRoom(ctx, ticket.UserID)
	if err != nil {
		return MatchmakingTicket{}, err
	}
	bot := FriendRoomPlayer{UserID: 0, Nickname: "对手", Ready: true}
	if err := s.rooms.JoinFriendRoom(ctx, room.RoomCode, bot); err != nil {
		return MatchmakingTicket{}, err
	}
	room, err = s.GetFriendRoom(ctx, room.RoomCode)
	if err != nil {
		return MatchmakingTicket{}, err
	}
	room.MatchID = room.RoomID
	room.Status = FriendRoomReady
	for index := range room.Players {
		room.Players[index].Ready = true
	}
	if lifecycle, ok := s.rooms.(FriendRoomLifecycleStore); ok {
		if _, err := lifecycle.SetFriendRoomReady(ctx, room.RoomCode, ticket.UserID, true); err != nil {
			return MatchmakingTicket{}, err
		}
		if _, err := lifecycle.SetFriendRoomReady(ctx, room.RoomCode, 0, true); err != nil {
			return MatchmakingTicket{}, err
		}
		started, err := lifecycle.StartFriendRoom(ctx, room.RoomCode, ticket.UserID, room)
		if err != nil {
			return MatchmakingTicket{}, err
		}
		room = started
	}
	difficulties := []string{"easy", "standard", "hard"}
	difficulty := difficulties[int(absMatchmakingInt64(room.RoomSeed))%len(difficulties)]
	botTicket := MatchmakingTicket{
		TicketID: "bot_" + ticket.TicketID, UserID: 0, Mode: ticket.Mode, RulesVersion: ticket.RulesVersion,
		Region: ticket.Region, Player: bot, Status: "matched", MatchID: room.MatchID, Room: &room,
		Opponent: &ticket.Player, CreatedAt: ticket.CreatedAt, ExpiresAt: time.Now().UTC().Add(matchmakingRetention),
		IsBot: true, BotDifficulty: difficulty,
	}
	current.Status, current.MatchID, current.Room, current.Opponent = "matched", room.MatchID, &room, &bot
	if err := s.matchmaking.SaveMatchmakingPair(ctx, current, botTicket); err != nil {
		return MatchmakingTicket{}, err
	}
	return current, nil
}

func botDisplayName(ticketID string) string {
	names := []string{"小林", "阿哲", "小雨", "安安", "小周"}
	var value int64
	for _, character := range ticketID {
		value = value*31 + int64(character)
	}
	return names[int(absMatchmakingInt64(value))%len(names)]
}

func absMatchmakingInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func publicMatchmakingResponse(ticket MatchmakingTicket) MatchmakingResponse {
	waitingSeconds := 0
	if ticket.Status == "searching" && !ticket.CreatedAt.IsZero() {
		waitingSeconds = maxInt(0, int(time.Since(ticket.CreatedAt)/time.Second))
	}
	return MatchmakingResponse{
		TicketID: ticket.TicketID, Mode: ticket.Mode, Status: ticket.Status,
		MatchID: ticket.MatchID, Room: ticket.Room, Opponent: ticket.Opponent,
		CreatedAt: ticket.CreatedAt, ExpiresAt: ticket.ExpiresAt, WaitingSeconds: waitingSeconds,
	}
}

func matchmakingQueueDiscriminator(ticket MatchmakingTicket) string {
	return fmt.Sprintf("%s:%d:%s", ticket.Mode, ticket.RulesVersion, strings.ToLower(strings.TrimSpace(ticket.Region)))
}

func randomMatchmakingTicketID() string {
	value, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return fmt.Sprintf("mm_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("mm_%x", value.Bytes())
}
