package player

import (
	"context"
	"testing"
)

type roundScopedFriendRoomStore struct {
	*friendRoomStoreFake
	progressByRound    map[string]map[uint64]FriendMatchProgress
	submissionsByRound map[string]map[uint64]FriendMatchSubmissionRecord
}

func (f *roundScopedFriendRoomStore) SaveFriendMatchProgressRound(_ context.Context, _ string, roundID string, progress FriendMatchProgress) error {
	if f.progressByRound == nil {
		f.progressByRound = map[string]map[uint64]FriendMatchProgress{}
	}
	if f.progressByRound[roundID] == nil {
		f.progressByRound[roundID] = map[uint64]FriendMatchProgress{}
	}
	f.progressByRound[roundID][progress.UserID] = progress
	return nil
}

func (f *roundScopedFriendRoomStore) GetFriendMatchProgressRound(_ context.Context, _ string, roundID string) (map[uint64]FriendMatchProgress, error) {
	result := map[uint64]FriendMatchProgress{}
	for userID, progress := range f.progressByRound[roundID] {
		result[userID] = progress
	}
	return result, nil
}

func (f *roundScopedFriendRoomStore) SaveFriendMatchProgressEventRound(ctx context.Context, roomCode, roundID string, progress FriendMatchProgress) (bool, error) {
	if existing := f.progressByRound[roundID][progress.UserID]; existing.EventID != "" && existing.EventID == progress.EventID {
		return false, nil
	}
	return true, f.SaveFriendMatchProgressRound(ctx, roomCode, roundID, progress)
}

func (f *roundScopedFriendRoomStore) SaveFriendMatchSubmissionRound(_ context.Context, _ string, roundID string, submission FriendMatchSubmissionRecord) error {
	if f.submissionsByRound == nil {
		f.submissionsByRound = map[string]map[uint64]FriendMatchSubmissionRecord{}
	}
	if f.submissionsByRound[roundID] == nil {
		f.submissionsByRound[roundID] = map[uint64]FriendMatchSubmissionRecord{}
	}
	if _, exists := f.submissionsByRound[roundID][submission.UserID]; exists {
		return ErrFriendMatchSubmissionAlreadyExists
	}
	f.submissionsByRound[roundID][submission.UserID] = submission
	return nil
}

func (f *roundScopedFriendRoomStore) GetFriendMatchSubmissionsRound(_ context.Context, _ string, roundID string) (map[uint64]FriendMatchSubmissionRecord, error) {
	result := map[uint64]FriendMatchSubmissionRecord{}
	for userID, submission := range f.submissionsByRound[roundID] {
		result[userID] = submission
	}
	return result, nil
}

func TestFriendMatchReadsOnlyCurrentRoundState(t *testing.T) {
	room := newFriendLifecycleRoom()
	room.Status = FriendRoomRunning
	room.MatchID = "match-round-b"
	room.RoundID = "round-b"
	store := &roundScopedFriendRoomStore{
		friendRoomStoreFake: &friendRoomStoreFake{
			room: room,
			progress: map[uint64]FriendMatchProgress{
				3: {UserID: 3, RoundID: "round-a", Score: 999, Solved: 9},
			},
		},
		progressByRound: map[string]map[uint64]FriendMatchProgress{
			"round-a": {3: {UserID: 3, RoundID: "round-a", Score: 700, Solved: 7}},
			"round-b": {3: {UserID: 3, RoundID: "round-b", Score: 120, Solved: 1}},
		},
		submissionsByRound: map[string]map[uint64]FriendMatchSubmissionRecord{
			"round-a": {3: {UserID: 3, RoundID: "round-a", Score: 700}},
			"round-b": {3: {UserID: 3, RoundID: "round-b", Score: 120}},
		},
	}
	service := NewServiceWithRooms(leaderboardProfileReader{profile: testFriendProfile(3)}, &leaderboardStore{}, store)

	progress, err := service.GetFriendMatchProgress(context.Background(), 3, room.RoomCode)
	if err != nil {
		t.Fatalf("GetFriendMatchProgress() error = %v", err)
	}
	if progress.RoundID != "round-b" || len(progress.Players) != 2 || progress.Players[0].Score != 120 {
		t.Fatalf("current round progress = %#v, want round-b score 120", progress)
	}

	submissions, err := service.getFriendMatchSubmissions(context.Background(), room)
	if err != nil || submissions[3].RoundID != "round-b" || submissions[3].Score != 120 {
		t.Fatalf("current round submissions = %#v, error = %v", submissions, err)
	}
}
