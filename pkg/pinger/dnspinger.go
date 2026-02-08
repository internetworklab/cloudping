package pinger

import (
	"context"
	"sync"

	pkgdnsprobe "example.com/rbmq-demo/pkg/dnsprobe"
	pkgratelimit "example.com/rbmq-demo/pkg/ratelimit"
)

type DNSPinger struct {
	Requests    []pkgdnsprobe.LookupParameter
	RateLimiter pkgratelimit.RateLimiter
}

func (dp *DNSPinger) Ping(ctx context.Context) <-chan PingEvent {
	evChan := make(chan PingEvent)
	go func() {
		defer close(evChan)
		wg := &sync.WaitGroup{}
		defer wg.Wait()
		for request := range pkgratelimit.GetThrottledRequests(ctx, dp.Requests, dp.RateLimiter) {
			wg.Add(1)
			go func(req pkgdnsprobe.LookupParameter) {
				defer wg.Done()
				queryResult, err := pkgdnsprobe.LookupDNS(ctx, req)
				if err != nil {
					evChan <- PingEvent{Error: err}
					return
				}
				queryResult, err = queryResult.PreStringify()
				if err != nil {
					evChan <- PingEvent{Error: err}
					return
				}
				evChan <- PingEvent{Data: queryResult}
			}(request)
		}
	}()
	return evChan
}
