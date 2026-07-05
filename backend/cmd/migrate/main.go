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
	if err := migrations.Up(context.Background(), config.Load()); err != nil {
		slog.Error("database migrations failed", "error", err)
		os.Exit(1)
	}
	slog.Info("database migrations applied")
}
