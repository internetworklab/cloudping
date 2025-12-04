package main

import (
	"context"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	pkghub "example.com/rbmq-demo/pkg/hub"
	pkgratelimit "example.com/rbmq-demo/pkg/throttle"
)

func main() {
	throttleConfig := pkgratelimit.TokenBasedThrottleConfig{
		RefreshInterval:       1 * time.Second,
		TokenQuotaPerInterval: 3,
	}
	smootherConfig := pkgratelimit.BurstSmoother{
		LeastSampleInterval: 10 * time.Millisecond,
	}
	mimoScheduler := pkgratelimit.NewMIMOScheduler(throttleConfig, smootherConfig)

	icmpHub := pkghub.NewICMPTransceiveHub(&pkghub.ICMPTransceiveHubConfig{
		MIMOScheduler: mimoScheduler,
	})
	ctx := context.TODO()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mimoScheduler.Run(ctx)

	icmpHub.Run(ctx)

	hubProxy1 := icmpHub.GetProxy()

	icmpId := rand.Intn(0xffff)
	log.Printf("ICMP ID: %d", icmpId)

	// fake death ping generator
	go func() {
		defer log.Println("Fake pings generator closed")
		defer hubProxy1.Close()

		log.Println("[DBG] fake pings generator started")

		writeCh := hubProxy1.GetWriter()

		// try generating ping requests at death speed
		lastSeq := 0

		for {
			mockPacket := pkghub.TestPacket{
				Host: "www.example.com",
				Type: pkghub.TestPacketTypePing,
				Seq:  lastSeq,
				Id:   icmpId,
			}
			select {
			case <-ctx.Done():
				return
			case writeCh <- mockPacket:
				lastSeq++
				log.Printf("Sent ping request: %+v", mockPacket)
			}
		}
	}()

	// fake death pong receiver
	go func() {
		defer log.Println("Fake pongs receiver closed")
		log.Println("[DBG] fake pongs receiver started")

		readCh := hubProxy1.GetReader()
		for pong := range readCh {
			if pongPkt, ok := pong.(pkghub.TestPacket); ok && pongPkt.Type == pkghub.TestPacketTypePong {
				log.Printf("Received pong: %+v", pongPkt)
			}
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigs
	log.Printf("Received signal: %v, exiting...", sig)
}
