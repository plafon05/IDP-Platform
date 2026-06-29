package notification

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const relayInterval = time.Second

type Publisher interface {
	Enqueue(ctx context.Context, job Job) error
	EnqueueTx(ctx context.Context, tx pgx.Tx, job Job) error
}

type Outbox struct{ db *pgxpool.Pool }

func NewOutbox(db *pgxpool.Pool) *Outbox { return &Outbox{db: db} }

func (o *Outbox) Enqueue(ctx context.Context, job Job) error {
	return insertJob(ctx, o.db, job)
}

func (o *Outbox) EnqueueTx(ctx context.Context, tx pgx.Tx, job Job) error {
	return insertJob(ctx, tx, job)
}

type queryExecer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func insertJob(ctx context.Context, db queryExecer, job Job) error {
	if len(job.To) == 0 || job.Template == "" {
		return errors.New("invalid notification job")
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `INSERT INTO notification_outbox (payload) VALUES ($1)`, payload)
	return err
}

type Relay struct {
	db    *pgxpool.Pool
	queue *Queue
}

func NewRelay(db *pgxpool.Pool, queue *Queue) *Relay { return &Relay{db: db, queue: queue} }

func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(relayInterval)
	defer ticker.Stop()
	for {
		if err := r.publishBatch(ctx); err != nil && ctx.Err() == nil {
			slog.Error("notification outbox relay failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *Relay) publishBatch(ctx context.Context) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT id, payload FROM notification_outbox
		WHERE published_at IS NULL ORDER BY created_at
		FOR UPDATE SKIP LOCKED LIMIT 50
	`)
	if err != nil {
		return err
	}
	type item struct {
		id  string
		job Job
	}
	items := make([]item, 0, 50)
	for rows.Next() {
		var current item
		var payload []byte
		if err := rows.Scan(&current.id, &payload); err != nil {
			rows.Close()
			return err
		}
		if err := json.Unmarshal(payload, &current.job); err != nil {
			rows.Close()
			return err
		}
		items = append(items, current)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, current := range items {
		if err := r.queue.Enqueue(ctx, current.job); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE notification_outbox SET published_at=NOW() WHERE id=$1`, current.id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
