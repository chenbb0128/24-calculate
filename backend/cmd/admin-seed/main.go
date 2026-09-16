package main

import (
	"context"
	"fmt"
	"os"

	"github.com/example/go-service/internal/config"
	"github.com/example/go-service/internal/modules/admin"
	"github.com/example/go-service/internal/store"
	db "github.com/example/go-service/internal/store/sqlc"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	username, ok := os.LookupEnv("GO_SERVICE_ADMIN_USERNAME")
	if !ok {
		return fmt.Errorf("GO_SERVICE_ADMIN_USERNAME is not set")
	}
	password, ok := os.LookupEnv("GO_SERVICE_ADMIN_PASSWORD")
	if !ok {
		return fmt.Errorf("GO_SERVICE_ADMIN_PASSWORD is not set")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	database, err := store.OpenMySQL(cfg.Database)
	if err != nil {
		return err
	}
	defer database.Close()

	adminRepository := admin.NewRepository(db.New(database))
	if err := admin.SeedAdmin(context.Background(), adminRepository, username, password); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "admin account seeded")
	return nil
}
