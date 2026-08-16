package store

import (
	"context"
	"database/sql"
	"fmt"

	db "github.com/example/go-service/internal/store/sqlc"
)

type TxManager struct {
	db *sql.DB
}

func NewTxManager(database *sql.DB) *TxManager {
	return &TxManager{db: database}
}

func (m *TxManager) Exec(ctx context.Context, fn func(*db.Queries) error) error {
	if m == nil || m.db == nil {
		return fmt.Errorf("transaction manager database is nil")
	}
	if fn == nil {
		return fmt.Errorf("transaction callback is nil")
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	queries := db.New(m.db).WithTx(tx)
	if err := fn(queries); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
