package notification

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type Job struct {
	UserID   string            `json:"user_id,omitempty"`
	To       []string          `json:"to"`
	Template string            `json:"template"`
	Data     map[string]string `json:"data"`
	Attempts int               `json:"attempts"`
}

type Queue struct {
	client        *redis.Client
	key           string
	processingKey string
}

func NewQueue(redisURL, key string) (*Queue, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	return &Queue{client: redis.NewClient(options), key: key, processingKey: key + ":processing"}, nil
}

func (q *Queue) Ping(ctx context.Context) error { return q.client.Ping(ctx).Err() }
func (q *Queue) Close() error                   { return q.client.Close() }

func (q *Queue) Enqueue(ctx context.Context, job Job) error {
	if len(job.To) == 0 || job.Template == "" {
		return errors.New("invalid notification job")
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return q.client.LPush(ctx, q.key, payload).Err()
}

// Dequeue moves a job to a processing list before returning it, so a crash does
// not silently lose the job. Call Ack only after a successful delivery.
func (q *Queue) Dequeue(ctx context.Context) (Job, string, error) {
	payload, err := q.client.BRPopLPush(ctx, q.key, q.processingKey, 5*time.Second).Result()
	if err != nil {
		return Job{}, "", err
	}
	var job Job
	if err := json.Unmarshal([]byte(payload), &job); err != nil {
		return Job{}, payload, err
	}
	return job, payload, nil
}

func (q *Queue) Ack(ctx context.Context, payload string) error {
	return q.client.LRem(ctx, q.processingKey, 1, payload).Err()
}

func (q *Queue) Requeue(ctx context.Context, payload string, job Job) error {
	if err := q.Ack(ctx, payload); err != nil {
		return err
	}
	return q.Enqueue(ctx, job)
}

// RecoverProcessing returns jobs left by an interrupted worker to the queue.
func (q *Queue) RecoverProcessing(ctx context.Context) error {
	for {
		moved, err := q.client.LMove(ctx, q.processingKey, q.key, "RIGHT", "LEFT").Result()
		if errors.Is(err, redis.Nil) || moved == "" {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func (q *Queue) DeadLetter(ctx context.Context, job Job) error {
	payload, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return q.client.LPush(ctx, q.key+":dead", payload).Err()
}

func IsQueueTimeout(err error) bool { return errors.Is(err, redis.Nil) }
