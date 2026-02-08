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
			go func(req pkghttpprobe.HTTPProbe) {
				defer wg.Done()
				for ev := range req.Do(ctx) {
					wrappedEV := PingEvent{
						Data: &ev,
					}
					if ev.Error != "" {
						wrappedEV.Err = &ev.Error
					}
					evChan <- wrappedEV
				}
			}(request)
		}
	}()
	return evChan
}
