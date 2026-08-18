package player

import (
	"context"
	"errors"
	"time"
)

// advanceFriendBot writes a bot's progress into the same Redis structures as
// a real player. It is deliberately driven by status polling, which keeps the
// implementation usable without a separate worker process while retaining
// one-question-at-a-time server timing.
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
	all, err := s.rooms.GetFriendMatchProgress(ctx, room.RoomCode)
	if err != nil {
		return err
	}
	previous := all[0]
	count := room.Rules.QuestionCount
	if count <= 0 {
		count = len(room.Puzzles)
	}
	if count <= 0 {
		count = 8
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
		UserID: 0, MatchID: room.MatchID, QuestionHash: room.QuestionHash,
		QuestionIndex: maxInt(0, solved-1), Solved: solved,
		Score: solved * 100, ElapsedMS: int(usedMS), Finished: finished, UpdatedAt: time.Now().UTC(),
	}
	if err := s.rooms.SaveFriendMatchProgress(ctx, room.RoomCode, progress); err != nil {
		return err
	}
	if finished {
		resultStore, ok := s.rooms.(FriendMatchResultStore)
		if ok {
			_ = resultStore.SaveFriendMatchSubmission(ctx, room.RoomCode, FriendMatchSubmissionRecord{
				UserID: 0, Solved: solved, Score: progress.Score, ElapsedMS: progress.ElapsedMS,
				CreatedAt: time.Now().UTC(),
			})
		}
		if lifecycle, ok := s.rooms.(FriendRoomLifecycleStore); ok {
			if err := lifecycle.FinishFriendRoom(ctx, room.RoomCode, room.MatchID); err != nil && !errors.Is(err, ErrFriendRoomStarted) {
				return err
			}
		}
	}
	return nil
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
