package worker

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestPool_AllJobsRun verifies every submitted job executes exactly once.
func TestPool_AllJobsRun(t *testing.T) {
	pool := NewPool(5, 100)
	pool.Start()

	const numJobs = 50
	var count atomic.Int64

	for range numJobs {
		pool.Submit(func() {
			count.Add(1)
		})
	}

	pool.Shutdown()

	if count.Load() != numJobs {
		t.Errorf("expected %d jobs to run, got %d", numJobs, count.Load())
	}
}

// TestPool_BoundedConcurrency verifies no more than numWorkers jobs run at the same time.
func TestPool_BoundedConcurrency(t *testing.T) {
	const numWorkers = 3
	pool := NewPool(numWorkers, 100)
	pool.Start()

	var (
		active    atomic.Int64
		maxActive atomic.Int64
		mu        sync.Mutex
	)

	for range 30 {
		pool.Submit(func() {
			current := active.Add(1)

			// record peak concurrency
			mu.Lock()
			if current > maxActive.Load() {
				maxActive.Store(current)
			}
			mu.Unlock()

			time.Sleep(10 * time.Millisecond) // simulate work
			active.Add(-1)
		})
	}

	pool.Shutdown()

	if maxActive.Load() > numWorkers {
		t.Errorf("concurrency exceeded numWorkers: got %d, want <= %d",
			maxActive.Load(), numWorkers)
	}
}

// TestPool_GracefulShutdown verifies in-flight jobs complete before Shutdown returns.
func TestPool_GracefulShutdown(t *testing.T) {
	pool := NewPool(3, 50)
	pool.Start()

	var completed atomic.Int64

	for range 10 {
		pool.Submit(func() {
			time.Sleep(50 * time.Millisecond)
			completed.Add(1)
		})
	}

	pool.Shutdown()

	if completed.Load() != 10 {
		t.Errorf("shutdown didn't wait for jobs: completed %d of 10", completed.Load())
	}
}

func TestPool_PanicRecovery(t *testing.T) {
	pool := NewPool(2, 10)
	pool.Start()

	var completed atomic.Int64

	pool.Submit(func() {
		panic("something went wrong")
	})

	for range 5 {
		pool.Submit(func() {
			completed.Add(1)
		})
	}

	pool.Shutdown()

	if completed.Load() != 5 {
		t.Errorf("pool died after panic: only %d of 5 jobs ran", completed.Load())
	}
}

func TestPool_ShutdownIdempotent(t *testing.T) {
	pool := NewPool(2, 10)
	pool.Start()
	pool.Shutdown()
	pool.Shutdown()
}