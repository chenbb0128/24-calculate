package player

import (
	"context"
	"errors"
	"time"
)

// advanceFriendBot writes a bot's progress into the same Redis structures as
// a real player while retaining one-question-at-a-time server timing. It is
// called by both API requests and the background ticker.
func (s *Service) advanceFriendBot(ctx context.Context, room FriendRoom) error {
	if room.Status != FriendRoomRunning || room.StartAt <= 0 || s.rooms == nil {
		return nil
	}
	botFound := false
	for _, player := range room.Players {
		if player.UserID == 0 {
			botFound = true
			break
		}
	}
	if !botFound {
		return nil
	}
	all, err := s.getFriendMatchProgress(ctx, room)
	if err != nil {
		return err
	}
	previous := all[0]
	count := room.Rules.QuestionCount
	if count <= 0 {
		count = len(room.Puzzles)
	}
	if count <= 0 {
		count = friendQuestionCount
	}
	elapsed := time.Now().UTC().UnixMilli() - room.StartAt
	if elapsed < 0 {
		return nil
	}

	difficulty := int(absMatchmakingInt64(room.RoomSeed) % 3)
	solved, usedMS := botProgressForElapsed(previous.Solved, elapsed, count, room.RoomSeed, difficulty)
	if solved < previous.Solved {
		return nil
	}
	if solved == previous.Solved && previous.UpdatedAt.After(time.Now().UTC().Add(-2*time.Second)) {
		return nil
	}
	finished := solved >= count
	progress := FriendMatchProgress{
		UserID: 0, RoundID: room.RoundID, MatchID: room.MatchID, QuestionHash: room.QuestionHash,
		QuestionIndex: maxInt(0, solved-1), Solved: solved,
		Score: solved * 100, ElapsedMS: int(usedMS), Finished: finished, UpdatedAt: time.Now().UTC(),
	}
	if err := s.saveFriendMatchProgress(ctx, room, progress); err != nil {
		return err
	}
	if finished {
		_ = s.saveFriendMatchSubmission(ctx, room, FriendMatchSubmissionRecord{
			UserID: 0, RoundID: room.RoundID, Solved: solved, Score: progress.Score, ElapsedMS: progress.ElapsedMS,
			CreatedAt: time.Now().UTC(),
		})
		if lifecycle, ok := s.rooms.(FriendRoomLifecycleStore); ok {
			if err := lifecycle.FinishFriendRoom(ctx, room.RoomCode, room.MatchID); err != nil && !errors.Is(err, ErrFriendRoomStarted) {
				return err
			}
		}
	}
	return nil
}

// StartFriendBotBackground runs the server-side bot clock independently from
// client polling. It is intentionally attached to the API context so deploys
// stop it cleanly, and it only touches rooms indexed by the matchmaking bot
// path.
func (s *Service) StartFriendBotBackground(ctx context.Context) {
	index, ok := s.rooms.(FriendBotRoomStore)
	if !ok {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		advance := func() {
			roomCodes, err := index.ListFriendBotRooms(ctx)
			if err != nil {
				return
			}
			for _, roomCode := range roomCodes {
				room, getErr := s.GetFriendRoom(ctx, roomCode)
				if getErr != nil {
					_ = index.RemoveFriendBotRoom(ctx, roomCode)
					continue
				}
				if room.Status == FriendRoomRunning || room.Status == FriendRoomCountdown {
					_ = s.advanceFriendBot(ctx, room)
				}
				if room.Status == FriendRoomFinished || friendRoomIsTerminal(room.Status) {
					_ = index.RemoveFriendBotRoom(ctx, roomCode)
				}
			}
		}
		advance()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				advance()
			}
		}
	}()
}

func botProgressForElapsed(previousSolved int, elapsed int64, count int, seed int64, difficulty int) (int, int64) {
	if count <= 0 || elapsed < 0 {
		return previousSolved, 0
	}
	if difficulty < 0 || difficulty > 2 {
		difficulty = 1
	}
	base := []int64{11000, 8000, 5000}[difficulty]
	targetSolved := 0
	elapsedForTarget := int64(0)
	for index := 0; index < count; index++ {
		jitter := (absMatchmakingInt64(seed)+int64(index*7919))%3500 - 1750
		duration := base + jitter
		if duration < 3000 {
			duration = 3000
		}
		elapsedForTarget += duration
		if elapsedForTarget > elapsed {
			break
		}
		targetSolved++
	}
	previousSolved = maxInt(0, minInt(previousSolved, count))
	solved := previousSolved
	if targetSolved > previousSolved {
		// Persist at most one new answer per status poll. This prevents a slow
		// client from observing an artificial multi-question jump.
		solved++
	}
	usedMS := int64(0)
	for index := 0; index < solved; index++ {
		jitter := (absMatchmakingInt64(seed)+int64(index*7919))%3500 - 1750
		duration := base + jitter
		if duration < 3000 {
			duration = 3000
		}
		usedMS += duration
	}
	return solved, usedMS
}
