package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
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
	if err := cfg.Validate(); err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
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
	var background sync.WaitGroup
	background.Add(2)
	go func() {
		defer background.Done()
		notification.NewRelay(db, queue, cfg.JWTSecret, cfg.FrontendURL).Run(ctx)
	}()
	go func() { defer background.Done(); notification.NewRetentionCleaner(db).Run(ctx) }()
	reminders, err := notification.NewReminderScheduler(db, outbox, cfg.FrontendURL, cfg.ReminderTimezone)
	if err != nil {
		slog.Error("deadline reminder scheduler initialization failed", "error", err)
		os.Exit(1)
	}
	background.Add(1)
	go func() { defer background.Done(); reminders.Run(ctx) }()
	slog.Info("email worker started")
	runErr := notification.NewWorker(queue, notification.NewSMTPSender(cfg)).Run(ctx)
	stop()
	background.Wait()
	if runErr != nil {
		slog.Error("email worker failed", "error", runErr)
		os.Exit(1)
	}
	slog.Info("email worker stopped")
}
