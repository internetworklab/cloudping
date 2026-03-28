package ratelimit

import (
	"context"
	"fmt"
	"log"
)

type KeyFunc func(ctx context.Context, object interface{}) (string, error)

func GlobalKeyFunc(ctx context.Context, object interface{}) (string, error) {
	return "_", nil
}

type MemoryBasedRateLimiter struct {
	Pool   RateLimitPool
	GetKey KeyFunc
}

func (rl *MemoryBasedRateLimiter) GetIO(ctx context.Context) (chan<- interface{}, <-chan interface{}) {
	inC := make(chan interface{})
	outC := make(chan interface{})

	go func(ctx context.Context) {
		defer close(outC)

		for {
			select {
			case <-ctx.Done():
				return
			case val, ok := <-inC:
				if !ok {
					return
				}

				key, err := rl.GetKey(ctx, val)
				if err != nil {
					log.Fatal(fmt.Errorf("failed to get key: %w", err))
				}

				ok, err = rl.Pool.Consume(ctx, key)
				if err != nil {
					log.Fatal(fmt.Errorf("rate limiter pool error: %w", err))
				}

				if !ok {
					for {
						err := rl.Pool.WaitForRefresh(ctx)
						if err != nil {
							log.Fatal(fmt.Errorf("rate limiter pool error: %w", err))
						}

						ok, err = rl.Pool.Consume(ctx, key)
						if err != nil {
							log.Fatal(fmt.Errorf("rate limiter pool error: %w", err))
						}

						if ok {
							break
						}
					}
				}

				outC <- val
			}
		}
	}(ctx)

	return inC, outC
}
