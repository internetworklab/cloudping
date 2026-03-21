package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/netip"
	"time"
)

func main() {
	// Define the DoT server details
	address := "[2606:4700:4700::1001]:853"
	tcpaddr := net.TCPAddrFromAddrPort(netip.MustParseAddrPort(address))
	serverName := "one.one.one.one"

	// Create a custom resolver
	resolver := &net.Resolver{
		PreferGo: true, // Crucial: ensures the Go-native resolver is used
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			dialer := &tls.Dialer{
				Config: &tls.Config{
					ServerName: serverName,
					MinVersion: tls.VersionTLS12,
				},
			}
			log.Printf("Dialing %s", tcpaddr.String())
			return dialer.DialContext(ctx, "tcp", tcpaddr.String())
		},
	}

	// Use the resolver to look up a host
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ips, err := resolver.LookupHost(ctx, "example.com")
	if err != nil {
		log.Fatalf("Lookup failed: %v", err)
	}

	for _, ip := range ips {
		fmt.Printf("example.com IN A %s\n", ip)
	}
}
