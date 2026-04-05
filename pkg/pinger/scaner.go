package pinger

import (
	"context"
	"fmt"
	"log"
	"math"
	"net"
	"sync"
	"time"

	pkgmyprom "github.com/internetworklab/cloudping/pkg/myprom"
	pkgratelimit "github.com/internetworklab/cloudping/pkg/ratelimit"
	pkgraw "github.com/internetworklab/cloudping/pkg/raw"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
	"github.com/prometheus/client_golang/prometheus"
)

type SimpleBlockScanner struct {
	PingRequest  *SimplePingRequest
	OnSent       pkgraw.ICMPTransceiverHook
	OnReceived   pkgraw.ICMPTransceiverHook
	RateLimiter  pkgratelimit.RateLimiter
	CommonLabels *prometheus.Labels
	CounterStore *pkgmyprom.CounterStore
}

type IPProbeEvent struct {
	RTT  int64
	Peer string
}

func (sp *SimpleBlockScanner) withRateLimiter(ctx context.Context, unthrottled <-chan net.IP, rateLimiter pkgratelimit.RateLimiter) <-chan net.IP {
	rlIn, rlOut := rateLimiter.GetIO(ctx)

	throttled := make(chan net.IP)

	// Forward rate-limited items from the rate limiter output to the throttled channel.
	go func() {
		defer close(throttled)
		for item := range rlOut {
			throttled <- item.(net.IP)
		}
	}()

	// Feed items from the unthrottled channel into the rate limiter.
	go func() {
		for ip := range unthrottled {
			rlIn <- ip
		}
	}()

	return throttled
}

func (sp *SimpleBlockScanner) Ping(ctx context.Context) <-chan PingEvent {
	outputEVChan := make(chan PingEvent)

	go func() {
		defer close(outputEVChan)

		var wg sync.WaitGroup
		for _, ipCidrStr := range sp.PingRequest.Targets {
			ch := sp.pingCIDR(ctx, ipCidrStr)
			wg.Add(1)
			go func(ch <-chan PingEvent) {
				defer wg.Done()
				for ev := range ch {
					outputEVChan <- ev
				}
			}(ch)
		}
		wg.Wait()
	}()

	return outputEVChan
}

func (sp *SimpleBlockScanner) pingCIDR(ctx context.Context, ipCidrStr string) <-chan PingEvent {
	ch := make(chan PingEvent)

	commonLabels := sp.CommonLabels
	counterStore := sp.CounterStore

	go func() {
		var goroutineWG sync.WaitGroup   // goroutine lifecycle
		var inFlightPktWg sync.WaitGroup // in-flight pings awaiting pong or timeout
		defer goroutineWG.Wait()         // 3. wait for all goroutines to exit
		defer close(ch)                  // 2. close output channel
		defer func() {                   // 1. wait for in-flight pings to settle
			// Context-aware wait: when ctx is cancelled the tracker stops
			// producing events, so we must not block on inFlightPktWg forever.
			done := make(chan struct{})
			go func() {
				inFlightPktWg.Wait()
				close(done)
			}()
			select {
			case <-done:
			case <-ctx.Done():
			}
		}()

		pingRequest := sp.PingRequest

		pkgTimeout := time.Duration(pingRequest.PktTimeoutMilliseconds) * time.Millisecond
		pkgInterval := time.Duration(pingRequest.IntvMilliseconds) * time.Millisecond
		trackerConfig := &pkgraw.ICMPTrackerConfig{
			PacketTimeout:                 pkgTimeout,
			TimeoutChannelEventBufferSize: 2 * int(pkgTimeout.Seconds()/math.Max(1, pkgInterval.Seconds())),
		}
		tracker, err := pkgraw.NewICMPTracker(trackerConfig)
		if err != nil {
			log.Fatalf("failed to create ICMP tracker: %v", err)
		}
		tracker.Run(ctx)

		nwAddr, ipNet, err := net.ParseCIDR(ipCidrStr)
		if err != nil {
			ch <- PingEvent{Error: fmt.Errorf("invalid cidr %w: %s", err, ipCidrStr)}
			return
		}

		if ipNet == nil {
			ch <- PingEvent{Error: fmt.Errorf("invalid cidr: %s", ipCidrStr)}
			return
		}

		if nwAddr.IsLinkLocalUnicast() || nwAddr.IsLinkLocalMulticast() {
			ch <- PingEvent{Error: fmt.Errorf("link-local addresses are not supported: %s", ipCidrStr)}
			return
		}

		ones, bits := ipNet.Mask.Size()
		hostBits := bits - ones
		if hostBits > 32 {
			ch <- PingEvent{Error: fmt.Errorf("too many host bits (%d) in cidr: %s (maximum 32)", hostBits, ipCidrStr)}
			return
		}

		useUDP := pingRequest.L4PacketType != nil && *pingRequest.L4PacketType == "udp"
		udpPort := pingRequest.UDPDstPort

		var transceiver pkgraw.GeneralICMPTransceiver
		if nwAddr.To4() != nil {
			icmp4tr, err := pkgraw.NewICMP4Transceiver(pkgraw.ICMP4TransceiverConfig{
				UDPBasePort: udpPort,
				UseUDP:      useUDP,
				OnSent:      sp.OnSent,
				OnReceived:  sp.OnReceived,
			})
			if err != nil {
				log.Fatalf("failed to create ICMP4 transceiver: %v", err)
			}
			transceiver = icmp4tr
		} else {
			icmp6tr, err := pkgraw.NewICMP6Transceiver(pkgraw.ICMP6TransceiverConfig{
				UseUDP:      useUDP,
				UDPBasePort: udpPort,
				OnSent:      sp.OnSent,
				OnReceived:  sp.OnReceived,
			})
			if err != nil {
				log.Fatalf("failed to create ICMP6 transceiver: %v", err)
			}
			transceiver = icmp6tr
		}

		// GetIO creates its own goroutine for send/receive — no need to call Run().
		inC, outC, errC := transceiver.GetIO(ctx)

		// Receiving goroutine: drain ICMP replies from the transceiver and feed
		// them to the tracker.  This runs independently so that outC is always
		// drained, which keeps the transceiver goroutine unblocked and able to
		// accept our sends on inC.
		goroutineWG.Add(1)
		go func() {
			defer goroutineWG.Done()

			for {
				select {
				case <-ctx.Done():
					return
				case rxPkt, ok := <-outC:
					if !ok {
						return
					}
					if err := tracker.MarkReceived(rxPkt.Seq, rxPkt); err != nil {
						log.Printf("failed to mark received for seq %d: %v", rxPkt.Seq, err)
						return
					}
					counterStore.LogPktReceive(commonLabels)
				case rxErr, ok := <-errC:
					if ok && rxErr != nil {
						log.Printf("transceiver error: %v", rxErr)
					}
					return
				}
			}
		}()

		// Tracker event drain goroutine: consume events from the tracker and
		// forward them as PingEvents.  This runs independently so RecvEvC is
		// always drained — preventing the deadlock cascade where RecvEvC fills
		// up → tracker blocks → MarkReceived blocks → receiver goroutine blocks
		// → transceiver blocks on outC → nobody reads inC.
		goroutineWG.Add(1)
		go func() {
			defer goroutineWG.Done()
			for ev := range tracker.RecvEvC {
				select {
				case ch <- PingEvent{Data: sp.newProbeEvent(&ev)}:
					inFlightPktWg.Done()
				case <-ctx.Done():
					// Context cancelled — stop draining. The defer in the
					// parent goroutine handles the context-aware wait on
					// inFlightPktWg so it won't deadlock.
					return
				}
			}
		}()

		// Main loop: iterate addresses and send pings.
		addressesCh := pkgutils.GetMemberAddresses32(ctx, *ipNet)
		if sp.RateLimiter != nil {
			addressesCh = sp.withRateLimiter(ctx, addressesCh, sp.RateLimiter)
		}
		numPktsSent := 0

		for {
			select {
			case <-ctx.Done():
				return
			case dstIP, ok := <-addressesCh:
				if !ok {
					// All addresses have been sent.  The deferred in-flight
					// wait will ensure every outstanding ping settles (via
					// pong or timeout) before ch is closed, unless the
					// context is cancelled first.
					return
				}

				dstIPAddr := net.IPAddr{IP: dstIP}
				nextTTL := pingRequest.TTL.Get()
				pingRequest.TTL.Forward()

				seq := numPktsSent + 1
				req := pkgraw.ICMPSendRequest{
					Seq: seq,
					TTL: nextTTL,
					Dst: dstIPAddr,
				}

				// MarkSent first, then actually send it.
				// If we send before marking, and the reply arrives too early,
				// the reply packet can't find the corresponding sent entry.
				if err := tracker.MarkSent(req.Seq, req.TTL, &dstIPAddr); err != nil {
					log.Printf("failed to mark sent for %s: %v", dstIPAddr, err)
					return
				}
				inFlightPktWg.Add(1)

				select {
				case inC <- req:
				case <-ctx.Done():
					// MarkSent succeeded so the tracker has a pending entry,
					// but we never sent the actual packet.  The tracker will
					// eventually emit a timeout event for it — unless its Run
					// loop also exits due to this same context cancellation.
					// In either case we are on the ctx-cancelled path; the
					// deferred in-flight wait selects ctx.Done() so there is
					// no deadlock.
					return
				}

				numPktsSent++
				counterStore.LogPktSent(commonLabels)
				<-time.After(pkgInterval)
			}
		}
	}()

	return ch
}

func (sp *SimpleBlockScanner) newProbeEvent(ev *pkgraw.ICMPTrackerEntry) *IPProbeEvent {
	outEv := &IPProbeEvent{
		RTT: -1,
	}
	if len(ev.ReceivedAt) > 0 {
		tRx := ev.ReceivedAt[len(ev.ReceivedAt)-1]
		tTx := ev.SentAt
		outEv.RTT = tRx.Sub(tTx).Milliseconds()
	}
	if ev.OriginDstAddr != nil {
		outEv.Peer = ev.OriginDstAddr.String()
	}
	return outEv
}
