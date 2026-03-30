package bot

import (
	"context"
	"fmt"

	pkgipinfo "github.com/internetworklab/cloudping/pkg/ipinfo"
)

// Bot text display models
// Ping event data
type PingEvent struct {
	// Error during the generation of ping/icmp events, if any
	Err string

	Seq          int
	RTTMs        int
	Peer         string
	PeerRDNS     string
	IPPacketSize int
	Timeout      bool

	// ASN of the network where the reply packet is associated with, useful for rendering traceroute
	ASN string

	// Network name of the reply packet sender, useful for rendering traceroute
	ISP string

	CountryAlpha2 string
	City          string
	ExactLocation *pkgipinfo.ExactLocation

	// The TTL of the reply IP packet, usually this is less matter than the OriginTTL
	TTL int

	// The TTL of the original outbound IP packet, when doing traceroute, the value of this field could be vary based on Seq.
	OriginTTL int

	// Useful for rendering traceroute
	LastHop bool
}

type PingRequestDescriptor struct {
	Sources      []string
	Destinations []string
	PreferV4     bool
	PreferV6     bool
	Traceroute   bool
	Count        int
}

type PingEventsProvider interface {
	GetEvents(ctx context.Context, requst *PingRequestDescriptor) <-chan PingEvent
	GetAllLocations(ctx context.Context) ([]LocationDescriptor, error)
}

// String returns a formatted string representation of the ping event
func (e *PingEvent) String() string {
	// Handle error
	if err := e.Err; err != "" {
		return fmt.Sprintf("Error: %s", err)
	}

	// Handle timeout events
	if e.Timeout {
		return fmt.Sprintf("Request timeout for icmp_seq %d", e.Seq)
	}

	// Handle normal events
	if e.PeerRDNS != "" {
		return fmt.Sprintf("%d bytes from %s (%s): icmp_seq=%d ttl=%d time=%d ms",
			e.IPPacketSize, e.Peer, e.PeerRDNS, e.Seq, e.TTL, e.RTTMs)
	}
	return fmt.Sprintf("%d bytes from %s: icmp_seq=%d ttl=%d time=%d ms",
		e.IPPacketSize, e.Peer, e.Seq, e.TTL, e.RTTMs)
}
