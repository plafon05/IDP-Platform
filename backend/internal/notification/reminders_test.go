package notification

import (
	"testing"
	"time"
)

func TestNextReminderRun(t *testing.T) {
	location := time.FixedZone("test", 3*60*60)
	before := time.Date(2026, 6, 29, 5, 0, 0, 0, time.UTC)
	if got := nextReminderRun(before, location); got.Hour() != 9 || got.Day() != 29 {
		t.Fatalf("expected same-day 09:00, got %v", got)
	}
	after := time.Date(2026, 6, 29, 7, 0, 0, 0, time.UTC)
	if got := nextReminderRun(after, location); got.Hour() != 9 || got.Day() != 30 {
		t.Fatalf("expected next-day 09:00, got %v", got)
	}
}
