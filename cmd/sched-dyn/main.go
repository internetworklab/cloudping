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

type MockPacket struct {
	Symbol int
	Seq    int
}

func (mp MockPacket) String() string {
	return fmt.Sprintf("{symbol: %d, seq: %d}", mp.Symbol, mp.Seq)
}

func anonymousSource(ctx context.Context, symbol int, limit *int, interval *time.Duration) chan interface{} {
	outC := make(chan interface{})
	numItemsCopied := 0
	go func() {

		// close source channel to signal the down stream consumers
		defer close(outC)
		defer fmt.Printf("source %d is closed\n", symbol)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				outC <- MockPacket{Symbol: symbol, Seq: numItemsCopied}
				numItemsCopied++
				if limit != nil && *limit > 0 && numItemsCopied >= *limit {
					return
				}
				if interval != nil {
					time.Sleep(*interval)
				}
			}
		}
	}()
	return outC
}

func add(ctx context.Context, symbol int, limit *int, evCenter *pkgthrottle.TimeSlicedEVLoopSched, interval *time.Duration) interface{} {

	dataSource := anonymousSource(ctx, symbol, limit, interval)

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	opaqueNodeId, err := evCenter.AddInput(ctx, dataSource)
	if err != nil {
		log.Fatalf("failed to add input to evCenter: %v", err)
	}
	return opaqueNodeId
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
	errorChan := evCenter.Run(ctx)

	outC := evCenter.GetOutput()

	var nodesCount *int = new(int)
	*nodesCount = 0

	var numEventsPassed *int = new(int)
	*numEventsPassed = 0

	aLim := 80
	bLim := 160
	cLim := 240

	// consumer goroutine
	go func() {
		stat := make(map[int]int)
		var total *int = new(int)

		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				fmt.Println("Total: ", *total)
			}
		}()

		for muxedItemRaw := range outC {
			muxedItem, ok := muxedItemRaw.(MockPacket)
			if !ok {
				panic("unexpected item type, it's not of a type of MockPacket")
			}

			stat[muxedItem.Symbol]++
			*total = *total + 1
			if *total%10 == 0 {
				fmt.Println("Current statistics:")
				for k, v := range stat {
					fmt.Printf("%d: %d, %.2f%%\n", k, v, 100*float64(v)/float64(*total))
				}
				fmt.Println("total: ", *total)
			}

			if *total == aLim+bLim+cLim+aLim+aLim {
				fmt.Println("Final statistics:")
				for k, v := range stat {
					fmt.Printf("%d: %d, %.2f%%\n", k, v, 100*float64(v)/float64(*total))
				}
			}
		}

	}()

	wg := sync.WaitGroup{}

	evCenter.RegisterCustomEVHandler(ctx, pkgthrottle.TSSchedEVNodeDrained, "node_drained", func(evObj *pkgthrottle.TSSchedEVObject) error {

		nodeId, ok := evObj.Payload.(int)
		if !ok {
			panic("unexpected ev payload, it's not of a type of int")
		}

		log.Printf("node %d is drained", nodeId)
		wg.Done()

		evObj.Result <- nil
		return nil
	})

	dur := 500 * time.Millisecond
	var sleepIntv *time.Duration = &dur
	opaqueNodeId := add(ctx, 1, &aLim, evCenter, sleepIntv)
	log.Printf("node %v is added", opaqueNodeId)
	wg.Add(1)

	opaqueNodeId = add(ctx, 2, &bLim, evCenter, sleepIntv)
	log.Printf("node %v is added", opaqueNodeId)
	wg.Add(1)

	opaqueNodeId = add(ctx, 3, &cLim, evCenter, sleepIntv)
	log.Printf("node %v is added", opaqueNodeId)
	wg.Add(1)

	// some timer background task
	go func() {
		<-time.After(20 * time.Second)
		opaqueNodeId := add(ctx, 4, &aLim, evCenter, sleepIntv)
		log.Printf("node %v is added", opaqueNodeId)
		wg.Add(1)

		<-time.After(20 * time.Second)
		opaqueNodeId = add(ctx, 5, &aLim, evCenter, sleepIntv)
		log.Printf("node %v is added", opaqueNodeId)
		wg.Add(1)
	}()

	sig := <-sigs
	log.Println("signal received: ", sig, " exitting...")

	log.Println("waiting for all nodes to be drained")
	wg.Wait()
	log.Println("all nodes are drained")

	cancel()

	err, ok := <-errorChan
	if ok && err != nil {
		log.Fatalf("event loop error: %v", err)
	}
}
