package ratelimit

import (
	"context"
	"fmt"
)

type KeyFunc func(ctx context.Context, object interface{}) (string, error)

type MemoryBasedRateLimiter struct {
	Pool   RateLimitPool
	GetKey KeyFunc
}

func (rl *MemoryBasedRateLimiter) GetIO(ctx context.Context, input chan Keyable) (chan interface{}, chan error) {
	outC := make(chan interface{})
	outErrC := make(chan error, 1)

	go func(ctx context.Context) {
		defer close(outC)
		defer close(outErrC)

		for {
			select {
			case <-ctx.Done():
				return
			case val, ok := <-input:
				if !ok {
					return
				}

				key, err := rl.GetKey(ctx, val)
				if err != nil {
					outErrC <- fmt.Errorf("failed to get key: %w", err)
					return
				}

				ok, err = rl.Pool.Consume(ctx, key)
				if err != nil {
					outErrC <- err
					return
				}

				if !ok {
					if err := rl.Pool.WaitForRefresh(ctx); err != nil {
						outErrC <- err
						return
					}
				}

				outC <- val
			}
		}
	}(ctx)

	return outC, outErrC
}
