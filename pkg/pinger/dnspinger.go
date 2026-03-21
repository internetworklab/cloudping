package pinger

import (
	"context"
	"crypto/x509"
	"errors"
	"sync"

	pkgdnsprobe "example.com/rbmq-demo/pkg/dnsprobe"
	pkgratelimit "example.com/rbmq-demo/pkg/ratelimit"
	pkgutils "example.com/rbmq-demo/pkg/utils"
)

type DNSPinger struct {
	Requests    []pkgdnsprobe.LookupParameter
	RateLimiter pkgratelimit.RateLimiter
	AddCAPaths  []string
}

func (dp *DNSPinger) Ping(ctx context.Context) <-chan PingEvent {
	evChan := make(chan PingEvent)
	go func() {
		var certPool *x509.CertPool = nil
		if len(dp.AddCAPaths) > 0 {
			var err error
			certPool, err = pkgutils.GetExtendedCAPool(dp.AddCAPaths)
			if err != nil {
				evChan <- PingEvent{Error: errors.New("Can't load additional CA cert pool")}
				return
			}
		}

		defer close(evChan)
		wg := &sync.WaitGroup{}
		defer wg.Wait()

		for request := range pkgratelimit.GetThrottledRequests(ctx, dp.Requests, dp.RateLimiter) {
			wg.Add(1)
			go func(req pkgdnsprobe.LookupParameter) {
				defer wg.Done()
				queryResult, err := pkgdnsprobe.LookupDNS(ctx, req, certPool)
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
