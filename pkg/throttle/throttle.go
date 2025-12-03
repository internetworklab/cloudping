package throttle

import (
	"context"
	"time"
)

type TokenBasedThrottleConfig struct {
	RefreshInterval       time.Duration
	TokenQuotaPerInterval int
}

type RateLimiterToken struct {
	Quota        int
	BufferedChan chan interface{}
}

type TokenBasedThrottle struct {
	OutC   chan interface{}
	config TokenBasedThrottleConfig
}

func NewTokenBasedThrottle(config TokenBasedThrottleConfig) *TokenBasedThrottle {
	return &TokenBasedThrottle{
		config: config,
	}
}

func (tbThrottle *TokenBasedThrottle) Run(inChan <-chan interface{}) <-chan interface{} {
	outChan := make(chan interface{})

	ctx := context.TODO()
	ctx, cancel := context.WithCancel(ctx)

	tokensChan := make(chan int, 1)
	tokensChan <- tbThrottle.config.TokenQuotaPerInterval

	// token generator goroutine
	go func() {
		ticker := time.NewTicker(tbThrottle.config.RefreshInterval)
		defer ticker.Stop()
		defer close(tokensChan)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				tokensChan <- tbThrottle.config.TokenQuotaPerInterval
			}
		}
	}()

	// copying goroutine
	go func() {
		defer close(outChan)
		defer cancel()

		quota := 0
		for item := range inChan {
			if quota == 0 {
				quotaInc := <-tokensChan
				quota += quotaInc
				if quota > tbThrottle.config.TokenQuotaPerInterval {
					quota = tbThrottle.config.TokenQuotaPerInterval
				}
			}
			outChan <- item
			quota--
		}

	}()

	return outChan
}

type SpeedMeasurer struct {
	RefreshInterval time.Duration
}

type SpeedRecord struct {
	Timestamp time.Time
	Counter   int
}

func (sm *SpeedMeasurer) Run(inChan <-chan interface{}) (<-chan interface{}, <-chan SpeedRecord) {
	outChan := make(chan interface{})
	speedRecordChan := make(chan SpeedRecord, 1)

	ctx := context.TODO()
	ctx, cancel := context.WithCancel(ctx)

	var counter *int = new(int)
	*counter = 0

	go func() {
		defer close(speedRecordChan)
		speedRecord := SpeedRecord{
			Timestamp: time.Now(),
			Counter:   *counter,
		}
		speedRecordChan <- speedRecord
		ticker := time.NewTicker(sm.RefreshInterval)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				speedRecord := SpeedRecord{
					Timestamp: time.Now(),
					Counter:   *counter,
				}
				speedRecordChan <- speedRecord
			}
		}
	}()

	go func() {
		defer close(outChan)
		defer cancel()

		for item := range inChan {
			outChan <- item
			*counter = *counter + 1
		}
	}()

	return outChan, speedRecordChan
}

type BurstFlattener struct {
	LeastSampleInterval time.Duration
}

func (bf *BurstFlattener) Run(inChan <-chan interface{}) <-chan interface{} {
	outChan := make(chan interface{})

	go func() {
		defer close(outChan)
		for item := range inChan {
			time.Sleep(bf.LeastSampleInterval)
			outChan <- item
		}
	}()

	return outChan
}
