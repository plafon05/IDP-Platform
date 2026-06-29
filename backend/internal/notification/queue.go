package notification

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type Job struct {
	To       []string          `json:"to"`
	Template string            `json:"template"`
	Data     map[string]string `json:"data"`
	Attempts int               `json:"attempts"`
}

type Queue struct {
	client *redis.Client
	key    string
}

func NewQueue(redisURL, key string) (*Queue, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	return &Queue{client: redis.NewClient(options), key: key}, nil
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

func (q *Queue) Dequeue(ctx context.Context) (Job, error) {
	result, err := q.client.BRPop(ctx, 5*time.Second, q.key).Result()
	if err != nil {
		return Job{}, err
	}
	var job Job
	if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
		return Job{}, err
	}
	return job, nil
}

func (q *Queue) DeadLetter(ctx context.Context, job Job) error {
	payload, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return q.client.LPush(ctx, q.key+":dead", payload).Err()
}

func IsQueueTimeout(err error) bool { return errors.Is(err, redis.Nil) }
