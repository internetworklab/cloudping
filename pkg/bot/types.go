package bot

import "context"

// Ping event data
type PingEvent struct {
	Seq          int
	RTTMs        int
	Peer         string
	PeerRDNS     string
	IPPacketSize int
	Timeout      bool
}

type PingEventsProvider interface {
	GetEventsByLocationCodeAndDestination(ctx context.Context, locationCode string, destination string) <-chan PingEvent
	GetAllLocations(ctx context.Context) []LocationDescriptor
}
