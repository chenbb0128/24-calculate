package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/example/go-service/internal/config"
)

func OpenMySQL(cfg config.DatabaseConfig) (*sql.DB, error) {
	dsnConfig := mysql.Config{
		User:                 cfg.User,
		Passwd:               cfg.Password,
		Net:                  "tcp",
		Addr:                 net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port)),
		DBName:               cfg.Name,
		AllowNativePasswords: true,
		ParseTime:            true,
		Loc:                  time.UTC,
		Timeout:              cfg.ConnectTimeout,
		ReadTimeout:          cfg.ConnectTimeout,
		WriteTimeout:         cfg.ConnectTimeout,
		Params: map[string]string{
			"charset": "utf8mb4",
		},
	}

	db, err := sql.Open("mysql", dsnConfig.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	return db, nil
}

func IsDuplicateEntry(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

type MySQLReadiness struct {
	DB      *sql.DB
	Timeout time.Duration
}

func (r MySQLReadiness) Check(ctx context.Context) error {
	if r.DB == nil {
		return fmt.Errorf("mysql database is nil")
	}
	checkCtx := ctx
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		checkCtx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}
	return r.DB.PingContext(checkCtx)
}
