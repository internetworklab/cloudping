package ratelimit

import (
	"context"
	"fmt"
	"log"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// It holds a slice of timestamps (one per request)
type SlidingWindowRateLimitEntry struct {
	RequestTimestamps []time.Time
}

// Returns nil when ratelimit exceeded,
// returns a  non-nil and brand new `*SlidingWindowRateLimitEntry` with request timestamp inserted otherwise
func (swRLEnt *SlidingWindowRateLimitEntry) TryAppend(windowLength time.Duration, numRequestsLimit int) *SlidingWindowRateLimitEntry {
	now := time.Now()

	if swRLEnt == nil {
		ent := &SlidingWindowRateLimitEntry{
			RequestTimestamps: make([]time.Time, 0),
		}
		ent.RequestTimestamps = append(ent.RequestTimestamps, now)
		return ent
	}

	since := now.Add(-windowLength)

	// we are tying to find the first timestamp sample that is WITHIN the sliding window,
	// so that we can count how many timestamps are fall into the range of the sliding window.
	firstInWindowTimestampIdx := slices.IndexFunc(swRLEnt.RequestTimestamps, func(tx time.Time) bool { return !tx.Before(since) })

	if firstInWindowTimestampIdx == -1 {
		// all timestamp samples are out of the range of the sliding window
		newRLEnt := &SlidingWindowRateLimitEntry{
			RequestTimestamps: make([]time.Time, 0),
		}
		newRLEnt.RequestTimestamps = append(newRLEnt.RequestTimestamps, now)
		return newRLEnt
	} else {
		// some samples are within the range of the sliding window
		numRequests := len(swRLEnt.RequestTimestamps) - firstInWindowTimestampIdx
		if numRequests >= numRequestsLimit {
			// ratelimit exceeded
			return nil
		}
		newRLEnt := &SlidingWindowRateLimitEntry{
			RequestTimestamps: make([]time.Time, numRequests),
		}
		copy(newRLEnt.RequestTimestamps, swRLEnt.RequestTimestamps[firstInWindowTimestampIdx:])
		newRLEnt.RequestTimestamps = append(newRLEnt.RequestTimestamps, now)
		return newRLEnt
	}
}

// `SlidingWindowRateLimitPool` an implementation of the `RateLimitPool` interface
// It doesn't support `WaitForRefresh` feature for now.
// It also maintains a a sync.Map that maps rate-limit keys → atomic.Pointer[SlidingWindowRateLimitEntry]
type SlidingWindowRateLimitPool struct {
	// this map is dictinary that maps rate limit key to request timestamps
	requestTimestamps *sync.Map

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

	return &SlidingWindowRateLimitPool{
		windowLength:      windowLength,
		numRequestsLimit:  numRequestsLimit,
		requestTimestamps: &sync.Map{},
	}, nil
}

// Block until refresh
func (swRL *SlidingWindowRateLimitPool) WaitForRefresh(ctx context.Context) error {
	// By design, this implementation doesn't support `WaitForRefresh` feature.
	return ErrUnsupportedFeature
}

func (swRL *SlidingWindowRateLimitPool) tryInsertRequestTimestampToRLEntStore(key string, rlEntStorePtr *atomic.Pointer[SlidingWindowRateLimitEntry]) bool {
	for {
		originStore := rlEntStorePtr.Load()
		newStore := originStore.TryAppend(swRL.windowLength, swRL.numRequestsLimit)
		if newStore == nil {
			// ratelimit exceeded
			return false
		}
		if rlEntStorePtr.CompareAndSwap(originStore, newStore) {
			return true
		}
		log.Printf("retrying to insert key %s into rl entry store", key)
		continue
	}
}

// returns false when quota is exhausted, true otherwise
// the second return value is error, if any, such as, when timeout occurs
func (swRL *SlidingWindowRateLimitPool) Consume(ctx context.Context, key string) (bool, error) {

	perKeyStoreAny, _ := swRL.requestTimestamps.LoadOrStore(key, &atomic.Pointer[SlidingWindowRateLimitEntry]{})

	return swRL.tryInsertRequestTimestampToRLEntStore(key, perKeyStoreAny.(*atomic.Pointer[SlidingWindowRateLimitEntry])), nil
}
