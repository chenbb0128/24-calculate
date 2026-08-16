package player

import (
	"context"
	"errors"
	"time"

	"github.com/example/go-service/internal/apperror"
)

func (s *Service) ResumeCampaignRun(ctx context.Context, userID uint64, runID string) (CampaignRunResponse, error) {
	if s.campaignRuns == nil {
		return CampaignRunResponse{}, apperror.ServiceUnavailable("campaign run service is unavailable", nil)
	}
	run, err := s.campaignRuns.GetCampaignRun(ctx, runID)
	if errors.Is(err, ErrCampaignRunNotFound) {
		return CampaignRunResponse{}, apperror.NotFound("campaign run was not found", err)
	}
	if err != nil {
		return CampaignRunResponse{}, err
	}
	if run.UserID != userID {
		return CampaignRunResponse{}, apperror.New(10004, 403, "you cannot access this run", nil)
	}
	if time.Now().UTC().After(run.ExpiresAt) && !runStatusTerminal(run.Status) {
		run.Status = RunExpired
		if stateStore, ok := s.campaignRuns.(CampaignRunStateStore); ok {
			_ = stateStore.UpdateCampaignRun(ctx, run)
		}
		return CampaignRunResponse{}, apperror.New(10005, 410, "campaign run has expired", nil)
	}
	return publicCampaignRun(run), nil
}

func (s *Service) ResumeDailyRun(ctx context.Context, userID uint64, runID string) (DailyRunResponse, error) {
	if s.dailyRuns == nil {
		return DailyRunResponse{}, apperror.ServiceUnavailable("daily run service is unavailable", nil)
	}
	run, err := s.dailyRuns.GetDailyRun(ctx, runID)
	if errors.Is(err, ErrDailyRunNotFound) {
		return DailyRunResponse{}, apperror.NotFound("daily run was not found", err)
	}
	if err != nil {
		return DailyRunResponse{}, err
	}
	if run.UserID != userID {
		return DailyRunResponse{}, apperror.New(10004, 403, "you cannot access this run", nil)
	}
	if time.Now().UTC().After(run.ExpiresAt) && !runStatusTerminal(run.Status) {
		run.Status = RunExpired
		if stateStore, ok := s.dailyRuns.(DailyRunStateStore); ok {
			_ = stateStore.UpdateDailyRun(ctx, run)
		}
		return DailyRunResponse{}, apperror.New(10005, 410, "daily run has expired", nil)
	}
	if run.DateKey != time.Now().In(shanghaiLocation).Format("2006-01-02") && !runStatusTerminal(run.Status) {
		return DailyRunResponse{}, apperror.New(10005, 410, "daily run is no longer today's challenge", nil)
	}
	return publicDailyRun(run), nil
}

func (s *Service) ResumeEndlessRun(ctx context.Context, userID uint64, runID string) (EndlessRunResponse, error) {
	if s.endlessRuns == nil {
		return EndlessRunResponse{}, apperror.ServiceUnavailable("endless run service is unavailable", nil)
	}
	run, err := s.endlessRuns.GetEndlessRun(ctx, runID)
	if errors.Is(err, ErrEndlessRunNotFound) {
		return EndlessRunResponse{}, apperror.NotFound("endless run was not found", err)
	}
	if err != nil {
		return EndlessRunResponse{}, err
	}
	if run.UserID != userID {
		return EndlessRunResponse{}, apperror.New(10004, 403, "you cannot access this run", nil)
	}
	if time.Now().UTC().After(run.ExpiresAt) && !runStatusTerminal(run.Status) {
		run.Status = RunExpired
		if stateStore, ok := s.endlessRuns.(EndlessRunStateStore); ok {
			_ = stateStore.UpdateEndlessRun(ctx, run)
		}
		return EndlessRunResponse{}, apperror.New(10005, 410, "endless run has expired", nil)
	}
	return publicEndlessRun(run), nil
}
