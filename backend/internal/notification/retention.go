package notification

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const retentionInterval = 24 * time.Hour

// RetentionCleaner removes only expired or historical delivery data. Pending
// outbox records are deliberately retained so an unsent notification is never
// discarded by cleanup.
type RetentionCleaner struct{ db *pgxpool.Pool }

func NewRetentionCleaner(db *pgxpool.Pool) *RetentionCleaner { return &RetentionCleaner{db: db} }

func (c *RetentionCleaner) Run(ctx context.Context) {
	c.clean(ctx)
	ticker := time.NewTicker(retentionInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.clean(ctx)
		}
	}
}

func (c *RetentionCleaner) clean(ctx context.Context) {
	queries := []struct {
		name  string
		query string
	}{
		{"audit logs", `DELETE FROM audit_logs WHERE created_at < NOW() - INTERVAL '1 year'`},
		{"in-app notifications", `DELETE FROM in_app_notifications WHERE created_at < NOW() - INTERVAL '30 days'`},
		{"published outbox", `DELETE FROM notification_outbox WHERE published_at IS NOT NULL AND published_at < NOW() - INTERVAL '30 days'`},
		{"refresh tokens", `DELETE FROM refresh_tokens WHERE expires_at < NOW() - INTERVAL '7 days'`},
		{"password reset tokens", `DELETE FROM password_reset_tokens WHERE expires_at < NOW() - INTERVAL '7 days'`},
	}
	for _, item := range queries {
		tag, err := c.db.Exec(ctx, item.query)
		if err != nil {
			slog.Error("retention cleanup failed", "target", item.name, "error", err)
			continue
		}
		if tag.RowsAffected() > 0 {
			slog.Info("retention cleanup completed", "target", item.name, "deleted", tag.RowsAffected())
		}
	}
}
