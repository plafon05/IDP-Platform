package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"idp-platform/backend/internal/config"
	"idp-platform/backend/internal/database"
	"idp-platform/backend/internal/handler"
	"idp-platform/backend/internal/migrations"
	appserver "idp-platform/backend/internal/server"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
	slog.SetDefault(logger)

	if err := migrations.Up(context.Background(), cfg); err != nil {
		slog.Error("database migrations failed", "error", err)
		os.Exit(1)
	}

	dbPool, err := database.Connect(context.Background(), cfg)
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	router := handler.NewRouter(cfg, dbPool)
	server := appserver.NewHTTPServer(cfg, router)

	errCh := make(chan error, 1)
	go func() {
		slog.Info("starting api server", "addr", server.Addr)
		errCh <- server.ListenAndServe()
	}()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("api server failed", "error", err)
			os.Exit(1)
		}
	case sig := <-stopCh:
		slog.Info("shutdown requested", "signal", sig.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("api server stopped")
}
