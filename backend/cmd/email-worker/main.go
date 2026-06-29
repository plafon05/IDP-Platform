package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"idp-platform/backend/internal/config"
	"idp-platform/backend/internal/database"
	"idp-platform/backend/internal/notification"
)

func main() {
	cfg := config.Load()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	queue, err := notification.NewQueue(cfg.RedisURL, cfg.EmailQueueKey)
	if err != nil {
		slog.Error("email queue initialization failed", "error", err)
		os.Exit(1)
	}
	defer queue.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := queue.Ping(pingCtx); err != nil {
		slog.Error("email queue unavailable", "error", err)
		os.Exit(1)
	}
	db, err := database.Connect(ctx, cfg)
	if err != nil {
		slog.Error("notification database unavailable", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	outbox := notification.NewOutbox(db)
	go notification.NewRelay(db, queue, cfg.JWTSecret, cfg.FrontendURL).Run(ctx)
	reminders, err := notification.NewReminderScheduler(db, outbox, cfg.FrontendURL, cfg.ReminderTimezone)
	if err != nil {
		slog.Error("deadline reminder scheduler initialization failed", "error", err)
		os.Exit(1)
	}
	go reminders.Run(ctx)
	slog.Info("email worker started")
	if err := notification.NewWorker(queue, notification.NewSMTPSender(cfg)).Run(ctx); err != nil {
		slog.Error("email worker failed", "error", err)
		os.Exit(1)
	}
	slog.Info("email worker stopped")
}
