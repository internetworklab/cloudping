package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	pkgthrottle "example.com/rbmq-demo/pkg/throttle"
)

func anonymousSource(ctx context.Context, symbol int, limit *int) chan interface{} {
	outC := make(chan interface{})
	numItemsCopied := 0
	go func() {
		// close source channel to signal the down stream consumers
		defer close(outC)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				outC <- symbol
				numItemsCopied++
				if limit != nil && *limit > 0 && numItemsCopied >= *limit {
					log.Printf("source %d reached limit %d, stopping", symbol, *limit)
					return
				}
			}
		}
	}()
	return outC
}

func main() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	evCenter, err := pkgthrottle.NewTimeSlicedEVLoopSched(&pkgthrottle.TimeSlicedEVLoopSchedConfig{})
	if err != nil {
		log.Fatalf("failed to create evCenter: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	evCenter.Run(ctx)

	outC := evCenter.GetOutput()

	var nodesCount *int = new(int)
	*nodesCount = 0

	nodesCountMutex := new(sync.Mutex)

	var numEventsPassed *int = new(int)
	*numEventsPassed = 0

	add := func(symbol int, limit *int, evCenter *pkgthrottle.TimeSlicedEVLoopSched) {
		nodesCountMutex.Lock()
		defer nodesCountMutex.Unlock()

		dataSource := anonymousSource(ctx, symbol, limit)
		if err := evCenter.AddInput(dataSource); err != nil {
			log.Fatalf("failed to add input to evCenter: %v", err)
		}
	}

	aLim := 8000000
	bLim := 16000000
	cLim := 24000000

	// consumer goroutine
	go func() {
		stat := make(map[string]int)
		var total *int = new(int)

		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				fmt.Println("Total: ", *total)
			}
		}()

		for muxedItem := range outC {
			stat[muxedItem.(string)]++
			*total = *total + 1
			if *total%1000 == 0 {
				for k, v := range stat {
					fmt.Printf("%s: %d, %.2f%%\n", k, v, 100*float64(v)/float64(*total))
				}
			}
			if *total == aLim+bLim+cLim {
				fmt.Println("Final statistics:")
				for k, v := range stat {
					fmt.Printf("%s: %d, %.2f%%\n", k, v, 100*float64(v)/float64(*total))
				}
			}
		}

	}()

	add(1, &aLim, evCenter)
	add(2, &bLim, evCenter)
	add(3, &cLim, evCenter)

	sig := <-sigs
	fmt.Println("signal received: ", sig, " exitting...")
}
