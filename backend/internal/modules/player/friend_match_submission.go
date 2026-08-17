package player

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/example/go-service/internal/apperror"
)

const friendMatchProtocolVersion = 2

const friendMatchElapsedGraceMS = 5000

type friendMatchCalculation struct {
	Solved    int
	Score     int
	Mistakes  int
	ElapsedMS int
}

type FriendMatchAttemptInput struct {
	ProtocolVersion int                       `json:"protocol_version"`
	PuzzleID        string                    `json:"puzzle_id"`
	QuestionIndex   int                       `json:"question_index"`
	ElapsedMS       int                       `json:"elapsed_ms"`
	Solved          bool                      `json:"solved"`
	Mistakes        int                       `json:"mistakes"`
	Score           int                       `json:"score"`
	ScoreDelta      int                       `json:"score_delta"`
	RoomSeed        int64                     `json:"room_seed"`
	QuestionHash    string                    `json:"question_hash"`
	EventID         string                    `json:"event_id"`
	SolutionSteps   []FriendMatchSolutionStep `json:"solution_steps"`
}

type FriendMatchSummaryInput struct {
	PlayerSolved   int     `json:"player_solved"`
	PlayerScore    int     `json:"player_score"`
	PlayerMistakes int     `json:"player_mistakes"`
	PlayerElapsed  float64 `json:"player_elapsed"`
	Outcome        string  `json:"outcome"`
}

type FriendMatchSubmissionInput struct {
	ProtocolVersion     int                       `json:"protocol_version"`
	Action              string                    `json:"action"`
	IdempotencyKey      string                    `json:"idempotency_key"`
	MatchID             string                    `json:"match_id"`
	RoomID              string                    `json:"room_id"`
	RoomSeed            int64                     `json:"room_seed"`
	QuestionCount       int                       `json:"question_count"`
	QuestionHash        string                    `json:"question_hash"`
	PuzzleIDs           []string                  `json:"puzzle_ids"`
	Attempts            []FriendMatchAttemptInput `json:"attempts"`
	Summary             FriendMatchSummaryInput   `json:"summary"`
	ClientAuthoritative bool                      `json:"client_authoritative"`
}

type FriendMatchSubmissionResponse struct {
	Mode                string             `json:"mode"`
	MatchID             string             `json:"match_id"`
	Score               int                `json:"score"`
	Questions           int                `json:"questions"`
	ElapsedMS           int                `json:"elapsed_ms"`
	Validated           bool               `json:"validated"`
	IdempotencyReplayed bool               `json:"idempotency_replayed"`
	Outcome             string             `json:"outcome,omitempty"`
	Pending             bool               `json:"pending,omitempty"`
	RankResult          *RankResult        `json:"rank_result,omitempty"`
	RewardCoins         int                `json:"reward_coins,omitempty"`
	Coins               int                `json:"coins,omitempty"`
	Progress            json.RawMessage    `json:"progress,omitempty"`
	MatchResult         *FriendMatchResult `json:"match_result,omitempty"`
}

func (s *Service) SubmitFriendMatch(ctx context.Context, userID uint64, roomCode string, input FriendMatchSubmissionInput) (FriendMatchSubmissionResponse, error) {
	if err := s.allowFriendRoomAction(ctx, userID, "submit", 8, time.Minute); err != nil {
		return FriendMatchSubmissionResponse{}, err
	}
	release, lockErr := s.acquireSettlementLock(ctx, "friend:"+strings.TrimSpace(roomCode)+":"+strconv.FormatUint(userID, 10))
	if lockErr != nil {
		return FriendMatchSubmissionResponse{}, lockErr
	}
	defer release()
	room, err := s.GetFriendRoom(ctx, roomCode)
	if err != nil {
		return FriendMatchSubmissionResponse{}, err
	}
	if !friendRoomHasPlayer(room, userID) {
		return FriendMatchSubmissionResponse{}, apperror.New(10004, 403, "当前用户不属于该好友房间", nil)
	}
	if room.MatchID == "" || (room.Status != FriendRoomRunning && room.Status != FriendRoomFinished) {
		return FriendMatchSubmissionResponse{}, mapFriendRoomError(errForFriendRoomStatus(room.Status))
	}
	// A finished room rejects further live progress updates, but the other
	// player still needs to submit their already-recorded final attempt. That
	// submission is read-only with respect to room progress and is required to
	// produce the server-side result when one player finishes first.
	calculated, err := validateFriendMatchSubmission(room, input)
	if err != nil {
		return FriendMatchSubmissionResponse{}, err
	}

	// The outcome is always calculated from both server-validated records. The
	// value supplied by the client is accepted only as a protocol field and is
	// never written to the leaderboard.
	validatedOutcome := ""
	matchResult := (*FriendMatchResult)(nil)
	rankSettlement := (*RankSettlementResult)(nil)
	idempotencyReplayed := false
	currentRecord := FriendMatchSubmissionRecord{
		UserID: userID, Solved: calculated.Solved, Score: calculated.Score,
		Mistakes: calculated.Mistakes, ElapsedMS: calculated.ElapsedMS,
		IdempotencyKey: input.IdempotencyKey, CreatedAt: time.Now().UTC(),
	}
	if resultStore, ok := s.rooms.(FriendMatchResultStore); ok {
		submissions, err := resultStore.GetFriendMatchSubmissions(ctx, room.RoomCode)
		if err != nil {
			return FriendMatchSubmissionResponse{}, err
		}
		if existing, exists := submissions[userID]; exists {
			if existing.IdempotencyKey != input.IdempotencyKey {
				return FriendMatchSubmissionResponse{}, apperror.New(10003, 409, "该玩家已经提交过本场对局", ErrFriendMatchSubmissionAlreadyExists)
			}
			currentRecord = existing
			idempotencyReplayed = true
		} else if err := resultStore.SaveFriendMatchSubmission(ctx, room.RoomCode, currentRecord); err != nil {
			if !errors.Is(err, ErrFriendMatchSubmissionAlreadyExists) {
				return FriendMatchSubmissionResponse{}, err
			}
			idempotencyReplayed = true
		}
		submissions, err = resultStore.GetFriendMatchSubmissions(ctx, room.RoomCode)
		if err != nil {
			return FriendMatchSubmissionResponse{}, err
		}
		if stored, exists := submissions[userID]; exists {
			currentRecord = stored
		}
		if len(submissions) >= 2 {
			var opponent FriendMatchSubmissionRecord
			foundOpponent := false
			for _, candidate := range submissions {
				if candidate.UserID != userID {
					opponent = candidate
					foundOpponent = true
					break
				}
			}
			if foundOpponent {
				outcome := compareFriendResults(submissions[userID], opponent)
				matchResult = &FriendMatchResult{
					Outcome: outcome, PlayerSolved: currentRecord.Solved, PlayerScore: currentRecord.Score,
					PlayerMistakes: currentRecord.Mistakes, PlayerElapsedMS: currentRecord.ElapsedMS,
					PlayerElapsed: float64(currentRecord.ElapsedMS) / 1000, OpponentSolved: opponent.Solved, OpponentScore: opponent.Score,
					OpponentMistakes: opponent.Mistakes, OpponentElapsedMS: opponent.ElapsedMS, OpponentElapsed: float64(opponent.ElapsedMS) / 1000,
				}
				validatedOutcome = outcome
				rankSettlement, err = s.settleRankedFriendMatch(ctx, userID, room, submissions)
				if err != nil {
					return FriendMatchSubmissionResponse{}, err
				}
				if rankSettlement != nil {
					matchResult.RankResult = &rankSettlement.Result
				}
			}
		} else {
			validatedOutcome = ""
		}
		if matchResult != nil {
			if lifecycle, ok := s.rooms.(FriendRoomLifecycleStore); ok {
				if err := lifecycle.FinishFriendRoom(ctx, room.RoomCode, room.MatchID); err != nil && !errors.Is(err, ErrFriendRoomStarted) {
					return FriendMatchSubmissionResponse{}, err
				}
			}
		}
	}
	leaderboard, err := s.submitLeaderboardRecord(ctx, userID, LeaderboardFriend, SubmitLeaderboardInput{
		IdempotencyKey: input.IdempotencyKey,
		Score:          currentRecord.Score,
		Questions:      currentRecord.Solved,
		ElapsedMS:      currentRecord.ElapsedMS,
		RoomID:         room.RoomID,
		Outcome:        validatedOutcome,
		Metadata: map[string]any{
			"protocol_version":  input.ProtocolVersion,
			"match_id":          input.MatchID,
			"question_hash":     input.QuestionHash,
			"attempt_count":     len(input.Attempts),
			"player_mistakes":   input.Summary.PlayerMistakes,
			"validated_outcome": validatedOutcome,
		},
	})
	if err != nil {
		return FriendMatchSubmissionResponse{}, err
	}
	response := FriendMatchSubmissionResponse{
		Mode:                leaderboard.Mode,
		MatchID:             room.MatchID,
		Score:               leaderboard.Score,
		Questions:           leaderboard.Questions,
		ElapsedMS:           leaderboard.ElapsedMS,
		Validated:           true,
		IdempotencyReplayed: leaderboard.IdempotencyReplayed || idempotencyReplayed,
		Outcome:             validatedOutcome, Pending: validatedOutcome == "", MatchResult: matchResult,
	}
	if rankSettlement != nil {
		response.RankResult = &rankSettlement.Result
	}
	if matchResult != nil {
		var reward int
		progress, mutateErr := s.store.MutatePlayerProgress(ctx, userID, func(state map[string]any) error {
			var err error
			reward, err = applyFriendServerResult(state, *matchResult, time.Now().In(shanghaiLocation).Format("2006-01-02"), room.RoomID)
			return err
		})
		if mutateErr != nil {
			return FriendMatchSubmissionResponse{}, mutateErr
		}
		if rankSettlement != nil {
			var progressWithRankValue string
			progressWithRankValue, mutateErr = progressWithRank(string(progress), rankView(rankSettlement.Profile))
			if mutateErr != nil {
				return FriendMatchSubmissionResponse{}, mutateErr
			}
			progress = json.RawMessage(progressWithRankValue)
		}
		response.RewardCoins = reward
		response.Coins = progressCoins(string(progress))
		response.Progress = progress
	}
	return response, nil
}

func validateFriendMatchSubmission(room FriendRoom, input FriendMatchSubmissionInput) (friendMatchCalculation, error) {
	questionCount := room.Rules.QuestionCount
	if questionCount <= 0 {
		questionCount = 8
	}
	timeLimit := room.Rules.TimeLimitSeconds
	if timeLimit <= 0 {
		timeLimit = 120
	}
	if input.ProtocolVersion != friendMatchProtocolVersion || input.Action != "submitFriendMatch" || input.ClientAuthoritative {
		return friendMatchCalculation{}, apperror.BadRequest("friend match protocol is invalid", nil)
	}
	if strings.TrimSpace(input.IdempotencyKey) == "" {
		return friendMatchCalculation{}, apperror.BadRequest("idempotency_key is required", nil)
	}
	expectedMatchID := room.MatchID
	if expectedMatchID == "" {
		expectedMatchID = room.RoomID
	}
	if (input.MatchID != expectedMatchID && input.MatchID != room.RoomID) || input.RoomID != room.RoomID || input.RoomSeed != room.RoomSeed {
		return friendMatchCalculation{}, apperror.BadRequest("friend match room contract is invalid", nil)
	}
	maxAttempts := questionCount + maxInt(1, timeLimit/5) + 1
	if input.QuestionCount != questionCount || len(input.PuzzleIDs) != questionCount || len(input.Attempts) == 0 || len(input.Attempts) > maxAttempts {
		return friendMatchCalculation{}, apperror.BadRequest("friend match question count is invalid", nil)
	}
	expectedHash, expectedPuzzleIDs, expectedPuzzles := friendRoomContract(room)
	if input.QuestionHash != expectedHash || len(input.QuestionHash) != 8 {
		return friendMatchCalculation{}, apperror.BadRequest("friend match question hash is invalid", nil)
	}
	if _, err := hex.DecodeString(input.QuestionHash); err != nil {
		return friendMatchCalculation{}, apperror.BadRequest("friend match question hash is invalid", err)
	}

	seenPuzzleIDs := make(map[string]struct{}, len(input.PuzzleIDs))
	for index, puzzleID := range input.PuzzleIDs {
		puzzleID = strings.TrimSpace(puzzleID)
		if puzzleID == "" {
			return friendMatchCalculation{}, apperror.BadRequest("friend match puzzle id is invalid", nil)
		}
		if puzzleID != expectedPuzzleIDs[index] {
			return friendMatchCalculation{}, apperror.BadRequest("friend match puzzle contract is invalid", nil)
		}
		if _, exists := seenPuzzleIDs[puzzleID]; exists {
			return friendMatchCalculation{}, apperror.BadRequest("friend match puzzle ids are duplicated", nil)
		}
		seenPuzzleIDs[puzzleID] = struct{}{}
	}

	previousScore := 0
	previousElapsed := 0
	previousMistakes := 0
	combo := 0
	nextQuestion := 0
	solved := 0
	lastMistakes := 0
	seenEvents := make(map[string]struct{}, len(input.Attempts))
	for _, attempt := range input.Attempts {
		if attempt.QuestionIndex < 0 || attempt.QuestionIndex >= questionCount || attempt.QuestionIndex != nextQuestion ||
			attempt.PuzzleID != expectedPuzzleIDs[attempt.QuestionIndex] {
			return friendMatchCalculation{}, apperror.BadRequest("friend match attempt sequence is invalid", nil)
		}
		if attempt.ProtocolVersion != friendMatchProtocolVersion ||
			attempt.RoomSeed != input.RoomSeed ||
			attempt.QuestionHash != input.QuestionHash {
			return friendMatchCalculation{}, apperror.BadRequest("friend match attempt sequence is invalid", nil)
		}
		if attempt.ElapsedMS < previousElapsed || attempt.ElapsedMS > timeLimit*1000 || attempt.ElapsedMS < 0 {
			return friendMatchCalculation{}, apperror.BadRequest("friend match attempt time is invalid", nil)
		}
		if attempt.Mistakes < previousMistakes || attempt.Mistakes > 10000 || attempt.Score < 0 || attempt.ScoreDelta < 0 || attempt.Score < previousScore {
			return friendMatchCalculation{}, apperror.BadRequest("friend match attempt score is invalid", nil)
		}
		if attempt.Mistakes > 10000 || len(attempt.SolutionSteps) > 3 {
			return friendMatchCalculation{}, apperror.BadRequest("friend match attempt details are invalid", nil)
		}
		if attempt.EventID == "" {
			return friendMatchCalculation{}, apperror.BadRequest("friend match attempt event_id is required", nil)
		}
		if len(attempt.EventID) > 256 {
			return friendMatchCalculation{}, apperror.BadRequest("friend match attempt event_id is invalid", nil)
		}
		if _, exists := seenEvents[attempt.EventID]; exists {
			return friendMatchCalculation{}, apperror.BadRequest("friend match attempt event_id is duplicated", nil)
		}
		seenEvents[attempt.EventID] = struct{}{}
		if attempt.ScoreDelta != attempt.Score-previousScore {
			return friendMatchCalculation{}, apperror.BadRequest("friend match score delta is invalid", nil)
		}
		if attempt.Solved {
			if !replayFriendSolution(expectedPuzzles[attempt.QuestionIndex].Numbers, attempt.SolutionSteps, expectedPuzzles[attempt.QuestionIndex].Rules) {
				return friendMatchCalculation{}, apperror.BadRequest("friend match solution is invalid", nil)
			}
			expectedDelta := friendMatchScoreDelta(timeLimit, attempt.ElapsedMS, combo, attempt.Mistakes)
			if attempt.ScoreDelta != expectedDelta || attempt.ScoreDelta > 1000 {
				return friendMatchCalculation{}, apperror.BadRequest("friend match score calculation is invalid", nil)
			}
			combo++
			nextQuestion++
			solved++
		} else {
			if attempt.ScoreDelta != 0 || len(attempt.SolutionSteps) != 0 {
				return friendMatchCalculation{}, apperror.BadRequest("friend match unsolved attempt is invalid", nil)
			}
			combo = 0
		}
		previousScore = attempt.Score
		previousElapsed = attempt.ElapsedMS
		previousMistakes = attempt.Mistakes
		lastMistakes = attempt.Mistakes
	}

	if input.Summary.PlayerSolved != solved ||
		input.Summary.PlayerScore != previousScore ||
		input.Summary.PlayerMistakes != lastMistakes ||
		int(math.Round(input.Summary.PlayerElapsed*1000)) != previousElapsed ||
		input.Summary.PlayerSolved < 0 ||
		input.Summary.PlayerSolved > questionCount ||
		input.Summary.PlayerScore < 0 ||
		input.Summary.PlayerElapsed < 0 ||
		input.Summary.PlayerElapsed > float64(timeLimit) {
		return friendMatchCalculation{}, apperror.BadRequest("friend match summary does not match attempts", nil)
	}
	if input.Summary.Outcome != "" && input.Summary.Outcome != "pending" && input.Summary.Outcome != "win" && input.Summary.Outcome != "lose" && input.Summary.Outcome != "draw" {
		return friendMatchCalculation{}, apperror.BadRequest("friend match outcome is invalid", nil)
	}
	if room.StartAt > 0 {
		serverElapsed := time.Now().UTC().UnixMilli() - room.StartAt
		if serverElapsed > 0 && int64(previousElapsed)+friendMatchElapsedGraceMS < minInt64(serverElapsed, int64(timeLimit*1000)) {
			return friendMatchCalculation{}, apperror.BadRequest("friend match elapsed time is inconsistent", nil)
		}
	}
	return friendMatchCalculation{Solved: solved, Score: previousScore, Mistakes: lastMistakes, ElapsedMS: previousElapsed}, nil
}

func friendMatchScoreDelta(timeLimitSeconds, elapsedMS, comboBefore, mistakes int) int {
	remainingSeconds := float64(timeLimitSeconds*1000-elapsedMS) / 1000
	delta := int(math.Round(remainingSeconds*6 + float64(comboBefore*30) - float64(mistakes*5)))
	if delta < 10 {
		return 10
	}
	return delta
}

func minInt64(left, right int64) int64 {
	if left < right {
		return left
	}
	return right
}
