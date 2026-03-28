package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSlidingWindowRateLimitPool_ConcurrentSpam(t *testing.T) {
	const (
		windowLength     = 15 * time.Second
		numRequestsLimit = 50
		testDuration     = 5 * time.Second
		numGoroutines    = 4
		requestInterval  = 50 * time.Millisecond

		// toleranceFactor is the allowed margin above the theoretical max
		// to account for timing jitter in concurrent scenarios.
		toleranceFactor = 1.20
	)

	pool, err := NewSlidingWindowRateLimitPool(windowLength, numRequestsLimit)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	var successCount atomic.Int64
	var rejectCount atomic.Int64
	var errorCount atomic.Int64

	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	var wg sync.WaitGroup
	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(requestInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					ok, err := pool.Consume(ctx, "test-key")
					if err != nil {
						errorCount.Add(1)
						return
					}
					if ok {
						successCount.Add(1)
					} else {
						rejectCount.Add(1)
					}
				}
			}
		}()
	}

	wg.Wait()

	successes := successCount.Load()
	rejects := rejectCount.Load()
	errors := errorCount.Load()

	// For a sliding window, the theoretical max is:
	//   - numRequestsLimit when testDuration < windowLength (entire test is within one window)
	//   - numRequestsLimit * testDuration / windowLength when testDuration >= windowLength
	theoreticalMax := max(float64(numRequestsLimit), float64(numRequestsLimit)*float64(testDuration)/float64(windowLength))

	attackRate := float64(numGoroutines) * float64(time.Second) / float64(requestInterval)
	t.Logf("config: window=%v, limit=%d, duration=%v, goroutines=%d, interval=%v",
		windowLength, numRequestsLimit, testDuration, numGoroutines, requestInterval)
	t.Logf("results: successes=%d, rejects=%d, errors=%d, attack_rate=%.0f req/s, theoretical_max=%.0f",
		successes, rejects, errors, attackRate, theoreticalMax)
	t.Logf("effective rate: %.1f req/s (theoretical max: %.1f req/s)",
		float64(successes)/testDuration.Seconds(), theoreticalMax/testDuration.Seconds())

	if errors > 0 {
		t.Errorf("unexpected errors from Consume: %d", errors)
	}

	// Verify some requests succeeded
	if successes == 0 {
		t.Fatal("expected some requests to succeed, but none did")
	}

	// Verify some requests were rejected (proves rate limiting is working)
	if rejects == 0 {
		t.Fatal("expected some requests to be rejected under heavy load, but none were")
	}

	// Verify the effective rate is bounded: allow tolerance above theoretical max
	upperBound := theoreticalMax * toleranceFactor
	if float64(successes) > upperBound {
		t.Errorf("rate limit exceeded: got %d successes (%.1f req/s), theoretical max is %.0f (%.1f req/s)",
			successes, float64(successes)/testDuration.Seconds(), theoreticalMax, theoreticalMax/testDuration.Seconds())
	}
}
