package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type fakeRateLimitCounter struct {
	mu     sync.Mutex
	counts map[string]int64
	err    error
}

func (f *fakeRateLimitCounter) Increment(_ context.Context, key string, _ time.Duration) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.counts == nil {
		f.counts = make(map[string]int64)
	}
	f.counts[key]++
	return f.counts[key], nil
}

func TestIPRateLimiterRejectsEleventhRequest(t *testing.T) {
	counter := &fakeRateLimitCounter{}
	limiter := ipRateLimiter{counter: counter, limit: 10, window: time.Minute}
	nextCalls := 0
	handler := limiter.middleware("/api/v1/auth/login", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalls++
		w.WriteHeader(http.StatusNoContent)
	}))

	for requestNumber := 1; requestNumber <= 11; requestNumber++ {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		request.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.2")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if requestNumber <= 10 && response.Code != http.StatusNoContent {
			t.Fatalf("request %d: got status %d", requestNumber, response.Code)
		}
		if requestNumber == 11 {
			if response.Code != http.StatusTooManyRequests {
				t.Fatalf("got status %d, want 429", response.Code)
			}
			var body struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Error.Code != "RATE_LIMITED" || body.Error.Message != "Too many requests" {
				t.Fatalf("unexpected response: %+v", body.Error)
			}
		}
	}
	if nextCalls != 10 {
		t.Fatalf("next called %d times, want 10", nextCalls)
	}
	if counter.counts["ratelimit:/api/v1/auth/login:203.0.113.10"] != 11 {
		t.Fatalf("first X-Forwarded-For address was not used")
	}
}

func TestIPRateLimiterUsesIndependentRouteAndIPKeys(t *testing.T) {
	counter := &fakeRateLimitCounter{}
	limiter := ipRateLimiter{counter: counter, limit: 10, window: time.Minute}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	for _, test := range []struct{ route, remote string }{
		{"/api/v1/auth/login", "192.0.2.1:1234"},
		{"/api/v1/auth/forgot-password", "192.0.2.1:5678"},
		{"/api/v1/auth/login", "192.0.2.2:1234"},
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, test.route, nil)
		request.RemoteAddr = test.remote
		limiter.middleware(test.route, next).ServeHTTP(response, request)
	}

	if len(counter.counts) != 3 {
		t.Fatalf("got %d keys, want 3: %#v", len(counter.counts), counter.counts)
	}
}

func TestIPRateLimiterFailsClosedWhenRedisIsUnavailable(t *testing.T) {
	limiter := ipRateLimiter{counter: &fakeRateLimitCounter{err: errors.New("redis unavailable")}, limit: 10, window: time.Minute}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", nil)
	limiter.middleware("/api/v1/auth/reset-password", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("got status %d, want 503", response.Code)
	}
}
