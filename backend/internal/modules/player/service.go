package player

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/example/go-service/internal/apperror"
	"github.com/example/go-service/internal/modules/user"
	"github.com/example/go-service/internal/store"
	db "github.com/example/go-service/internal/store/sqlc"
)

type ProfileReader interface {
	GetProfile(ctx context.Context, id uint64) (user.ProfileResponse, error)
}

type ProgressMutation func(map[string]any) error

type Store interface {
	GetPlayerProfile(ctx context.Context, userID uint64) (db.PlayerProfile, error)
	CreatePlayerProfile(ctx context.Context, arg db.CreatePlayerProfileParams) error
	MutatePlayerProgress(ctx context.Context, userID uint64, mutate ProgressMutation) (json.RawMessage, error)
	GetLeaderboardSubmissionByKey(ctx context.Context, arg db.GetLeaderboardSubmissionByKeyParams) (db.PlayerLeaderboardSubmission, error)
	CreateLeaderboardSubmission(ctx context.Context, arg db.CreateLeaderboardSubmissionParams) error
	ListEndlessLeaderboard(ctx context.Context) ([]db.LeaderboardScoreRow, error)
	ListFriendLeaderboard(ctx context.Context) ([]db.LeaderboardScoreRow, error)
	ListFriendUserIDs(ctx context.Context, userID uint64) ([]uint64, error)
	ListCampaignLeaderboard(ctx context.Context) ([]db.CampaignLeaderboardRow, error)
	ListDailyLeaderboard(ctx context.Context, dateKey string) ([]db.DailyLeaderboardRow, error)
	CompleteLevel(ctx context.Context, params CompleteLevelParams) (CompleteLevelResult, error)
	CompleteDaily(ctx context.Context, params CompleteDailyParams) (CompleteDailyResult, error)
}

type Service struct {
	profiles        ProfileReader
	store           Store
	rooms           FriendRoomStore
	endlessRuns     EndlessRunStore
	campaignRuns    CampaignRunStore
	dailyRuns       DailyRunStore
	matchmaking     MatchmakingStore
	locks           DistributedLockStore
	dailySeedSecret string
	matchmakingWait time.Duration
}

const defaultMatchmakingWait = 15 * time.Second

func NewService(profiles ProfileReader, store Store) *Service {
	return &Service{profiles: profiles, store: store, matchmakingWait: defaultMatchmakingWait}
}

func NewServiceWithRoomsAndEndless(profiles ProfileReader, store Store, rooms FriendRoomStore, endlessRuns EndlessRunStore) *Service {
	service := &Service{
		profiles: profiles, store: store, rooms: rooms, endlessRuns: endlessRuns,
		campaignRuns: campaignRunStoreFrom(endlessRuns), dailyRuns: dailyRunStoreFrom(endlessRuns), matchmaking: matchmakingStoreFrom(endlessRuns),
		matchmakingWait: defaultMatchmakingWait,
	}
	if locks, ok := rooms.(DistributedLockStore); ok {
		service.locks = locks
	} else if locks, ok := endlessRuns.(DistributedLockStore); ok {
		service.locks = locks
	}
	return service
}

func (s *Service) acquireSettlementLock(ctx context.Context, scope string) (func(), error) {
	if s == nil || s.locks == nil {
		return func() {}, nil
	}
	token, acquired, err := s.locks.AcquireDistributedLock(ctx, scope, 30*time.Second)
	if err != nil {
		return nil, err
	}
	if !acquired {
		return nil, apperror.New(10003, 409, "该结算请求正在处理中，请稍后重试", nil)
	}
	return func() {
		_ = s.locks.ReleaseDistributedLock(context.Background(), scope, token)
	}, nil
}

func (s *Service) SetDailySeedSecret(secret string) {
	if s != nil {
		s.dailySeedSecret = strings.TrimSpace(secret)
	}
}

func (s *Service) SetMatchmakingWait(wait time.Duration) {
	if s != nil && wait > 0 {
		s.matchmakingWait = wait
	}
}

func (s *Service) Bootstrap(ctx context.Context, userID uint64) (BootstrapResponse, error) {
	profile, err := s.profiles.GetProfile(ctx, userID)
	if err != nil {
		return BootstrapResponse{}, err
	}

	_, err = s.store.GetPlayerProfile(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		now := time.Now().UTC()
		createErr := s.store.CreatePlayerProfile(ctx, db.CreatePlayerProfileParams{
			UserID:       userID,
			ProgressJSON: DefaultProgressJSON,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
		if createErr != nil {
			// Two first requests for the same new user may race. A duplicate row
			// means another request already initialized the profile, so read it.
			if !store.IsDuplicateEntry(createErr) {
				return BootstrapResponse{}, createErr
			}
			_, err = s.store.GetPlayerProfile(ctx, userID)
			if err != nil {
				return BootstrapResponse{}, err
			}
		}
	} else if err != nil {
		return BootstrapResponse{}, err
	}

	dateKey := time.Now().In(shanghaiLocation).Format("2006-01-02")
	loginReward := 0
	progressRaw, err := s.store.MutatePlayerProgress(ctx, userID, func(state map[string]any) error {
		login := ensureObject(state, "login")
		if strings.TrimSpace(fmt.Sprint(login["last_date"])) == dateKey {
			return nil
		}
		streak := 1
		if strings.TrimSpace(fmt.Sprint(login["last_date"])) == previousShanghaiDate(dateKey) {
			streak = minInt(7, maxInt(1, readInt(login["streak"])+1))
		}
		loginReward = 5 + maxInt(0, streak-1)*2
		if streak == 7 {
			loginReward += 10
		}
		login["last_date"] = dateKey
		login["streak"] = streak
		login["last_reward"] = loginReward
		state["login"] = login
		state["coins"] = minInt(999999, maxInt(0, readInt(state["coins"]))+loginReward)
		return nil
	})
	if err != nil {
		return BootstrapResponse{}, err
	}
	progress := strings.TrimSpace(string(progressRaw))
	if !json.Valid([]byte(progress)) || progress == "" {
		progress = DefaultProgressJSON
	}
	return BootstrapResponse{
		User:        profile,
		Progress:    json.RawMessage(progress),
		LoginReward: loginReward,
		ServerDate:  dateKey,
	}, nil
}

func (s *Service) CompleteLevel(ctx context.Context, userID uint64, levelID int, input CompleteLevelInput) (CompleteLevelResponse, error) {
	return s.completeLevel(ctx, userID, levelID, input, CompletionMetrics{})
}

func (s *Service) completeLevel(ctx context.Context, userID uint64, levelID int, input CompleteLevelInput, metrics CompletionMetrics) (CompleteLevelResponse, error) {
	if levelID < 0 || levelID >= 200 {
		return CompleteLevelResponse{}, apperror.BadRequest("level_id is out of range", nil)
	}
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if len(idempotencyKey) < 8 || len(idempotencyKey) > 128 {
		return CompleteLevelResponse{}, apperror.BadRequest("idempotency_key length is invalid", nil)
	}
	if input.Score < 0 || input.Score > 100 {
		return CompleteLevelResponse{}, apperror.BadRequest("score is out of range", nil)
	}
	if input.Stars < 1 || input.Stars > 3 {
		return CompleteLevelResponse{}, apperror.BadRequest("stars is out of range", nil)
	}
	if _, err := s.profiles.GetProfile(ctx, userID); err != nil {
		return CompleteLevelResponse{}, err
	}
	result, err := s.store.CompleteLevel(ctx, CompleteLevelParams{
		UserID:         userID,
		LevelID:        levelID,
		IdempotencyKey: idempotencyKey,
		Score:          input.Score,
		Stars:          input.Stars,
		Questions:      metrics.Questions,
		ElapsedMS:      metrics.ElapsedMS,
		FastestMS:      metrics.FastestMS,
		Mistakes:       metrics.Mistakes,
		Hints:          metrics.Hints,
		BestCombo:      metrics.BestCombo,
		Operators:      append([]string(nil), metrics.Operators...),
	})
	if err != nil {
		return CompleteLevelResponse{}, err
	}
	return CompleteLevelResponse{
		LevelID:             result.LevelID,
		Stars:               result.Stars,
		BestScore:           result.BestScore,
		RewardCoins:         result.RewardCoins,
		Coins:               result.Coins,
		UnlockedLevel:       result.UnlockedLevel,
		Progress:            result.Progress,
		IdempotencyReplayed: result.IdempotencyReplayed,
	}, nil
}

func (s *Service) CompleteDaily(ctx context.Context, userID uint64, input CompleteDailyInput) (CompleteDailyResponse, error) {
	return s.completeDaily(ctx, userID, input, CompletionMetrics{})
}

func (s *Service) completeDaily(ctx context.Context, userID uint64, input CompleteDailyInput, metrics CompletionMetrics) (CompleteDailyResponse, error) {
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if len(idempotencyKey) < 8 || len(idempotencyKey) > 128 {
		return CompleteDailyResponse{}, apperror.BadRequest("idempotency_key length is invalid", nil)
	}
	if input.Score < 0 || input.Score > 10000000 {
		return CompleteDailyResponse{}, apperror.BadRequest("score is out of range", nil)
	}
	if _, err := s.profiles.GetProfile(ctx, userID); err != nil {
		return CompleteDailyResponse{}, err
	}
	result, err := s.store.CompleteDaily(ctx, CompleteDailyParams{
		UserID:         userID,
		IdempotencyKey: idempotencyKey,
		Score:          input.Score,
		Questions:      metrics.Questions,
		ElapsedMS:      metrics.ElapsedMS,
		FastestMS:      metrics.FastestMS,
		Mistakes:       metrics.Mistakes,
		Hints:          metrics.Hints,
		BestCombo:      metrics.BestCombo,
		Operators:      append([]string(nil), metrics.Operators...),
	})
	if err != nil {
		return CompleteDailyResponse{}, err
	}
	return CompleteDailyResponse{
		DateKey:             result.DateKey,
		Score:               result.Score,
		BestScore:           result.BestScore,
		Streak:              result.Streak,
		RewardCoins:         result.RewardCoins,
		Coins:               result.Coins,
		Progress:            result.Progress,
		IdempotencyReplayed: result.IdempotencyReplayed,
	}, nil
}

const DefaultProgressJSON = `{"version":12,"unlocked_level":0,"last_level":0,"levels":{},"level_rewards":{},"coins":0,"owned_skins":["classic"],"equipped_skin":"classic","owned_cosmetics":["card_classic","operator_classic","result_classic"],"equipped_cosmetics":{"card":"card_classic","operator":"operator_classic","result":"result_classic"},"login":{"last_date":"","streak":0,"last_reward":0},"daily":{"last_date":"","streak":0,"best_score":0,"completed":{},"reward_claimed":{}},"endless":{"best_score":0,"best_questions":0,"best_combo":0,"best_stage":0,"last_score":0,"reward_date":"","reward_coins_today":0,"reward_run_id":"","rewarded_questions":{}},"friend_matches":{"date":"","played":0,"wins":0,"best_score":0,"best_time_ms":0,"reward_date":"","reward_count":0},"tasks":{"date":"","values":{},"claimed":{}},"weekly_tasks":{"week":"","values":{},"claimed":{}},"player_stats":{"total_solved":0,"total_score":0,"fastest_ms":0,"best_combo":0,"best_level":0,"best_chapter":0,"operator_counts":{},"mode_questions":{},"last_solve":{}},"audio":{"music_enabled":true,"sfx_enabled":true,"music_track":0,"music_volume":0.42,"sfx_volume":0.72},"server_events":{"endless":{},"friend":{}},"achievements":{"unlocked":{},"claimed":{}}}`

var shanghaiLocation = time.FixedZone("Asia/Shanghai", 8*60*60)
