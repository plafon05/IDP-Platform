package notification

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestQueueRecoversProcessingJob(t *testing.T) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Skip("REDIS_URL is not set")
	}
	key := "idp:test:queue:" + time.Now().Format("20060102150405.000000000")
	queue, err := NewQueue(redisURL, key)
	if err != nil {
		t.Fatal(err)
	}
	defer queue.Close()
	defer queue.client.Del(context.Background(), key, key+":processing", key+":dead")
	if err := queue.Ping(context.Background()); err != nil {
		t.Skipf("Redis is unavailable: %v", err)
	}

	ctx := context.Background()
	job := Job{To: []string{"user@example.test"}, Template: PasswordResetTemplate}
	if err := queue.Enqueue(ctx, job); err != nil {
		t.Fatal(err)
	}
	dequeued, payload, err := queue.Dequeue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dequeued.Template != job.Template || payload == "" {
		t.Fatalf("unexpected dequeued job: %#v", dequeued)
	}
	if err := queue.RecoverProcessing(ctx); err != nil {
		t.Fatal(err)
	}
	recovered, recoveredPayload, err := queue.Dequeue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Template != job.Template || recoveredPayload != payload {
		t.Fatalf("job was not recovered: %#v", recovered)
	}
	if err := queue.Ack(ctx, recoveredPayload); err != nil {
		t.Fatal(err)
	}
	count, err := queue.client.LLen(ctx, key+":processing").Result()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("processing queue contains %d jobs after acknowledgement", count)
	}
}
