package bot

import (
	"context"

	"net"

	pkgtui "github.com/internetworklab/cloudping/pkg/tui"
)

type PingRequestDescriptor struct {
	Sources      []string
	Destinations []string
	PreferV4     bool
	PreferV6     bool
	Traceroute   bool
	Count        int
}

type PingEventsProvider interface {
	GetEvents(ctx context.Context, requst *PingRequestDescriptor) <-chan pkgtui.PingEvent
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
