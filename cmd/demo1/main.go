package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/internetworklab/cloudping/pkg/pinger"
)

func main() {
	nwCIDRs := []string{
		"172.23.0.0/24",
		"172.22.0.0/24",
		"172.21.0.0/24",
		"172.20.0.0/24",
	}
	for _, nwCIDR := range nwCIDRs {
		_, ipnet, _ := net.ParseCIDR(nwCIDR)
		log.Println("Started to scan", nwCIDR)
		testNetwork(*ipnet)
	}
}

func testNetwork(nwCIDR net.IPNet) {

	ctx := context.Background()

	scanner := &pinger.SimpleBlockScanner{
		PingRequest: &pinger.SimplePingRequest{
			Targets:                []string{nwCIDR.String()},
			IntvMilliseconds:       100,
			PktTimeoutMilliseconds: 3000,
			TTL:                    &pinger.RangeTTL{TTLs: []int{64}},
		},
	}

	eventCh := scanner.Ping(ctx)

	responded := 0
	timedOut := 0
	for ev := range eventCh {
		if ev.Error != nil {
			log.Printf("Error: %v", ev.Error)
			continue
		}
		if probe, ok := ev.Data.(*pinger.IPProbeEvent); ok {
			if probe.RTT == -1 {
				fmt.Printf("  TIMEOUT  %s\n", probe.Peer)
				timedOut++
			} else {
				fmt.Printf("  UP       %s  RTT=%dms\n", probe.Peer, probe.RTT)
				responded++
			}
		}
	}

	fmt.Printf("\nScan complete. %d host(s) reached, %d timed out in %s.\n", responded, timedOut, nwCIDR)
}
