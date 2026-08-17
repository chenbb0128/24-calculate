package player

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/example/go-service/internal/apperror"
)

const (
	FriendRoomWaiting   = "waiting"
	FriendRoomReady     = "ready"
	FriendRoomCountdown = "countdown"
	FriendRoomRunning   = "running"
	FriendRoomFinished  = "finished"
	FriendRoomExpired   = "expired"
	FriendRoomCancelled = "cancelled"
	friendRoomTTL       = 30 * time.Minute
)

var friendRoomCodePattern = regexp.MustCompile(`^\d{6}$`)

type FriendRoomStore interface {
	CreateFriendRoom(ctx context.Context, room FriendRoom) error
	JoinFriendRoom(ctx context.Context, roomCode string, player FriendRoomPlayer) error
	GetFriendRoom(ctx context.Context, roomCode string) (FriendRoom, error)
	RemoveFriendRoomPlayer(ctx context.Context, roomCode string, userID uint64) error
	DeleteFriendRoom(ctx context.Context, roomCode string) error
	SaveFriendMatchProgress(ctx context.Context, roomCode string, progress FriendMatchProgress) error
	GetFriendMatchProgress(ctx context.Context, roomCode string) (map[uint64]FriendMatchProgress, error)
}

// FriendRoomLifecycleStore contains atomic room state transitions. It is kept
// separate so small in-memory stores used by older tests remain valid.
type FriendRoomLifecycleStore interface {
	SetFriendRoomReady(ctx context.Context, roomCode string, userID uint64, ready bool) (FriendRoom, error)
	StartFriendRoom(ctx context.Context, roomCode string, userID uint64, started FriendRoom) (FriendRoom, error)
	FinishFriendRoom(ctx context.Context, roomCode string, matchID string) error
}

type FriendMatchProgressEventStore interface {
	SaveFriendMatchProgressEvent(ctx context.Context, roomCode string, progress FriendMatchProgress) (bool, error)
}

type FriendRoomRateLimitStore interface {
	AllowFriendRoomAction(ctx context.Context, userID uint64, action string, limit int64, window time.Duration) (bool, error)
}

type FriendRoomPresenceStore interface {
	TouchFriendRoomPlayer(ctx context.Context, roomCode string, userID uint64) error
}

type FriendPuzzleHistoryStore interface {
	GetRecentFriendPuzzleHashes(ctx context.Context, userID uint64) (map[string]struct{}, error)
	RecordFriendPuzzleHashes(ctx context.Context, userID uint64, hashes []string, ttl time.Duration) error
}

type FriendMatchResultStore interface {
	SaveFriendMatchSubmission(context.Context, string, FriendMatchSubmissionRecord) error
	GetFriendMatchSubmissions(context.Context, string) (map[uint64]FriendMatchSubmissionRecord, error)
}

type FriendMatchSubmissionRecord struct {
	UserID         uint64    `json:"user_id"`
	Solved         int       `json:"solved"`
	Score          int       `json:"score"`
	Mistakes       int       `json:"mistakes"`
	ElapsedMS      int       `json:"elapsed_ms"`
	IdempotencyKey string    `json:"idempotency_key"`
	CreatedAt      time.Time `json:"created_at"`
}

type FriendRoom struct {
	Version        int                    `json:"version"`
	RoomID         string                 `json:"room_id"`
	RoomCode       string                 `json:"room_code"`
	MatchID        string                 `json:"match_id,omitempty"`
	Ranked         bool                   `json:"ranked"`
	SeasonID       string                 `json:"season_id,omitempty"`
	RankedEligible bool                   `json:"ranked_eligible"`
	MatchSource    string                 `json:"match_source,omitempty"`
	RoomSeed       int64                  `json:"room_seed"`
	OwnerID        uint64                 `json:"owner_id"`
	Status         string                 `json:"status"`
	StartAt        int64                  `json:"start_at,omitempty"`
	Rules          FriendRoomRules        `json:"rules"`
	QuestionHash   string                 `json:"question_hash"`
	PuzzleIDs      []string               `json:"puzzle_ids"`
	Puzzles        []FriendPuzzleContract `json:"puzzles"`
	Players        []FriendRoomPlayer     `json:"players"`
	CreatedAt      time.Time              `json:"created_at"`
	ExpiresAt      time.Time              `json:"expires_at"`
}

type FriendRoomRules struct {
	QuestionCount       int  `json:"question_count"`
	TimeLimitSeconds    int  `json:"time_limit"`
	Target              int  `json:"target"`
	NoHint              bool `json:"no_hint"`
	UseSameSeed         bool `json:"use_same_seed"`
	IntegerIntermediate bool `json:"integer_intermediate_results"`
}

type FriendRoomCreateInput struct {
	QuestionCount    int    `json:"question_count"`
	TimeLimitSeconds int    `json:"time_limit_seconds"`
	Ranked           bool   `json:"ranked"`
	SeasonID         string `json:"season_id"`
	MatchSource      string `json:"match_source"`
}

type FriendRoomPlayer struct {
	UserID       uint64    `json:"user_id,omitempty"`
	Nickname     string    `json:"nickname"`
	Avatar       string    `json:"avatar"`
	Ready        bool      `json:"ready"`
	LastSeenAt   time.Time `json:"last_seen_at,omitempty"`
	Disconnected bool      `json:"disconnected,omitempty"`
}

type FriendMatchProgress struct {
	UserID        uint64    `json:"user_id"`
	MatchID       string    `json:"match_id,omitempty"`
	QuestionHash  string    `json:"question_hash,omitempty"`
	EventID       string    `json:"event_id,omitempty"`
	QuestionIndex int       `json:"question_index"`
	Solved        int       `json:"solved"`
	Score         int       `json:"score"`
	ElapsedMS     int       `json:"elapsed_ms"`
	Finished      bool      `json:"finished"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type FriendMatchProgressInput struct {
	QuestionIndex int                      `json:"question_index"`
	Solved        int                      `json:"solved"`
	Score         int                      `json:"score"`
	ElapsedMS     int                      `json:"elapsed_ms"`
	Finished      bool                     `json:"finished"`
	MatchID       string                   `json:"match_id"`
	QuestionHash  string                   `json:"question_hash"`
	EventID       string                   `json:"event_id"`
	Attempt       *FriendMatchAttemptInput `json:"attempt,omitempty"`
}

type FriendRoomReadyInput struct {
	Ready bool `json:"ready"`
}

type FriendRoomReadyResponse struct {
	RoomCode string             `json:"room_code"`
	Status   string             `json:"status"`
	Players  []FriendRoomPlayer `json:"players"`
}

type FriendMatchStartResponse struct {
	MatchID        string                 `json:"match_id"`
	RoomID         string                 `json:"room_id"`
	RoomCode       string                 `json:"room_code"`
	Ranked         bool                   `json:"ranked"`
	SeasonID       string                 `json:"season_id,omitempty"`
	RankedEligible bool                   `json:"ranked_eligible"`
	RoomSeed       int64                  `json:"room_seed"`
	QuestionHash   string                 `json:"question_hash"`
	PuzzleIDs      []string               `json:"puzzle_ids"`
	Puzzles        []FriendPuzzleContract `json:"puzzles,omitempty"`
	QuestionCount  int                    `json:"question_count"`
	TimeLimit      int                    `json:"time_limit"`
	StartAt        int64                  `json:"start_at"`
	Status         string                 `json:"status"`
}

type FriendMatchPlayerState struct {
	UserID        uint64    `json:"user_id,omitempty"`
	Nickname      string    `json:"nickname"`
	Avatar        string    `json:"avatar"`
	QuestionIndex int       `json:"question_index"`
	Solved        int       `json:"solved"`
	Score         int       `json:"score"`
	ElapsedMS     int       `json:"elapsed_ms"`
	Finished      bool      `json:"finished"`
	UpdatedAt     time.Time `json:"updated_at"`
	LastSeenAt    time.Time `json:"last_seen_at,omitempty"`
	Disconnected  bool      `json:"disconnected,omitempty"`
	IsMe          bool      `json:"is_me"`
}

type FriendMatchProgressResponse struct {
	RoomID      string                   `json:"room_id"`
	RoomCode    string                   `json:"room_code"`
	Status      string                   `json:"status"`
	Players     []FriendMatchPlayerState `json:"players"`
	MatchResult *FriendMatchResult       `json:"match_result,omitempty"`
	RankResult  *RankResult              `json:"rank_result,omitempty"`
	RewardCoins int                      `json:"reward_coins,omitempty"`
	Coins       int                      `json:"coins,omitempty"`
	Progress    json.RawMessage          `json:"progress,omitempty"`
}

func NewServiceWithRooms(profiles ProfileReader, store Store, rooms FriendRoomStore) *Service {
	service := &Service{profiles: profiles, store: store, rooms: rooms}
	if locks, ok := rooms.(DistributedLockStore); ok {
		service.locks = locks
	}
	return service
}

func (s *Service) allowFriendRoomAction(ctx context.Context, userID uint64, action string, limit int64, window time.Duration) error {
	limiter, ok := s.rooms.(FriendRoomRateLimitStore)
	if !ok {
		return nil
	}
	allowed, err := limiter.AllowFriendRoomAction(ctx, userID, action, limit, window)
	if err != nil {
		return err
	}
	if !allowed {
		return apperror.New(10007, 429, "请求过于频繁，请稍后再试", nil)
	}
	return nil
}

func (s *Service) CreateFriendRoom(ctx context.Context, userID uint64) (FriendRoom, error) {
	return s.CreateFriendRoomWithRules(ctx, userID, FriendRoomCreateInput{})
}

func (s *Service) CreateFriendRoomWithRules(ctx context.Context, userID uint64, input FriendRoomCreateInput) (FriendRoom, error) {
	// Public room creation is always casual. Ranked rooms are created only by
	// the server's matchmaking path below; client-supplied ranked and season
	// fields are intentionally ignored here.
	return s.createFriendRoomWithRules(ctx, userID, input, false, "manual")
}

func (s *Service) createMatchmakingFriendRoom(ctx context.Context, userID uint64, ranked bool, seasonID string) (FriendRoom, error) {
	return s.createFriendRoomWithRules(ctx, userID, FriendRoomCreateInput{
		Ranked:   ranked,
		SeasonID: seasonID,
	}, true, "matchmaking")
}

func (s *Service) createBotFriendRoom(ctx context.Context, userID uint64) (FriendRoom, error) {
	return s.createFriendRoomWithRules(ctx, userID, FriendRoomCreateInput{}, false, "bot")
}

func (s *Service) createFriendRoomWithRules(ctx context.Context, userID uint64, input FriendRoomCreateInput, allowRanked bool, trustedSource string) (FriendRoom, error) {
	if s.rooms == nil {
		return FriendRoom{}, apperror.ServiceUnavailable("好友房间服务暂不可用", nil)
	}
	if err := s.allowFriendRoomAction(ctx, userID, "create", 10, time.Minute); err != nil {
		return FriendRoom{}, err
	}
	profile, err := s.profiles.GetProfile(ctx, userID)
	if err != nil {
		return FriendRoom{}, err
	}
	questionCount := input.QuestionCount
	if questionCount == 0 {
		questionCount = 8
	}
	if questionCount < 1 || questionCount > 16 {
		return FriendRoom{}, apperror.BadRequest("question_count must be between 1 and 16", nil)
	}
	timeLimitSeconds := input.TimeLimitSeconds
	if timeLimitSeconds == 0 {
		timeLimitSeconds = 120
	}
	if timeLimitSeconds < 30 || timeLimitSeconds > 600 {
		return FriendRoom{}, apperror.BadRequest("time_limit_seconds must be between 30 and 600", nil)
	}
	ranked := allowRanked && input.Ranked
	seasonID := ""
	if ranked {
		currentSeason := s.currentRankSeasonID()
		if strings.TrimSpace(input.SeasonID) != "" {
			requestedSeason, seasonErr := normalizeRankSeasonID(input.SeasonID)
			if seasonErr != nil || requestedSeason != currentSeason {
				return FriendRoom{}, apperror.New(10001, 409, "排位赛季已切换，请重新匹配", seasonErr)
			}
		}
		// The current season is selected by the server, never by the client.
		seasonID = currentSeason
	}
	matchSource := strings.TrimSpace(trustedSource)
	if matchSource == "" {
		matchSource = "manual"
	}

	now := time.Now().UTC()
	for attempt := 0; attempt < 5; attempt++ {
		code, seed, err := newFriendRoomIdentifiers()
		if err != nil {
			return FriendRoom{}, err
		}
		room := FriendRoom{
			Version:        1,
			RoomID:         "friend-" + code,
			RoomCode:       code,
			Ranked:         ranked,
			SeasonID:       seasonID,
			RankedEligible: ranked && matchSource == "matchmaking",
			MatchSource:    matchSource,
			RoomSeed:       seed,
			OwnerID:        userID,
			Status:         FriendRoomWaiting,
			Rules: FriendRoomRules{
				QuestionCount:       questionCount,
				TimeLimitSeconds:    timeLimitSeconds,
				Target:              24,
				NoHint:              true,
				UseSameSeed:         true,
				IntegerIntermediate: true,
			},
			Players: []FriendRoomPlayer{{
				UserID:     profile.ID,
				Nickname:   profile.Nickname,
				Avatar:     profile.Avatar,
				Ready:      false,
				LastSeenAt: now,
			}},
			CreatedAt: now,
			ExpiresAt: now.Add(friendRoomTTL),
		}
		excluded := map[string]struct{}(nil)
		if history, ok := s.rooms.(FriendPuzzleHistoryStore); ok {
			if recent, historyErr := history.GetRecentFriendPuzzleHashes(ctx, userID); historyErr == nil {
				excluded = recent
			}
		}
		// A room seed is the deterministic identity of its question contract.
		// Avoid recent questions by choosing another seed, rather than changing
		// the contract produced by an already chosen seed.
		room.QuestionHash, room.PuzzleIDs, room.Puzzles = friendRoomContract(room)
		for reroll := 0; reroll < 8 && friendPuzzleContractOverlapsRecentHistory(room.Puzzles, excluded); reroll++ {
			_, nextSeed, seedErr := newFriendRoomIdentifiers()
			if seedErr != nil {
				return FriendRoom{}, seedErr
			}
			room.RoomSeed = nextSeed
			room.QuestionHash, room.PuzzleIDs, room.Puzzles = friendRoomContract(room)
		}
		if err := s.rooms.CreateFriendRoom(ctx, room); err != nil {
			if errors.Is(err, ErrFriendRoomCodeTaken) {
				continue
			}
			return FriendRoom{}, err
		}
		if history, ok := s.rooms.(FriendPuzzleHistoryStore); ok {
			hashes := friendPuzzleHashes(room.Puzzles)
			_ = history.RecordFriendPuzzleHashes(ctx, userID, hashes, 14*24*time.Hour)
		}
		return room, nil
	}
	return FriendRoom{}, apperror.New(50001, 503, "暂时无法创建好友房间", nil)
}

func (s *Service) JoinFriendRoom(ctx context.Context, userID uint64, roomCode string) (FriendRoom, error) {
	if s.rooms == nil {
		return FriendRoom{}, apperror.ServiceUnavailable("好友房间服务暂不可用", nil)
	}
	if err := s.allowFriendRoomAction(ctx, userID, "join", 10, time.Minute); err != nil {
		return FriendRoom{}, err
	}
	roomCode = strings.TrimSpace(roomCode)
	if !friendRoomCodePattern.MatchString(roomCode) {
		return FriendRoom{}, apperror.BadRequest("room_code is invalid", nil)
	}
	profile, err := s.profiles.GetProfile(ctx, userID)
	if err != nil {
		return FriendRoom{}, err
	}
	room, err := s.rooms.GetFriendRoom(ctx, roomCode)
	if err != nil {
		return FriendRoom{}, mapFriendRoomError(err)
	}
	if !room.ExpiresAt.IsZero() && time.Now().UTC().After(room.ExpiresAt) {
		return FriendRoom{}, apperror.New(10005, 410, "好友房间已过期", ErrFriendRoomExpired)
	}
	if room.OwnerID == userID {
		return FriendRoom{}, apperror.New(10003, 409, "不能加入自己创建的好友房间", ErrFriendRoomCannotJoinSelf)
	}
	if friendRoomHasPlayer(room, userID) {
		return FriendRoom{}, apperror.New(10003, 409, "已经在该好友房间中", ErrFriendRoomAlreadyIn)
	}
	if friendRoomIsTerminal(room.Status) || room.Status == FriendRoomCountdown || room.Status == FriendRoomRunning || room.Status == FriendRoomFinished {
		return FriendRoom{}, mapFriendRoomError(ErrFriendRoomStarted)
	}
	if len(room.Players) >= 2 {
		return FriendRoom{}, apperror.New(10003, 409, "好友房间已满", ErrFriendRoomFull)
	}
	player := FriendRoomPlayer{UserID: profile.ID, Nickname: profile.Nickname, Avatar: profile.Avatar, Ready: false, LastSeenAt: time.Now().UTC()}
	if err := s.rooms.JoinFriendRoom(ctx, roomCode, player); err != nil {
		return FriendRoom{}, mapFriendRoomError(err)
	}
	if history, ok := s.rooms.(FriendPuzzleHistoryStore); ok {
		_ = history.RecordFriendPuzzleHashes(ctx, userID, friendPuzzleHashes(room.Puzzles), 14*24*time.Hour)
	}
	return s.GetFriendRoom(ctx, roomCode)
}

func (s *Service) ReadyFriendRoom(ctx context.Context, userID uint64, roomCode string, ready bool) (FriendRoom, error) {
	if err := s.allowFriendRoomAction(ctx, userID, "ready", 30, time.Minute); err != nil {
		return FriendRoom{}, err
	}
	lifecycle, ok := s.rooms.(FriendRoomLifecycleStore)
	if !ok {
		return FriendRoom{}, apperror.ServiceUnavailable("好友房间状态服务暂不可用", nil)
	}
	room, err := s.GetFriendRoom(ctx, roomCode)
	if err != nil {
		return FriendRoom{}, err
	}
	if !friendRoomHasPlayer(room, userID) {
		return FriendRoom{}, apperror.New(10004, 403, "当前用户不属于该好友房间", ErrFriendRoomNotMember)
	}
	updated, err := lifecycle.SetFriendRoomReady(ctx, room.RoomCode, userID, ready)
	if err != nil {
		// A concurrent second ready request may win the transition between the
		// read and this Lua script. Returning the persisted countdown is
		// idempotent and avoids making clients retry a successful action.
		if errors.Is(err, ErrFriendRoomStarted) {
			current, getErr := s.GetFriendRoom(ctx, room.RoomCode)
			if getErr == nil && (current.Status == FriendRoomCountdown || current.Status == FriendRoomRunning || current.Status == FriendRoomFinished) {
				return current, nil
			}
		}
		return FriendRoom{}, mapFriendRoomError(err)
	}
	if ready && len(updated.Players) == 2 && friendRoomAllReady(updated) {
		started, startErr := s.startFriendRoom(ctx, userID, updated.RoomCode)
		if startErr == nil {
			return started, nil
		}
		// Another request may have completed the same transition. Read the
		// canonical room and return it instead of creating a second contract.
		if errors.Is(startErr, ErrFriendRoomAlreadyStarted) || errors.Is(startErr, ErrFriendRoomNotReady) {
			current, getErr := s.GetFriendRoom(ctx, updated.RoomCode)
			if getErr == nil && (current.Status == FriendRoomCountdown || current.Status == FriendRoomRunning || current.Status == FriendRoomFinished) {
				return current, nil
			}
		}
		return FriendRoom{}, mapFriendRoomError(startErr)
	}
	return updated, nil
}

func (s *Service) StartFriendRoom(ctx context.Context, userID uint64, roomCode string) (FriendMatchStartResponse, error) {
	if err := s.allowFriendRoomAction(ctx, userID, "start", 10, time.Minute); err != nil {
		return FriendMatchStartResponse{}, err
	}
	room, err := s.startFriendRoom(ctx, userID, roomCode)
	if err != nil {
		return FriendMatchStartResponse{}, mapFriendRoomError(err)
	}
	return friendMatchStartResponse(room), nil
}

func (s *Service) startFriendRoom(ctx context.Context, userID uint64, roomCode string) (FriendRoom, error) {
	lifecycle, ok := s.rooms.(FriendRoomLifecycleStore)
	if !ok {
		return FriendRoom{}, apperror.ServiceUnavailable("好友房间状态服务暂不可用", nil)
	}
	room, err := s.GetFriendRoom(ctx, roomCode)
	if err != nil {
		return FriendRoom{}, err
	}
	if !friendRoomHasPlayer(room, userID) {
		return FriendRoom{}, apperror.New(10004, 403, "当前用户不属于该好友房间", ErrFriendRoomNotMember)
	}
	if friendRoomIsTerminal(room.Status) {
		return FriendRoom{}, mapFriendRoomError(errForFriendRoomStatus(room.Status))
	}
	if room.Status == FriendRoomCountdown || room.Status == FriendRoomRunning || room.Status == FriendRoomFinished {
		if room.MatchID == "" {
			room.MatchID = room.RoomID
		}
		return room, nil
	}
	if len(room.Players) != 2 || !friendRoomAllReady(room) {
		return FriendRoom{}, apperror.New(10003, 409, "双方准备后才能开始对战", ErrFriendRoomNotReady)
	}

	questionHash, puzzleIDs, puzzles := friendRoomContract(room)
	room.MatchID = room.RoomID
	room.QuestionHash = questionHash
	room.PuzzleIDs = puzzleIDs
	room.Puzzles = puzzles
	room.StartAt = time.Now().UTC().Add(3 * time.Second).UnixMilli()
	room.Status = FriendRoomCountdown
	started, err := lifecycle.StartFriendRoom(ctx, room.RoomCode, userID, room)
	if err != nil {
		return FriendRoom{}, err
	}
	return started, nil
}

func friendMatchStartResponse(room FriendRoom) FriendMatchStartResponse {
	questionCount := room.Rules.QuestionCount
	if questionCount <= 0 {
		questionCount = len(room.PuzzleIDs)
	}
	if questionCount <= 0 {
		questionCount = 8
	}
	timeLimit := room.Rules.TimeLimitSeconds
	if timeLimit <= 0 {
		timeLimit = 120
	}
	matchID := room.MatchID
	if matchID == "" {
		matchID = room.RoomID
	}
	return FriendMatchStartResponse{
		MatchID: matchID, RoomID: room.RoomID, RoomCode: room.RoomCode,
		Ranked: room.Ranked, SeasonID: room.SeasonID, RankedEligible: room.RankedEligible,
		RoomSeed: room.RoomSeed, QuestionHash: room.QuestionHash,
		PuzzleIDs: append([]string(nil), room.PuzzleIDs...), QuestionCount: questionCount,
		Puzzles:   append([]FriendPuzzleContract(nil), room.Puzzles...),
		TimeLimit: timeLimit, StartAt: room.StartAt, Status: room.Status,
	}
}

func (s *Service) GetFriendRoomForUser(ctx context.Context, userID uint64, roomCode string) (FriendRoom, error) {
	if err := s.allowFriendRoomAction(ctx, userID, "room_get", 240, time.Minute); err != nil {
		return FriendRoom{}, err
	}
	room, err := s.GetFriendRoom(ctx, roomCode)
	if err != nil {
		return FriendRoom{}, err
	}
	if !friendRoomHasPlayer(room, userID) {
		return FriendRoom{}, apperror.New(10004, 403, "当前用户不属于该好友房间", ErrFriendRoomNotMember)
	}
	if presence, ok := s.rooms.(FriendRoomPresenceStore); ok {
		if err := presence.TouchFriendRoomPlayer(ctx, room.RoomCode, userID); err != nil {
			return FriendRoom{}, err
		}
		now := time.Now().UTC()
		for index := range room.Players {
			if room.Players[index].UserID == userID {
				room.Players[index].LastSeenAt = now
				room.Players[index].Disconnected = false
			}
		}
	}
	return room, nil
}

func (s *Service) GetFriendRoom(ctx context.Context, roomCode string) (FriendRoom, error) {
	if s.rooms == nil {
		return FriendRoom{}, apperror.ServiceUnavailable("好友房间服务暂不可用", nil)
	}
	roomCode = strings.TrimSpace(roomCode)
	if !friendRoomCodePattern.MatchString(roomCode) {
		return FriendRoom{}, apperror.BadRequest("room_code is invalid", nil)
	}
	room, err := s.rooms.GetFriendRoom(ctx, roomCode)
	if errors.Is(err, ErrFriendRoomNotFound) {
		return FriendRoom{}, apperror.NotFound("好友房间不存在或已过期", err)
	}
	if err != nil {
		return FriendRoom{}, err
	}
	if room.QuestionHash == "" || len(room.PuzzleIDs) == 0 || len(room.Puzzles) == 0 {
		room.QuestionHash, room.PuzzleIDs, room.Puzzles = friendRoomContract(room)
	}
	for index := range room.Players {
		// Redis stores a room snapshot for fast polling, but profile edits must
		// be visible immediately in room and match-result responses. AI players
		// use user_id 0 and intentionally keep the neutral public display name.
		if room.Players[index].UserID != 0 && s.profiles != nil {
			if profile, profileErr := s.profiles.GetProfile(ctx, room.Players[index].UserID); profileErr == nil {
				room.Players[index].Nickname = profile.Nickname
				room.Players[index].Avatar = profile.Avatar
			}
		}
		room.Players[index].Nickname = normalizePublicNickname(room.Players[index].Nickname)
		room.Players[index].Avatar = normalizePublicAvatar(room.Players[index].Avatar)
	}
	return room, nil
}

func (s *Service) LeaveFriendRoom(ctx context.Context, userID uint64, roomCode string) error {
	if err := s.allowFriendRoomAction(ctx, userID, "leave", 20, time.Minute); err != nil {
		return err
	}
	if s.rooms == nil {
		return apperror.ServiceUnavailable("濂藉弸鎴块棿鏈嶅姟鏆備笉鍙敤", nil)
	}
	room, err := s.GetFriendRoom(ctx, roomCode)
	if err != nil {
		return err
	}
	if !friendRoomHasPlayer(room, userID) {
		return apperror.New(10004, 403, "褰撳墠鐢ㄦ埛涓嶅睘浜庤濂藉弸鎴块棿", nil)
	}
	if room.OwnerID == userID {
		return s.rooms.DeleteFriendRoom(ctx, room.RoomCode)
	}
	return s.rooms.RemoveFriendRoomPlayer(ctx, room.RoomCode, userID)
}

func (s *Service) UpdateFriendMatchProgress(ctx context.Context, userID uint64, roomCode string, input FriendMatchProgressInput) (FriendMatchProgressResponse, error) {
	if err := s.allowFriendRoomAction(ctx, userID, "progress", 180, time.Minute); err != nil {
		return FriendMatchProgressResponse{}, err
	}
	room, err := s.GetFriendRoom(ctx, roomCode)
	if err != nil {
		return FriendMatchProgressResponse{}, err
	}
	if !friendRoomHasPlayer(room, userID) {
		return FriendMatchProgressResponse{}, apperror.New(10004, 403, "当前用户不属于该好友房间", nil)
	}
	if room.MatchID == "" || (room.Status != FriendRoomCountdown && room.Status != FriendRoomRunning) {
		return FriendMatchProgressResponse{}, mapFriendRoomError(errForFriendRoomStatus(room.Status))
	}
	if existingProgress, progressErr := s.rooms.GetFriendMatchProgress(ctx, room.RoomCode); progressErr == nil {
		for playerID, current := range existingProgress {
			if playerID != userID && current.Finished {
				return FriendMatchProgressResponse{}, apperror.New(10003, 409, "对手已完成本局，不能继续提交", ErrFriendRoomAlreadyStarted)
			}
		}
	}
	if !friendMatchContractMatches(room, input.MatchID, input.QuestionHash) {
		return FriendMatchProgressResponse{}, apperror.BadRequest("friend match progress contract is invalid", nil)
	}
	questionCount := room.Rules.QuestionCount
	if questionCount <= 0 {
		questionCount = 8
	}
	if input.QuestionIndex < 0 || input.QuestionIndex >= questionCount {
		return FriendMatchProgressResponse{}, apperror.BadRequest("question_index is out of range", nil)
	}
	if input.Solved < 0 || input.Solved > questionCount || input.Solved > input.QuestionIndex+1 {
		return FriendMatchProgressResponse{}, apperror.BadRequest("solved is out of range", nil)
	}
	if input.Score < 0 || input.Score > 50000000 {
		return FriendMatchProgressResponse{}, apperror.BadRequest("score is out of range", nil)
	}
	if input.Score > input.Solved*1000 {
		return FriendMatchProgressResponse{}, apperror.BadRequest("score is outside the server range", nil)
	}
	timeLimit := room.Rules.TimeLimitSeconds
	if timeLimit <= 0 {
		timeLimit = 120
	}
	if input.ElapsedMS < 0 || input.ElapsedMS > timeLimit*1000 {
		return FriendMatchProgressResponse{}, apperror.BadRequest("elapsed_ms is out of range", nil)
	}
	if input.EventID != "" && len(input.EventID) > 256 {
		return FriendMatchProgressResponse{}, apperror.BadRequest("event_id is invalid", nil)
	}
	if input.Attempt != nil {
		if err := validateFriendProgressAttempt(room, input); err != nil {
			return FriendMatchProgressResponse{}, err
		}
	}
	previous, exists, err := s.friendMatchPlayerProgress(ctx, room.RoomCode, userID)
	if err != nil {
		return FriendMatchProgressResponse{}, err
	}
	if exists {
		if input.QuestionIndex < previous.QuestionIndex ||
			input.Solved < previous.Solved ||
			input.Score < previous.Score ||
			input.ElapsedMS < previous.ElapsedMS ||
			(previous.Finished && !input.Finished) {
			return FriendMatchProgressResponse{}, apperror.BadRequest("friend match progress cannot move backwards", nil)
		}
	}

	progress := FriendMatchProgress{
		UserID: userID, MatchID: room.MatchID, QuestionHash: room.QuestionHash,
		EventID:       strings.TrimSpace(input.EventID),
		QuestionIndex: input.QuestionIndex,
		Solved:        input.Solved,
		Score:         input.Score,
		ElapsedMS:     input.ElapsedMS,
		Finished:      input.Finished,
		UpdatedAt:     time.Now().UTC(),
	}
	if eventStore, ok := s.rooms.(FriendMatchProgressEventStore); ok {
		accepted, saveErr := eventStore.SaveFriendMatchProgressEvent(ctx, room.RoomCode, progress)
		if saveErr != nil {
			if errors.Is(saveErr, ErrFriendMatchProgressStale) {
				return FriendMatchProgressResponse{}, apperror.BadRequest("friend match progress cannot move backwards", saveErr)
			}
			return FriendMatchProgressResponse{}, saveErr
		}
		if !accepted {
			return FriendMatchProgressResponse{}, apperror.BadRequest("friend match event_id has already been processed or progress is stale", nil)
		}
	} else {
		if exists && input.EventID != "" && input.EventID == previous.EventID {
			return FriendMatchProgressResponse{}, apperror.BadRequest("friend match event_id has already been processed", nil)
		}
		if err := s.rooms.SaveFriendMatchProgress(ctx, room.RoomCode, progress); err != nil {
			return FriendMatchProgressResponse{}, err
		}
	}
	if input.Finished {
		if lifecycle, ok := s.rooms.(FriendRoomLifecycleStore); ok {
			if finishErr := lifecycle.FinishFriendRoom(ctx, room.RoomCode, room.MatchID); finishErr != nil && !errors.Is(finishErr, ErrFriendRoomStarted) {
				return FriendMatchProgressResponse{}, finishErr
			}
			room.Status = FriendRoomFinished
		}
	}
	return s.friendMatchProgressResponse(ctx, userID, room)
}

func (s *Service) friendMatchPlayerProgress(ctx context.Context, roomCode string, userID uint64) (FriendMatchProgress, bool, error) {
	progress, err := s.rooms.GetFriendMatchProgress(ctx, roomCode)
	if err != nil {
		return FriendMatchProgress{}, false, err
	}
	current, exists := progress[userID]
	return current, exists, nil
}

func (s *Service) GetFriendMatchProgress(ctx context.Context, userID uint64, roomCode string) (FriendMatchProgressResponse, error) {
	if err := s.allowFriendRoomAction(ctx, userID, "progress_get", 240, time.Minute); err != nil {
		return FriendMatchProgressResponse{}, err
	}
	room, err := s.GetFriendRoom(ctx, roomCode)
	if err != nil {
		return FriendMatchProgressResponse{}, err
	}
	if !friendRoomHasPlayer(room, userID) {
		return FriendMatchProgressResponse{}, apperror.New(10004, 403, "当前用户不属于该好友房间", nil)
	}
	if presence, ok := s.rooms.(FriendRoomPresenceStore); ok {
		if err := presence.TouchFriendRoomPlayer(ctx, room.RoomCode, userID); err != nil {
			return FriendMatchProgressResponse{}, err
		}
		now := time.Now().UTC()
		for index := range room.Players {
			if room.Players[index].UserID == userID {
				room.Players[index].LastSeenAt = now
				room.Players[index].Disconnected = false
			}
		}
	}
	if err := s.advanceFriendBot(ctx, room); err != nil {
		return FriendMatchProgressResponse{}, err
	}
	if latest, refreshErr := s.GetFriendRoom(ctx, room.RoomCode); refreshErr == nil {
		room = latest
	}
	result, err := s.friendMatchProgressResponse(ctx, userID, room)
	if err != nil {
		return FriendMatchProgressResponse{}, err
	}
	matchResult, reward, progress, err := s.resolveFriendMatchForUser(ctx, userID, room)
	if err != nil {
		return FriendMatchProgressResponse{}, err
	}
	result.MatchResult = matchResult
	if matchResult != nil {
		result.RankResult = matchResult.RankResult
	}
	result.RewardCoins = reward
	if len(progress) > 0 {
		result.Progress = progress
		result.Coins = progressCoins(string(progress))
	}
	return result, nil
}

func (s *Service) resolveFriendMatchForUser(ctx context.Context, userID uint64, room FriendRoom) (*FriendMatchResult, int, json.RawMessage, error) {
	resultStore, ok := s.rooms.(FriendMatchResultStore)
	if !ok {
		return nil, 0, nil, nil
	}
	submissions, err := resultStore.GetFriendMatchSubmissions(ctx, room.RoomCode)
	if err != nil || len(submissions) < 2 {
		return nil, 0, nil, err
	}
	current, exists := submissions[userID]
	if !exists {
		return nil, 0, nil, nil
	}
	var opponent FriendMatchSubmissionRecord
	found := false
	for candidateID, candidate := range submissions {
		if candidateID != userID {
			opponent = candidate
			found = true
			break
		}
	}
	if !found {
		return nil, 0, nil, nil
	}
	outcome := compareFriendResults(current, opponent)
	matchResult := &FriendMatchResult{
		Outcome: outcome, PlayerSolved: current.Solved, PlayerScore: current.Score, PlayerMistakes: current.Mistakes,
		PlayerElapsedMS: current.ElapsedMS, PlayerElapsed: float64(current.ElapsedMS) / 1000,
		OpponentSolved: opponent.Solved, OpponentScore: opponent.Score, OpponentMistakes: opponent.Mistakes,
		OpponentElapsedMS: opponent.ElapsedMS, OpponentElapsed: float64(opponent.ElapsedMS) / 1000,
	}
	rankSettlement, err := s.settleRankedFriendMatch(ctx, userID, room, submissions)
	if err != nil {
		return nil, 0, nil, err
	}
	if rankSettlement != nil {
		matchResult.RankResult = &rankSettlement.Result
	}
	reward := 0
	progress, err := s.store.MutatePlayerProgress(ctx, userID, func(state map[string]any) error {
		var applyErr error
		reward, applyErr = applyFriendServerResult(state, *matchResult, time.Now().In(shanghaiLocation).Format("2006-01-02"), room.RoomID)
		return applyErr
	})
	if err != nil {
		return nil, 0, nil, err
	}
	if rankSettlement != nil {
		var progressWithRankValue string
		progressWithRankValue, err = progressWithRank(string(progress), rankView(rankSettlement.Profile))
		if err != nil {
			return nil, 0, nil, err
		}
		progress = json.RawMessage(progressWithRankValue)
	}
	return matchResult, reward, progress, nil
}

func (s *Service) friendMatchProgressResponse(ctx context.Context, userID uint64, room FriendRoom) (FriendMatchProgressResponse, error) {
	progress, err := s.rooms.GetFriendMatchProgress(ctx, room.RoomCode)
	if err != nil {
		return FriendMatchProgressResponse{}, err
	}
	players := make([]FriendMatchPlayerState, 0, len(room.Players))
	for _, player := range room.Players {
		current := progress[player.UserID]
		players = append(players, FriendMatchPlayerState{
			UserID:        player.UserID,
			Nickname:      player.Nickname,
			Avatar:        player.Avatar,
			QuestionIndex: current.QuestionIndex,
			Solved:        current.Solved,
			Score:         current.Score,
			ElapsedMS:     current.ElapsedMS,
			Finished:      current.Finished,
			UpdatedAt:     current.UpdatedAt,
			LastSeenAt:    player.LastSeenAt,
			Disconnected:  player.Disconnected,
			IsMe:          player.UserID == userID,
		})
	}
	return FriendMatchProgressResponse{
		RoomID:   room.RoomID,
		RoomCode: room.RoomCode,
		Status:   room.Status,
		Players:  players,
	}, nil
}

func friendRoomHasPlayer(room FriendRoom, userID uint64) bool {
	for _, player := range room.Players {
		if player.UserID == userID {
			return true
		}
	}
	return false
}

func friendRoomAllReady(room FriendRoom) bool {
	if len(room.Players) != 2 {
		return false
	}
	for _, player := range room.Players {
		if !player.Ready {
			return false
		}
	}
	return true
}

func friendRoomIsTerminal(status string) bool {
	switch status {
	case FriendRoomExpired, FriendRoomCancelled:
		return true
	default:
		return false
	}
}

func errForFriendRoomStatus(status string) error {
	switch status {
	case FriendRoomExpired:
		return ErrFriendRoomExpired
	case FriendRoomCancelled:
		return ErrFriendRoomCancelled
	case FriendRoomCountdown, FriendRoomRunning, FriendRoomFinished:
		return ErrFriendRoomStarted
	default:
		return ErrFriendRoomNotReady
	}
}

func mapFriendRoomError(err error) error {
	switch {
	case errors.Is(err, ErrFriendRoomNotFound):
		return apperror.NotFound("好友房间不存在或已过期", err)
	case errors.Is(err, ErrFriendRoomExpired):
		return apperror.New(10005, 410, "好友房间已过期", err)
	case errors.Is(err, ErrFriendRoomCancelled):
		return apperror.New(10006, 410, "好友房间已取消", err)
	case errors.Is(err, ErrFriendRoomFull):
		return apperror.New(10003, 409, "好友房间已满", err)
	case errors.Is(err, ErrFriendRoomAlreadyIn):
		return apperror.New(10003, 409, "已经在该好友房间中", err)
	case errors.Is(err, ErrFriendRoomCannotJoinSelf):
		return apperror.New(10003, 409, "不能加入自己创建的好友房间", err)
	case errors.Is(err, ErrFriendRoomStarted), errors.Is(err, ErrFriendRoomAlreadyStarted):
		return apperror.New(10003, 409, "好友房间已经开始", err)
	case errors.Is(err, ErrFriendRoomNotMember):
		return apperror.New(10004, 403, "当前用户不属于该好友房间", err)
	case errors.Is(err, ErrFriendRoomNotReady):
		return apperror.New(10003, 409, "双方准备后才能开始对战", err)
	default:
		return err
	}
}

func friendMatchContractMatches(room FriendRoom, matchID, questionHash string) bool {
	matchID = strings.TrimSpace(matchID)
	questionHash = strings.TrimSpace(questionHash)
	expectedMatchID := room.MatchID
	if expectedMatchID == "" {
		expectedMatchID = room.RoomID
	}
	expectedHash := room.QuestionHash
	if expectedHash == "" {
		expectedHash, _, _ = friendRoomContract(room)
	}
	return matchID == expectedMatchID && questionHash == expectedHash
}

func validateFriendProgressAttempt(room FriendRoom, input FriendMatchProgressInput) error {
	attempt := input.Attempt
	if attempt.ProtocolVersion != friendMatchProtocolVersion || attempt.QuestionIndex != input.QuestionIndex ||
		attempt.RoomSeed != room.RoomSeed || attempt.QuestionHash != input.QuestionHash ||
		attempt.PuzzleID == "" || input.QuestionIndex < 0 || input.QuestionIndex >= len(room.Puzzles) ||
		attempt.PuzzleID != room.Puzzles[input.QuestionIndex].PuzzleID {
		return apperror.BadRequest("friend match progress attempt is invalid", nil)
	}
	if attempt.ElapsedMS < 0 || attempt.ElapsedMS > room.Rules.TimeLimitSeconds*1000 || attempt.Mistakes < 0 || attempt.Score < 0 || attempt.ScoreDelta < 0 {
		return apperror.BadRequest("friend match progress attempt is invalid", nil)
	}
	if input.EventID != "" && input.EventID != attempt.EventID {
		return apperror.BadRequest("friend match progress event_id is invalid", nil)
	}
	if attempt.Solved && !replayFriendSolution(room.Puzzles[input.QuestionIndex].Numbers, attempt.SolutionSteps, room.Puzzles[input.QuestionIndex].Rules) {
		return apperror.BadRequest("friend match progress solution is invalid", nil)
	}
	return nil
}

func newFriendRoomIdentifiers() (string, int64, error) {
	codeValue, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "", 0, fmt.Errorf("generate friend room code: %w", err)
	}
	seedValue, err := rand.Int(rand.Reader, big.NewInt(2147483647))
	if err != nil {
		return "", 0, fmt.Errorf("generate friend room seed: %w", err)
	}
	return strconv.FormatInt(100000+codeValue.Int64(), 10), seedValue.Int64(), nil
}
