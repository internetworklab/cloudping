package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	pkgpinger "example.com/rbmq-demo/pkg/pinger"
	pkgtcping "example.com/rbmq-demo/pkg/tcping"
)

var (
	hostport     = flag.String("hostport", "127.0.0.1:80", "host:port to ping")
	intvMs       = flag.Int("intvMs", 1000, "interval between pings in milliseconds")
	pktTimeoutMs = flag.Int("pktTimeoutMs", 3000, "packet timeout in milliseconds")
	inetPref     = flag.String("inetPref", "ip", "ip family preference: ip, ipv4, or ipv6")
	count        = flag.Int("count", -1, "number of pings to send")
)

func init() {
	flag.Parse()
}

func main() {
	pingRequest := &pkgpinger.SimplePingRequest{
		Destination:            *hostport,
		IntvMilliseconds:       *intvMs,
		PktTimeoutMilliseconds: *pktTimeoutMs,
	}
	if *count > 0 {
		pingRequest.TotalPkts = count
	}
	if *inetPref != "ip" {
		itstrue := true
		if *inetPref == "ipv4" {
			pingRequest.PreferV4 = &itstrue
		} else if *inetPref == "ipv6" {
			pingRequest.PreferV6 = &itstrue
		}
	}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pinger := &pkgpinger.TCPSYNPinger{
		PingRequest: pingRequest,
	}
	go func() {
		log.Printf("start listening for pinger events")
		defer log.Printf("stop listening for pinger events")

		for ev := range pinger.Ping(ctx) {
			if ev.Error != nil {
				log.Fatalf("error: %v", ev.Error)
			}
			if ev.Data != nil {
				if tcpTrackEv, ok := ev.Data.(pkgtcping.TrackerEvent); ok {
					switch tcpTrackEv.Type {
					case pkgtcping.TrackerEVTimeout:
						log.Printf("timeout: seq=%v", tcpTrackEv.Entry.Value.Seq)
					case pkgtcping.TrackerEVReceived:
						from := net.JoinHostPort(tcpTrackEv.Entry.Value.SrcIP.String(), strconv.Itoa(tcpTrackEv.Entry.Value.SrcPort))
						to := net.JoinHostPort(tcpTrackEv.Entry.Value.Request.DstIP.String(), strconv.Itoa(tcpTrackEv.Entry.Value.Request.DstPort))
						log.Printf("received: seq=%v, rtt=%v, ttl=%v, %s <- %s", tcpTrackEv.Entry.Value.Seq, tcpTrackEv.Entry.Value.RTT, tcpTrackEv.Entry.Value.ReceivedPkt.TTL, from, to)
					}
				}
			}
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigs
	log.Printf("received signal: %s", sig.String())
}
