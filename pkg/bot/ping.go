package bot

import (
	"context"
	"fmt"
)

// Bot text display models
// Ping event data
type PingEvent struct {
	Seq          int
	RTTMs        int
	Peer         string
	PeerRDNS     string
	IPPacketSize int
	Timeout      bool
	Err          string
}

type PingEventsProvider interface {
	GetEventsByLocationCodeAndDestination(ctx context.Context, locationCode string, destination string) <-chan PingEvent
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
		return fmt.Sprintf("%d bytes from %s (%s): icmp_seq=%d ttl=64 time=%d ms",
			e.IPPacketSize, e.Peer, e.PeerRDNS, e.Seq, e.RTTMs)
	}
	return fmt.Sprintf("%d bytes from %s: icmp_seq=%d ttl=64 time=%d ms",
		e.IPPacketSize, e.Peer, e.Seq, e.RTTMs)
}
