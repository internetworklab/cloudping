package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	pkgratelimit "example.com/rbmq-demo/pkg/ratelimit"
)

type Request struct {
	Class string
	Seq   int
	Date  time.Time
}

func (req *Request) String() string {
	return fmt.Sprintf("[%s] Seq=%d, Date=%s", req.Class, req.Seq, req.Date.Format(time.RFC3339Nano))
}

func genRequests(ctx context.Context, class string, intv time.Duration) chan *Request {
	outC := make(chan *Request)
	go func(ctx context.Context) {
		defer close(outC)

		seq := 0
		for {

			req := &Request{
				Class: class,
				Seq:   seq,
				Date:  time.Now(),
			}
			seq++

			select {
			case <-ctx.Done():
				return
			case outC <- req:
				time.Sleep(intv)
				continue
			}
		}
	}(ctx)

	return outC
}

func consumeRequestsStream(ctx context.Context, rateLimiter pkgratelimit.RateLimiter, reqs <-chan *Request) {
	inC, outC, errCh := rateLimiter.GetIO(ctx)
	go func() {
		defer close(inC)
		for req := range reqs {
			inC <- req
		}
	}()

	for {
		select {
		case err, ok := <-errCh:
			if ok && err != nil {
				log.Printf("error from req class A: %v", err)
			}
			return
		case req, ok := <-outC:
			if !ok {
				log.Printf("output channel from req class A closed")
				return
			}
			log.Printf("%s", req.(*Request).String())
		}
	}
}

func main() {
	refreshIntv, _ := time.ParseDuration("5s")
	numTokensPerPeriod := 10
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ratelimitPool := &pkgratelimit.MemoryBasedRateLimitPool{
		RefreshIntv:     refreshIntv,
		NumTokensPerKey: numTokensPerPeriod,
	}
	ratelimitPool.Run(ctx)

	// since all requests yield the same key, so this is considered as a globally shared rate limiter
	globalSharedRL := &pkgratelimit.MemoryBasedRateLimiter{
		Pool: ratelimitPool,
		GetKey: func(ctx context.Context, obj interface{}) (string, error) {
			return "", nil
		},
	}

	go consumeRequestsStream(ctx, globalSharedRL, genRequests(ctx, "A", 100*time.Millisecond))
	log.Printf("Started consuming requests stream for class A")

	go consumeRequestsStream(ctx, globalSharedRL, genRequests(ctx, "B", 100*time.Millisecond))
	log.Printf("Started consuming requests stream for class B")

	go consumeRequestsStream(ctx, globalSharedRL, genRequests(ctx, "C", 100*time.Millisecond))
	log.Printf("Started consuming requests stream for class C")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	log.Printf("Got signal %s, shutting down ...", sig.String())
}
