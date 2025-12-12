package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	pkgraw "example.com/rbmq-demo/pkg/raw"
)

var host = flag.String("host", "www.google.com", "host to trace")
var preferV4 = flag.Bool("prefer-v4", false, "prefer IPv4")
var preferV6 = flag.Bool("prefer-v6", false, "prefer IPv6")

func init() {
	flag.Parse()
}

func selectDstIP(ctx context.Context, resolver *net.Resolver, host string, preferV4 *bool, preferV6 *bool) (*net.IPAddr, error) {
	familyPrefer := "ip"
	if preferV6 != nil && *preferV6 {
		familyPrefer = "ip6"
	} else if preferV4 != nil && *preferV4 {
		familyPrefer = "ip4"
	}
	ips, err := resolver.LookupIP(ctx, familyPrefer, host)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup IP: %v", err)
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no IP found for host: %s", host)
	}
	dst := net.IPAddr{IP: ips[0]}
	return &dst, nil
}

func main() {
	ctx := context.TODO()
	ctx, cancel := context.WithCancel(ctx)

	pktTimeout := 3 * time.Second
	pktInterval := 1 * time.Second
	buffRedundancyFactor := 2
	trackerConfig := &pkgraw.ICMPTrackerConfig{
		PacketTimeout:                 pktTimeout,
		TimeoutChannelEventBufferSize: buffRedundancyFactor * int(pktTimeout.Seconds()/math.Max(1, pktInterval.Seconds())),
	}
	tracker, err := pkgraw.NewICMPTracker(trackerConfig)
	if err != nil {
		log.Fatalf("failed to create ICMP tracker: %v", err)
	}
	tracker.Run(ctx)

	dstPtr, err := selectDstIP(context.TODO(), net.DefaultResolver, *host, preferV4, preferV6)
	if err != nil {
		log.Fatalf("failed to select destination IP for host %s: %v", *host, err)
	}
	if dstPtr == nil {
		log.Fatalf("no destination IP found for host: %s", *host)
	}
	dst := *dstPtr

	var transceiver pkgraw.GeneralICMPTransceiver
	idToUse := rand.Intn(0x10000)
	log.Printf("using ID: %v", idToUse)

	if dst.IP.To4() != nil {
		icmp4tr, err := pkgraw.NewICMP4Transceiver(pkgraw.ICMP4TransceiverConfig{
			ID: idToUse,
		})
		if err != nil {
			log.Fatalf("failed to create ICMP4 transceiver: %v", err)
		}
		if err := icmp4tr.Run(ctx); err != nil {
			log.Fatalf("failed to run ICMP4 transceiver: %v", err)
		}
		defer cancel()
		transceiver = icmp4tr
	} else {
		icmp6tr, err := pkgraw.NewICMP6Transceiver(pkgraw.ICMP6TransceiverConfig{
			ID: idToUse,
		})
		if err != nil {
			log.Fatalf("failed to create ICMP6 transceiver: %v", err)
		}
		if err := icmp6tr.Run(ctx); err != nil {
			log.Fatalf("failed to run ICMP6 transceiver: %v", err)
		}
		defer cancel()
		transceiver = icmp6tr
	}

	pingRequests := []pkgraw.ICMPSendRequest{
		{Seq: 1, TTL: 64, Dst: dst},
		{Seq: 2, TTL: 64, Dst: dst},
		{Seq: 3, TTL: 64, Dst: dst},
	}

	go func() {
		log.Printf("Started listening for ICMPTracker events")
		for ev := range tracker.RecvEvC {
			evJsonB, _ := json.Marshal(ev)
			evJson := string(evJsonB)
			log.Printf("ICMPReply: %s, %s", evJson, time.Now().Format(time.RFC3339Nano))
		}
	}()

	go func() {
		receiverCh := transceiver.GetReceiver()
		for {
			subCh := make(chan pkgraw.ICMPReceiveReply)
			select {
			case <-ctx.Done():
				return
			case receiverCh <- subCh:
				reply := <-subCh
				tracker.MarkReceived(reply.Seq)
			}
		}
	}()

	go func() {
		senderCh := transceiver.GetSender()

		for _, pingRequest := range pingRequests {
			senderCh <- pingRequest
			tracker.MarkSent(pingRequest.Seq)
			<-time.After(pktInterval)
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigs
	log.Printf("Received signal: %v, exiting...", sig.String())
}
