package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/example/go-service/internal/config"
	"github.com/example/go-service/internal/modules/moderation"
	wechatplatform "github.com/example/go-service/internal/platform/wechat"
	"github.com/example/go-service/internal/store"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "report moderation decisions without changing user profiles")
	pageSize := flag.Int("page-size", 100, "number of users to scan per page")
	flag.Parse()

	if err := run(*dryRun, *pageSize); err != nil {
		slog.Error("moderation cleanup failed", "error", err)
		os.Exit(1)
	}
}

func run(dryRun bool, pageSize int) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	database, err := store.OpenMySQL(cfg.Database)
	if err != nil {
		return err
	}
	defer database.Close()

	client := wechatplatform.NewClient(cfg.WeChat)
	client.SetContentSafetyPolicy(cfg.Moderation.Timeout, cfg.Moderation.MaxRetries)
	auditStore := moderation.NewSQLAuditStore(database)
	moderator := moderation.NewService(client, auditStore)
	userStore := moderation.NewSQLUserStore(database)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	report, err := moderator.Cleanup(ctx, userStore, dryRun, pageSize)
	if err != nil {
		return err
	}
	mode := "apply"
	if dryRun {
		mode = "dry-run"
	}
	fmt.Printf("moderation cleanup %s: scanned=%d updated=%d approved=%d rejected=%d unreviewed=%d failed=%d avatars_hidden=%d cache_changes=%d\n", mode, report.Scanned, report.Updated, report.Approved, report.Rejected, report.Unreviewed, report.Failed, report.AvatarsHidden, report.CacheChanges)
	return nil
}
