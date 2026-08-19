package player

import (
	"context"
	"testing"
	"time"

	"github.com/example/go-service/internal/apperror"
)

type campaignResumeStoreFake struct{ run CampaignRun }

func (f *campaignResumeStoreFake) CreateCampaignRun(context.Context, CampaignRun) error { return nil }
func (f *campaignResumeStoreFake) GetCampaignRun(context.Context, string) (CampaignRun, error) {
	return f.run, nil
}
func (f *campaignResumeStoreFake) UpdateCampaignRun(_ context.Context, run CampaignRun) error {
	f.run = run
	return nil
}

func TestResumeCampaignRunRestoresServerStateAndAttempts(t *testing.T) {
	store := &campaignResumeStoreFake{run: CampaignRun{
		Version: 1, RunID: "cr_resume", UserID: 7, LevelID: 2, QuestionCount: 1,
		Questions: []CampaignPuzzle{{PuzzleID: "q1", Numbers: []int{1, 2, 3, 4}}},
		Attempts:  []CampaignRunAttemptInput{{PuzzleID: "q1", QuestionIndex: 0, Solved: true}},
		Status:    RunRunning, CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(time.Hour),
	}}
	service := NewService(nil, nil)
	service.campaignRuns = store
	result, err := service.ResumeCampaignRun(context.Background(), 7, "cr_resume")
	if err != nil {
		t.Fatalf("ResumeCampaignRun() error = %v", err)
	}
	if result.RunID != "cr_resume" || len(result.Questions) != 1 || len(result.Attempts) != 1 || result.Status != RunRunning {
		t.Fatalf("resume result = %+v", result)
	}
}

func TestResumeCampaignRunExpiredReturns410AndPersistsExpiredState(t *testing.T) {
	store := &campaignResumeStoreFake{run: CampaignRun{
		Version: 1, RunID: "cr_expired", UserID: 7, Status: RunRunning,
		ExpiresAt: time.Now().UTC().Add(-time.Minute),
	}}
	service := NewService(nil, nil)
	service.campaignRuns = store
	_, err := service.ResumeCampaignRun(context.Background(), 7, "cr_expired")
	appErr, ok := err.(*apperror.AppError)
	if !ok || appErr.HTTPStatus != 410 || store.run.Status != RunExpired {
		t.Fatalf("expired resume error = %v, state = %+v", err, store.run)
	}
	_, err = service.ResumeCampaignRun(context.Background(), 7, "cr_expired")
	appErr, ok = err.(*apperror.AppError)
	if !ok || appErr.HTTPStatus != 410 {
		t.Fatalf("repeated expired resume error = %v", err)
	}
}
