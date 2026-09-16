package moderation

import (
	"context"
	"database/sql"
	"fmt"

	db "github.com/example/go-service/internal/store/sqlc"
)

type UserStore interface {
	ListUsersForModeration(context.Context, uint64, int) ([]db.User, error)
	UpdateUserModeration(context.Context, db.UpdateUserModerationParams) error
}

type SQLAuditStore struct {
	queries *db.Queries
}

func NewSQLAuditStore(database *sql.DB) *SQLAuditStore {
	if database == nil {
		return &SQLAuditStore{}
	}
	return &SQLAuditStore{queries: db.New(database)}
}

func (s *SQLAuditStore) RecordModerationEvent(ctx context.Context, event AuditEvent) error {
	if s == nil || s.queries == nil {
		return fmt.Errorf("moderation audit repository is not initialized")
	}
	return s.queries.RecordModerationEvent(ctx, db.RecordModerationEventParams{
		UserID:            event.UserID,
		ResourceType:      event.ResourceType,
		ResourceID:        event.ResourceID,
		Source:            event.Source,
		ModerationStatus:  string(event.ModerationStatus),
		ReasonCode:        event.ReasonCode,
		ProviderRequestID: event.ProviderRequestID,
		CreatedAt:         event.CreatedAt,
	})
}

type SQLUserStore struct {
	queries *db.Queries
}

func NewSQLUserStore(database *sql.DB) *SQLUserStore {
	if database == nil {
		return &SQLUserStore{}
	}
	return &SQLUserStore{queries: db.New(database)}
}

func (s *SQLUserStore) ListUsersForModeration(ctx context.Context, afterID uint64, limit int) ([]db.User, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("moderation user repository is not initialized")
	}
	if limit < 1 || limit > 500 {
		return nil, fmt.Errorf("moderation user page size is invalid")
	}
	return s.queries.ListUsersForModeration(ctx, db.ListUsersForModerationParams{AfterID: afterID, Limit: int32(limit)})
}

func (s *SQLUserStore) UpdateUserModeration(ctx context.Context, arg db.UpdateUserModerationParams) error {
	if s == nil || s.queries == nil {
		return fmt.Errorf("moderation user repository is not initialized")
	}
	return s.queries.UpdateUserModeration(ctx, arg)
}
