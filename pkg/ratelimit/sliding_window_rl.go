package ratelimit

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"
)

type SlidingWindowRateLimitEntry struct {
	RequestTimestamps []time.Time
}

// Returns nil when ratelimit exceeded,
// returns a  non-nil and brand new `*SlidingWindowRateLimitEntry` with request timestamp inserted otherwise
func (swRLEnt *SlidingWindowRateLimitEntry) TryAppend(windowLength time.Duration, numRequestsLimit int) *SlidingWindowRateLimitEntry {
	since := time.Now().Add(-windowLength)
	idx := slices.IndexFunc(swRLEnt.RequestTimestamps, func(tx time.Time) bool { return tx.Before(since) })
	if idx == -1 {
		newRLEnt := &SlidingWindowRateLimitEntry{
			RequestTimestamps: make([]time.Time, len(swRLEnt.RequestTimestamps)),
		}
		copy(newRLEnt.RequestTimestamps, swRLEnt.RequestTimestamps)
		newRLEnt.RequestTimestamps = append(newRLEnt.RequestTimestamps, time.Now())
		return newRLEnt
	} else {
		numRequests := len(swRLEnt.RequestTimestamps) - idx
		if numRequests >= numRequestsLimit {
			// ratelimit exceeded
			return nil
		}
		newRLEnt := &SlidingWindowRateLimitEntry{
			RequestTimestamps: make([]time.Time, numRequests),
		}
		copy(newRLEnt.RequestTimestamps, swRLEnt.RequestTimestamps[idx:])
		newRLEnt.RequestTimestamps = append(newRLEnt.RequestTimestamps, time.Now())
		return newRLEnt
	}
}

// `SlidingWindowRateLimitPool` an implementation of the `RateLimitPool` interface
// It doesn't support `WaitForRefresh` feature for now.
type SlidingWindowRateLimitPool struct {
	// this map is dictinary that maps rate limit key to request timestamps
	requestTimestamps sync.Map

	windowLength     time.Duration
	numRequestsLimit int
}

func NewSlidingWindowRateLimitPool(windowLength time.Duration, numRequestsLimit int) (*SlidingWindowRateLimitPool, error) {
	if windowLength <= 0 {
		return nil, fmt.Errorf("invalid window length: %v", windowLength.String())
	}

	if numRequestsLimit <= 0 {
		return nil, fmt.Errorf("invalid num requests limit: %v", numRequestsLimit)
	}

	// todo
	return nil, nil
}

// Block until refresh
func (swRL *SlidingWindowRateLimitPool) WaitForRefresh(ctx context.Context) error {
	// By design, this implementation doesn't support `WaitForRefresh` feature.
	return ErrUnsupportedFeature
}

// returns false when quota is exhausted, true otherwise
// the second return value is error, if any, such as, when timeout occurs
func (swRL *SlidingWindowRateLimitPool) Consume(ctx context.Context, key string) (bool, error) {
	// todo
	return false, nil
}
