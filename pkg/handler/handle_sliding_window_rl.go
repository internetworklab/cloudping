package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/internetworklab/cloudping/pkg/ratelimit"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

// slidingWindowRateLimiterMiddleware wraps an http.Handler with per-key rate limiting
// using a sliding window algorithm backed by SlidingWindowRateLimitPool.
type slidingWindowRateLimiterMiddleware struct {
	pool        *ratelimit.SlidingWindowRateLimitPool
	keyGetter   func(r *http.Request) string
	nextHandler http.Handler
}

// WithSlidingWindowRatelimit returns an http.Handler that rate-limits requests
// using a sliding window. The keyGetter function extracts a rate-limit key from
// each request (e.g., client IP, user ID). Requests that exceed the limit receive
// a 429 Too Many Requests response.
func WithSlidingWindowRatelimit(nextHandler http.Handler, windowLength time.Duration, numRequestsLimit int, keyGetter func(r *http.Request) string) http.Handler {
	pool, err := ratelimit.NewSlidingWindowRateLimitPool(windowLength, numRequestsLimit)
	if err != nil {
		panic(fmt.Sprintf("failed to create sliding window rate limit pool: %v", err))
	}

	return &slidingWindowRateLimiterMiddleware{
		pool:        pool,
		keyGetter:   keyGetter,
		nextHandler: nextHandler,
	}
}

func (m *slidingWindowRateLimiterMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := m.keyGetter(r)

	ok, err := m.pool.Consume(r.Context(), key)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: "rate limit check failed"})
		return
	}

	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: "too many requests"})
		return
	}

	m.nextHandler.ServeHTTP(w, r)
}
