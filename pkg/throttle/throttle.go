package throttle

import (
	"context"
	"time"
)

// todo: this is a todo
type TokenBasedThrottleConfig struct {
	RefreshInterval       time.Duration
	TokenQuotaPerInterval int
}

type RateLimiterToken struct {
	Quota        int
	BufferedChan chan interface{}
}

type TokenBasedThrottle struct {
	OutC      chan interface{}
	config    TokenBasedThrottleConfig
	serviceCh chan *RateLimiterToken
}

func NewTokenBasedThrottle(config TokenBasedThrottleConfig) *TokenBasedThrottle {
	return &TokenBasedThrottle{
		config: config,
	}
}

func (tbThrottle *TokenBasedThrottle) Run(ctx context.Context) {
	// start fifo muxer
	muxerCh := make(chan chan interface{})
	go func() {
		for subChan := range muxerCh {
			for item := range subChan {
				tbThrottle.OutC <- item
			}
		}
	}()

	// start server
	go func() {
		defer close(tbThrottle.serviceCh)
		defer close(muxerCh)

		quota := tbThrottle.config.TokenQuotaPerInterval
		refresher := time.NewTicker(tbThrottle.config.RefreshInterval)

		for {
			token := &RateLimiterToken{
				Quota:        quota,
				BufferedChan: make(chan interface{}, quota),
			}

			select {
			case <-ctx.Done():
				return
			case <-refresher.C:
				quota = tbThrottle.config.TokenQuotaPerInterval
			case tbThrottle.serviceCh <- token:
				muxerCh <- token.BufferedChan
			}
		}
	}()
}

func (tbThrottle *TokenBasedThrottle) GetWriter() chan<- interface{} {
	inputCh := make(chan interface{})
	go func() {
		var upstreamCh chan interface{}
		var upstreamQuota int = 0

		for item := range inputCh {
			if upstreamQuota == 0 || upstreamCh == nil {
				newToken := <-tbThrottle.serviceCh
				upstreamCh = newToken.BufferedChan
				upstreamQuota = newToken.Quota
			}
			upstreamCh <- item
			upstreamQuota--
		}
	}()
	return inputCh
}
