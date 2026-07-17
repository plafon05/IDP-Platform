package handler

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"idp-platform/backend/internal/httpjson"

	"github.com/redis/go-redis/v9"
)

const incrementWindowScript = `
local count = redis.call("INCR", KEYS[1])
if count == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return count
`

type rateLimitCounter interface {
	Increment(context.Context, string, time.Duration) (int64, error)
}

type redisRateLimitCounter struct{ client *redis.Client }

func newRedisRateLimitCounter(redisURL string) (*redisRateLimitCounter, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	return &redisRateLimitCounter{client: redis.NewClient(options)}, nil
}

func (c *redisRateLimitCounter) Increment(ctx context.Context, key string, window time.Duration) (int64, error) {
	return c.client.Eval(ctx, incrementWindowScript, []string{key}, window.Milliseconds()).Int64()
}

type ipRateLimiter struct {
	counter rateLimitCounter
	limit   int64
	window  time.Duration
}

func (l ipRateLimiter) middleware(route string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		key := "ratelimit:" + route + ":" + ip
		count, err := l.counter.Increment(r.Context(), key, l.window)
		if err != nil {
			slog.Error("rate limiter unavailable", "route", route, "error", err)
			httpjson.WriteError(w, http.StatusServiceUnavailable, "RATE_LIMIT_UNAVAILABLE", "Authentication service temporarily unavailable")
			return
		}
		if count > l.limit {
			httpjson.WriteError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		candidate := strings.TrimSpace(strings.Split(forwarded, ",")[0])
		if parsed := net.ParseIP(candidate); parsed != nil {
			return parsed.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		if parsed := net.ParseIP(host); parsed != nil {
			return parsed.String()
		}
	}
	if parsed := net.ParseIP(strings.TrimSpace(r.RemoteAddr)); parsed != nil {
		return parsed.String()
	}
	return "unknown"
}
