package player

import (
	"context"
	"time"
)

const (
	RunCreated   = "created"
	RunRunning   = "running"
	RunPaused    = "paused"
	RunSubmitted = "submitted"
	RunFinished  = "finished"
	RunFailed    = "failed"
	RunExpired   = "expired"
	RunCancelled = "cancelled"
)

func runStatusTerminal(status string) bool {
	switch status {
	case RunFinished, RunFailed, RunExpired, RunCancelled:
		return true
	default:
		return false
	}
}

// These optional interfaces let Redis persist the mutable Run state without
// breaking the small in-memory stores used by the existing unit tests.
type CampaignRunStateStore interface {
	UpdateCampaignRun(context.Context, CampaignRun) error
}

type DailyRunStateStore interface {
	UpdateDailyRun(context.Context, DailyRun) error
}

type EndlessRunStateStore interface {
	UpdateEndlessRun(context.Context, EndlessRun) error
}

// DistributedLockStore serializes server-side settlement and reward flows.
// It is optional so the existing in-memory unit-test stores remain small; the
// production Redis repository implements it.
type DistributedLockStore interface {
	AcquireDistributedLock(context.Context, string, time.Duration) (string, bool, error)
	ReleaseDistributedLock(context.Context, string, string) error
}
