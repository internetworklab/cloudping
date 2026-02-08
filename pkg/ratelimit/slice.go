package ratelimit

import "context"

func GetThrottledRequests[T any](ctx context.Context, requests []T, ratelimiter RateLimiter) chan T {
	requestChan := make(chan T)

	go func(ctx context.Context) {
		defer close(requestChan)
		if ratelimiter == nil {
			for _, req := range requests {
				requestChan <- req
			}
			return
		}

		inC, outC, _ := ratelimiter.GetIO(ctx)
		go func(ctx context.Context) {
			defer close(inC)
			for _, req := range requests {
				inC <- req
			}
		}(ctx)

		for req := range outC {
			requestChan <- req.(T)
		}
	}(ctx)
	return requestChan
}
