package main

import (
	"context"
	"log/slog"
	"os"

	"idp-platform/backend/internal/config"
	"idp-platform/backend/internal/migrations"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	if err := migrations.Up(context.Background(), cfg); err != nil {
		slog.Error("database migrations failed", "error", err)
		os.Exit(1)
	}
	slog.Info("database migrations applied")
}
