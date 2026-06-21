package retry

import (
	"testing"
	"time"
)

func TestBackoff_GrowsWithAttempt(t *testing.T) {
	prev := time.Duration(0)
	for attempt := range 6 {
		d := Backoff(attempt)

		if attempt > 0 && d < prev-time.Second {
			t.Errorf("attempt %d delay %v is not growing from %v", attempt, d, prev)
		}
		prev = d
		t.Logf("attempt %d → %v", attempt, d)
	}
}

func TestBackoff_NeverExceedsMaxDelay(t *testing.T) {
	for range 100 {
		d := Backoff(20)
		if d > maxDelay+time.Second {
			t.Errorf("delay %v exceeded maxDelay %v", d, maxDelay)
		}
	}
}

func TestBackoff_HighAttemptStaysCapped(t *testing.T) {
	// Large attempts must not overflow into a negative or out-of-range delay.
	for _, attempt := range []int{33, 60, 100, 1000} {
		d := Backoff(attempt)
		if d < 0 {
			t.Errorf("attempt %d produced negative delay %v", attempt, d)
		}
		if d > maxDelay {
			t.Errorf("attempt %d delay %v exceeded maxDelay %v", attempt, d, maxDelay)
		}
	}
}

func TestBackoff_HasJitter(t *testing.T) {
	results := make(map[time.Duration]bool)
	for range 10 {
		results[Backoff(2)] = true
	}
	if len(results) == 1 {
		t.Error("backoff has no jitter — all 10 results were identical")
	}
}
