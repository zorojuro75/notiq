package retry

import (
	"math"
	"math/rand"
	"time"
)

const (
	baseDelay = 2 * time.Second
	maxDelay  = 5 * time.Minute
	jitterMax = 1000
)

func Backoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	// Compute base * 2^attempt and the jitter in float64, then clamp BEFORE
	// converting to a Duration. Converting an out-of-range float to int64
	// wraps to a negative value, so high attempt counts would otherwise yield
	// a negative (or absurd) delay. math.Pow overflows to +Inf cleanly here,
	// and +Inf compares greater than maxDelay, so the cap holds.
	total := float64(baseDelay)*math.Pow(2, float64(attempt)) +
		float64(time.Duration(rand.Intn(jitterMax))*time.Millisecond)

	if total >= float64(maxDelay) {
		return maxDelay
	}
	return time.Duration(total)
}
