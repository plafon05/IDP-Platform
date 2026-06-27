package notification

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

const maxAttempts = 3

type Sender interface {
	Send(to []string, message RenderedMessage) error
}

type Worker struct {
	queue  *Queue
	sender Sender
}

func NewWorker(queue *Queue, sender Sender) *Worker { return &Worker{queue: queue, sender: sender} }

func (w *Worker) Run(ctx context.Context) error {
	for {
		job, err := w.queue.Dequeue(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if IsQueueTimeout(err) {
				continue
			}
			return err
		}
		message, err := Render(job)
		if err == nil {
			err = w.sender.Send(job.To, message)
		}
		if err == nil {
			slog.Info("email sent", "template", job.Template, "recipients", len(job.To))
			continue
		}
		job.Attempts++
		slog.Error("email delivery failed", "template", job.Template, "attempt", job.Attempts, "error", err)
		if job.Attempts >= maxAttempts {
			if deadErr := w.queue.DeadLetter(ctx, job); deadErr != nil {
				return errors.Join(err, deadErr)
			}
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Duration(job.Attempts) * time.Second):
		}
		if err := w.queue.Enqueue(ctx, job); err != nil {
			return err
		}
	}
}
