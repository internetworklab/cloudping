package main

import (
	"fmt"
	"time"

	pkgratelimit "example.com/rbmq-demo/pkg/throttle"
)

func main() {

	N := 500
	dataChan := make(chan interface{})
	// generator goroutine, generating mock samples at death speed
	go func() {
		defer close(dataChan)
		for i := 0; i < N; i++ {
			dataChan <- i
		}
	}()

	throttleConfig := pkgratelimit.TokenBasedThrottleConfig{
		RefreshInterval:       1 * time.Second,
		TokenQuotaPerInterval: 100,
	}
	tbThrottle := pkgratelimit.NewTokenBasedThrottle(throttleConfig)
	outChan := tbThrottle.Run(dataChan)

	smoother := pkgratelimit.BurstSmoother{
		LeastSampleInterval: 10 * time.Millisecond,
	}
	outChan = smoother.Run(outChan)

	speedMeasurer := pkgratelimit.SpeedMeasurer{
		RefreshInterval: 250 * time.Millisecond,
	}

	nullChan, recorderChan := speedMeasurer.Run(outChan)
	previousCounter := 0
	previousTimestamp := time.Now()
	for {
		select {
		case _, ok :=<-nullChan:
			if !ok {
				fmt.Println("No more null records")
				goto for_end
			}
			continue
		case record, ok := <-recorderChan:
			if !ok {
				fmt.Println("No more records")
				goto for_end
			}
			timeDelta := record.Timestamp.Sub(previousTimestamp)
			if timeDelta.Milliseconds() <= 100 {
				continue
			}
			increment := record.Counter - previousCounter
			speed := float64(increment) / timeDelta.Seconds()
			fmt.Printf("speed: %f items/s\n", speed)
			previousCounter = record.Counter
			previousTimestamp = record.Timestamp
		}
	}
	for_end:
}
