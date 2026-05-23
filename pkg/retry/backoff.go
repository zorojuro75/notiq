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
	exp := math.Pow(2, float64(attempt))
	delay := time.Duration(float64(baseDelay) * exp)

	jitter := time.Duration(rand.Intn(jitterMax)) * time.Millisecond

	total := min(delay + jitter, maxDelay)

	return total
}