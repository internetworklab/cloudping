package tui

import (
	"context"
	"fmt"
	"net"
	"time"

	pkgipinfo "github.com/internetworklab/cloudping/pkg/ipinfo"
)

// Text based UI

const DefaultMaxColWidth int = 24
const DefaultColGap int = 2
const DefaultRowGap int = 0

// Bot text display models
// Ping event data
type PingEvent struct {
	// Error during the generation of ping/icmp events, if any
	Err string

	// could be empty, useful in many-to-many pinging
	From string

	// could be empty, useful in many-to-many pinging
	To string

	Seq          int
	RTTMs        int
	RttMsFlt     *float64
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

	rttStr := fmt.Sprintf("%d", e.RTTMs)
	if e.RttMsFlt != nil {
		rttStr = fmt.Sprintf("%.2f", *e.RttMsFlt)
	}

	// Handle normal events
	if e.PeerRDNS != "" {
		return fmt.Sprintf("%d bytes from %s (%s): icmp_seq=%d ttl=%d time=%s ms",
			e.IPPacketSize, e.Peer, e.PeerRDNS, e.Seq, e.TTL, rttStr)
	}
	return fmt.Sprintf("%d bytes from %s: icmp_seq=%d ttl=%d time=%s ms",
		e.IPPacketSize, e.Peer, e.Seq, e.TTL, rttStr)
}

type PingRequestDescriptor struct {
	Sources      []string
	Destinations []string
	PreferV4     bool
	PreferV6     bool
	Traceroute   bool
	Count        int
	Resolver     string
	ICMP         bool
	UDP          bool
	TCP          bool
	PingIntv     time.Duration
}

type LocationDescriptor struct {
	Id                string
	Label             string
	Alpha2CountryCode string
	CityIATACode      string

	// This field is optional and implementation-specific
	ExtendedAttributes map[string]string
}

type PingEventsProvider interface {
	GetEvents(ctx context.Context, requst *PingRequestDescriptor) <-chan PingEvent
	GetAllLocations(ctx context.Context) ([]LocationDescriptor, error)
}

type LocationsProvider interface {
	GetAllLocations(ctx context.Context) ([]LocationDescriptor, error)
}

type ProbeRequestDescriptor struct {
	FromNodeId string
	TargetCIDR net.IPNet
}

type ProbeEvent struct {
	Err   error
	IP    net.IP
	RTTMs int
}

type ProbeEventsProvider interface {
	GetProbeEvents(ctx context.Context, request ProbeRequestDescriptor) <-chan ProbeEvent
}
