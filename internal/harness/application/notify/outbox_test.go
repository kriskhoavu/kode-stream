package notify

import (
	"testing"
	"time"
)

func TestBackoffDoublesUpToTenMinutes(t *testing.T) {
	for attempts, want := range map[int]time.Duration{1: 5 * time.Second, 2: 10 * time.Second, 3: 20 * time.Second, 10: 10 * time.Minute} {
		if got := Backoff(attempts); got != want {
			t.Fatalf("Backoff(%d) = %s, want %s", attempts, got, want)
		}
	}
	if EventID(42) != "evt-42" {
		t.Fatal("event id format changed")
	}
}
