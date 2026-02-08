package pinger

import (
	"context"
	"sync"

	pkghttpprobe "example.com/rbmq-demo/pkg/httpprobe"
	pkgratelimit "example.com/rbmq-demo/pkg/ratelimit"
)

type HTTPPinger struct {
	Requests    []pkghttpprobe.HTTPProbe
	RateLimiter pkgratelimit.RateLimiter
}

func (dp *HTTPPinger) Ping(ctx context.Context) <-chan PingEvent {
	evChan := make(chan PingEvent)
	go func() {
		defer close(evChan)
		wg := &sync.WaitGroup{}
		defer wg.Wait()
		for request := range pkgratelimit.GetThrottledRequests(ctx, dp.Requests, dp.RateLimiter) {
			wg.Add(1)
			// Each request runs in a separate goroutine, so one slow HTTP request won't slow down the other requests,
			// Each request has a unique correlation_id, so one can easily pair the events with the corresponding original request.
			go func(req pkghttpprobe.HTTPProbe) {
				defer wg.Done()
				for ev := range req.Do(ctx) {
					wrappedEV := PingEvent{
						Data: &ev,
					}

					evChan <- wrappedEV
				}
			}(request)
		}
	}()
	return evChan
}
