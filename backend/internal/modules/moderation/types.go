package moderation

import (
	"context"
	"time"
)

type Status string

const (
	StatusApproved    Status = "approved"
	StatusPending     Status = "pending"
	StatusRejected    Status = "rejected"
	StatusUnreviewed  Status = "unreviewed"
	StatusUnavailable Status = "unavailable"
)

type TextCheckRequest struct {
	UserID  uint64
	Subject string
	Source  string
	Content string
}

type ImageCheckRequest struct {
	UserID  uint64
	Source  string
	Content []byte
}

type ProviderResult struct {
	Status            Status
	ReasonCode        string
	ProviderRequestID string
}

type Provider interface {
	CheckText(context.Context, TextCheckRequest) (ProviderResult, error)
	CheckImage(context.Context, ImageCheckRequest) (ProviderResult, error)
}

type AuditEvent struct {
	UserID            uint64
	ResourceType      string
	ResourceID        string
	Source            string
	ModerationStatus  Status
	ReasonCode        string
	ProviderRequestID string
	CreatedAt         time.Time
}

type AuditStore interface {
	RecordModerationEvent(context.Context, AuditEvent) error
}

type Decision struct {
	Status            Status
	ReasonCode        string
	ProviderRequestID string
}

func IsPubliclyApproved(status Status) bool {
	return status == StatusApproved
}
