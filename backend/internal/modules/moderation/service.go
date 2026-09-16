package moderation

import (
	"context"
	"fmt"
	"time"
)

type Service struct {
	provider Provider
	audit    AuditStore
}

func NewService(provider Provider, audit AuditStore) *Service {
	return &Service{provider: provider, audit: audit}
}

func (s *Service) ModerateText(ctx context.Context, userID uint64, subject, source, content string) (Decision, error) {
	return s.moderateText(ctx, userID, subject, source, content, true)
}

func (s *Service) moderateText(ctx context.Context, userID uint64, subject, source, content string, recordAudit bool) (Decision, error) {
	normalized, err := NormalizeText(content)
	if err != nil {
		return Decision{Status: StatusRejected, ReasonCode: "invalid_text"}, nil
	}
	if s == nil || s.provider == nil {
		return Decision{Status: StatusUnavailable}, fmt.Errorf("moderation text provider is unavailable")
	}
	result, err := s.provider.CheckText(ctx, TextCheckRequest{
		UserID: userID, Subject: subject, Source: source, Content: normalized,
	})
	if err != nil {
		return Decision{Status: StatusUnavailable}, fmt.Errorf("moderation text provider failed")
	}
	decision := normalizeDecision(result)
	if recordAudit {
		if err := s.record(ctx, AuditEvent{
			UserID: userID, ResourceType: "nickname", Source: source,
			ModerationStatus: decision.Status, ReasonCode: decision.ReasonCode,
			ProviderRequestID: decision.ProviderRequestID,
		}); err != nil {
			return Decision{Status: StatusUnavailable}, fmt.Errorf("record moderation event: %w", err)
		}
	}
	return decision, nil
}

func (s *Service) ModerateImage(ctx context.Context, userID uint64, source string, content []byte) (Decision, error) {
	if len(content) == 0 {
		return Decision{Status: StatusRejected, ReasonCode: "empty_image"}, nil
	}
	if s == nil || s.provider == nil {
		return Decision{Status: StatusUnavailable}, fmt.Errorf("moderation image provider is unavailable")
	}
	result, err := s.provider.CheckImage(ctx, ImageCheckRequest{
		UserID: userID, Source: source, Content: append([]byte(nil), content...),
	})
	if err != nil {
		return Decision{Status: StatusUnavailable}, fmt.Errorf("moderation image provider failed")
	}
	decision := normalizeDecision(result)
	if err := s.record(ctx, AuditEvent{
		UserID: userID, ResourceType: "avatar", Source: source,
		ModerationStatus: decision.Status, ReasonCode: decision.ReasonCode,
		ProviderRequestID: decision.ProviderRequestID,
	}); err != nil {
		return Decision{Status: StatusUnavailable}, fmt.Errorf("record moderation event: %w", err)
	}
	return decision, nil
}

func (s *Service) record(ctx context.Context, event AuditEvent) error {
	if s == nil || s.audit == nil {
		return nil
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	return s.audit.RecordModerationEvent(ctx, event)
}

func normalizeDecision(result ProviderResult) Decision {
	status := result.Status
	switch status {
	case StatusApproved, StatusPending, StatusRejected:
	default:
		status = StatusUnavailable
	}
	return Decision{
		Status:            status,
		ReasonCode:        result.ReasonCode,
		ProviderRequestID: result.ProviderRequestID,
	}
}
