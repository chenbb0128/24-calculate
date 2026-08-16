package player

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/example/go-service/internal/apperror"
	"github.com/example/go-service/internal/store"
	db "github.com/example/go-service/internal/store/sqlc"
)

func (s *Service) SubmitLeaderboard(ctx context.Context, userID uint64, mode string, input SubmitLeaderboardInput) (SubmitLeaderboardResponse, error) {
	if mode == LeaderboardEndless || mode == LeaderboardFriend {
		return SubmitLeaderboardResponse{}, apperror.BadRequest("该排行榜必须通过对应对局接口提交", nil)
	}
	return s.submitLeaderboardRecord(ctx, userID, mode, input)
}

func (s *Service) submitLeaderboardRecord(ctx context.Context, userID uint64, mode string, input SubmitLeaderboardInput) (SubmitLeaderboardResponse, error) {
	if mode != LeaderboardEndless && mode != LeaderboardFriend {
		return SubmitLeaderboardResponse{}, apperror.BadRequest("leaderboard mode is invalid", nil)
	}
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if len(idempotencyKey) < 8 || len(idempotencyKey) > 128 {
		return SubmitLeaderboardResponse{}, apperror.BadRequest("idempotency_key length is invalid", nil)
	}
	if input.Score < 0 || input.Score > 50000000 {
		return SubmitLeaderboardResponse{}, apperror.BadRequest("score is out of range", nil)
	}
	if input.Questions < 0 || input.Questions > 100000 {
		return SubmitLeaderboardResponse{}, apperror.BadRequest("questions is out of range", nil)
	}
	if input.ElapsedMS < 0 || input.ElapsedMS > 86400000 {
		return SubmitLeaderboardResponse{}, apperror.BadRequest("elapsed_ms is out of range", nil)
	}
	roomID := strings.TrimSpace(input.RoomID)
	if len(roomID) > 128 {
		return SubmitLeaderboardResponse{}, apperror.BadRequest("room_id is too long", nil)
	}
	if mode == LeaderboardFriend && roomID == "" {
		return SubmitLeaderboardResponse{}, apperror.BadRequest("room_id is required for friend leaderboard", nil)
	}
	if mode == LeaderboardFriend && input.Outcome != "" && input.Outcome != "win" && input.Outcome != "lose" && input.Outcome != "draw" {
		return SubmitLeaderboardResponse{}, apperror.BadRequest("outcome is invalid", nil)
	}

	if _, err := s.profiles.GetProfile(ctx, userID); err != nil {
		return SubmitLeaderboardResponse{}, err
	}
	previous, err := s.store.GetLeaderboardSubmissionByKey(ctx, db.GetLeaderboardSubmissionByKeyParams{
		UserID:         userID,
		Mode:           mode,
		IdempotencyKey: idempotencyKey,
	})
	if err == nil {
		return toLeaderboardSubmissionResponse(previous, true), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return SubmitLeaderboardResponse{}, err
	}

	if mode == LeaderboardFriend {
		if err := s.verifyFriendRoomPlayer(ctx, userID, roomID); err != nil {
			return SubmitLeaderboardResponse{}, err
		}
	}
	metadata := input.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return SubmitLeaderboardResponse{}, apperror.BadRequest("metadata is invalid", err)
	}
	now := time.Now().UTC()
	createErr := s.store.CreateLeaderboardSubmission(ctx, db.CreateLeaderboardSubmissionParams{
		UserID:         userID,
		Mode:           mode,
		IdempotencyKey: idempotencyKey,
		Score:          uint32(input.Score),
		Questions:      uint32(input.Questions),
		ElapsedMs:      uint32(input.ElapsedMS),
		RoomID:         roomID,
		Outcome:        strings.TrimSpace(input.Outcome),
		MetadataJSON:   string(encoded),
		CreatedAt:      now,
	})
	if createErr != nil {
		if !store.IsDuplicateEntry(createErr) {
			return SubmitLeaderboardResponse{}, createErr
		}
		previous, err = s.store.GetLeaderboardSubmissionByKey(ctx, db.GetLeaderboardSubmissionByKeyParams{
			UserID:         userID,
			Mode:           mode,
			IdempotencyKey: idempotencyKey,
		})
		if err != nil {
			return SubmitLeaderboardResponse{}, err
		}
		return toLeaderboardSubmissionResponse(previous, true), nil
	}

	return SubmitLeaderboardResponse{
		Mode:                mode,
		IdempotencyKey:      idempotencyKey,
		Score:               input.Score,
		Questions:           input.Questions,
		ElapsedMS:           input.ElapsedMS,
		IdempotencyReplayed: false,
	}, nil
}

func (s *Service) verifyFriendRoomPlayer(ctx context.Context, userID uint64, roomID string) error {
	if s.rooms == nil {
		return apperror.ServiceUnavailable("好友房间服务暂不可用", nil)
	}
	roomCode := strings.TrimPrefix(roomID, "friend-")
	room, err := s.rooms.GetFriendRoom(ctx, roomCode)
	if errors.Is(err, ErrFriendRoomNotFound) {
		return apperror.NotFound("好友房间不存在或已过期", err)
	}
	if err != nil {
		return err
	}
	for _, player := range room.Players {
		if player.UserID == userID {
			return nil
		}
	}
	return apperror.New(10004, 403, "当前用户不属于该好友房间", nil)
}

func toLeaderboardSubmissionResponse(row db.PlayerLeaderboardSubmission, replayed bool) SubmitLeaderboardResponse {
	return SubmitLeaderboardResponse{
		Mode:                row.Mode,
		IdempotencyKey:      row.IdempotencyKey,
		Score:               int(row.Score),
		Questions:           int(row.Questions),
		ElapsedMS:           int(row.ElapsedMs),
		IdempotencyReplayed: replayed,
	}
}
