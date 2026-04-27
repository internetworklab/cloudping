package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	pkgtui "github.com/internetworklab/cloudping/pkg/tui"
	pkgtuitraceroute "github.com/internetworklab/cloudping/pkg/tui/traceroute"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type FlatHopGroup struct {
	// `TTL` is from `HopGroup.TTL`
	TTL int `json:"ttl" jsonschema:"TTL is the TTL (or HopLimit) of the origin outgoing packets sent to probe the destination. Hosts in the halfway is expected to send back a TTL-exceeded ICMP error to us."`

	// `Peers` are from `HopGroup.Peers`, while arranging in the order defined by `HopGroup.PeerOrder`
	Peers []*pkgtuitraceroute.PeerStats `json:"peers" jsonschema:"Peers encountered in this hop, in the case of ECMP, peers of different IPs might be encountered in the same hop, the last encountered peer in the hop will be placed at first."`
}

func FlatHopGroupFromHopGroup(hopGroup pkgtuitraceroute.HopGroup) FlatHopGroup {
	flat := FlatHopGroup{
		TTL:   hopGroup.TTL,
		Peers: make([]*pkgtuitraceroute.PeerStats, 0, len(hopGroup.PeerOrder)),
	}
	for _, peerKey := range hopGroup.PeerOrder {
		if peer, ok := hopGroup.Peers[peerKey]; ok {
			flat.Peers = append(flat.Peers, peer)
		}
	}
	return flat
}

type FlatRouteTrace struct {
	// `HopsGroups` are come from `TraceStats.Hops`,
	// while arranging in the order defined by `TraceStats.HopOrder`
	HopGroups []FlatHopGroup `json:"hops" jsonschema:"Trace of the packet, in terms of the hops traversed (i.e. TTLs)."`
}

type RouteTraceResultEntry struct {
	From        string         `json:"from" jsonschema:"The name of the route tracing provider to use."`
	TraceResult FlatRouteTrace `json:"trace_result" jsonschema:"The result of route tracing (traceroute) from this source to the given destination."`
}

type RouteTraceResult struct {
	Error string                           `json:"error,omitempty"`
	Data  map[string]RouteTraceResultEntry `json:"data" jsonschema:"It's a dictionary that maps name of the traceroute provider (key) to the traceroute result struct data (value)."`
}

func FlatRouteTraceFromTraceStats(traceStats pkgtuitraceroute.TraceStats) FlatRouteTrace {
	flat := FlatRouteTrace{
		HopGroups: make([]FlatHopGroup, 0, len(traceStats.HopOrder)),
	}
	for _, ttl := range traceStats.HopOrder {
		if hop, ok := traceStats.Hops[ttl]; ok {
			flat.HopGroups = append(flat.HopGroups, FlatHopGroupFromHopGroup(*hop))
		}
	}
	return flat
}

type TracerouteHandler struct {
	ToolName           string
	ToolDescription    string
	LocationsProvider  pkgtui.LocationsProvider
	PingEventsProvider pkgtui.PingEventsProvider
}

// Using the generic AddTool automatically populates the the input and output
// schema of the tool.
type TracerouteArgs struct {
	IP       string   `json:"ip" jsonschema:"The ip or ipv6 address to trace to."`
	From     []string `json:"from" jsonschema:"The name of route-tracing providers to select, or use all available ones if leave it empty."`
	PreferV4 bool     `json:"prefer-ipv4" jsonschema:"Prefer to use IPv4 when there is a chance to resolve domain to address"`
	PreferV6 bool     `json:"prefer-ipv6" jsonschema:"Prefer to use IPv6 when there is a chance to resolve domain to address"`
}

func (handler *TracerouteHandler) GetName() string {
	if name := handler.ToolName; name != "" {
		return name
	}
	return "traceroute"
}

func (handler *TracerouteHandler) GetDescription() string {
	if description := handler.ToolDescription; description != "" {
		return description
	}
	return "Trace the actual IP onward route path to the specified destination."
}

func (handler *TracerouteHandler) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        handler.GetName(),
		Description: handler.GetDescription(),
	}
}

const defaultPerDestPktsCount = 24

// for ai facing traceroute, we can make it faster,
// otherwise, i wonder if the agent has patience to wait.
const defaultPktIntv = 200 * time.Millisecond

func (handler *TracerouteHandler) HandleToolRequest(ctx context.Context, req *mcp.CallToolRequest, args TracerouteArgs) (*mcp.CallToolResult, RouteTraceResult, error) {
	logRequest(ctx, args, handler.GetName())

	sortedFrom, sortedDests, err := getSortedSourcesAndDests(args.From, []string{args.IP}, handler.LocationsProvider)
	if err != nil {
		return nil, RouteTraceResult{}, fmt.Errorf("can not sort out sources and destinations: %w", err)
	}

	// Kick off real traceroute via PingEventsProvider
	evDataCh := handler.PingEventsProvider.GetEvents(ctx, &pkgtui.PingRequestDescriptor{
		Sources:      sortedFrom,
		Destinations: sortedDests,
		Traceroute:   true,
		Count:        defaultPerDestPktsCount,
		ICMP:         true,
		PingIntv:     defaultPktIntv,
		PreferV4:     args.PreferV4,
		PreferV6:     args.PreferV6,
	})

	// Collect traceroute events asynchronously and synchronize via channel
	collectCh := make(chan tracerouteCollectResult, 1)
	go handler.collectTracerouteEvents(evDataCh, collectCh)

	var result RouteTraceResult
	select {
	case <-ctx.Done():
		result = RouteTraceResult{Error: "request cancelled"}
	case cr := <-collectCh:
		resultMap := make(map[string]RouteTraceResultEntry, len(cr.data))
		for src, sb := range cr.data {
			resultMap[src] = RouteTraceResultEntry{
				From:        src,
				TraceResult: FlatRouteTraceFromTraceStats(*sb.GetTraceStats()),
			}
		}
		result = RouteTraceResult{Data: resultMap}
		if cr.err != "" {
			result.Error = cr.err
		}
	}

	j, err := json.Marshal(result)
	if err != nil {
		log.Printf("[err] failed to marshal results: %v", err)
		return nil, RouteTraceResult{}, err
	}

	mcpToolCallResult := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(j)},
		},
	}

	return mcpToolCallResult, result, nil
}

// tracerouteCollectResult holds the outcome of asynchronously collecting traceroute events.
type tracerouteCollectResult struct {
	data map[string]*pkgtuitraceroute.TraceStatsBuilder
	err  string
}

// collectTracerouteEvents drains the ping event channel, feeds each event into
// a per-source TraceStatsBuilder, and sends the aggregated result to doneCh.
// Intended to be run as a goroutine.
func (handler *TracerouteHandler) collectTracerouteEvents(evCh <-chan pkgtui.PingEvent, doneCh chan<- tracerouteCollectResult) {
	statsBuilders := make(map[string]*pkgtuitraceroute.TraceStatsBuilder)
	var traceErr string
	for ev := range evCh {
		if ev.Err != "" {
			traceErr = ev.Err
			continue
		}
		sb, ok := statsBuilders[ev.From]
		if !ok {
			sb = pkgtuitraceroute.NewTraceStatsBuilder()
			statsBuilders[ev.From] = sb
		}
		sb.WriteEvent(ev)
	}
	doneCh <- tracerouteCollectResult{data: statsBuilders, err: traceErr}
}

func (handler *TracerouteHandler) RegisterTool(mcpsrv *mcp.Server) {
	mcp.AddTool(mcpsrv, handler.GetTool(), handler.HandleToolRequest)
}

func (handler *TracerouteHandler) getResourceURI() string {
	return "config://traceroute-providers"
}

func (handler *TracerouteHandler) getResourceMIMEType() string {
	return "application/json"
}

func (handler *TracerouteHandler) getResourceName() string {
	return "TracerouteProviders"
}

func (handler *TracerouteHandler) getResourceDescription() string {
	return `The list of probes that are capable of providing tracerouting service.
To initiate a traceroute task, the user might specify a list of providers to use,
or leave it empty to use all available providers.`
}

func (handler *TracerouteHandler) HandleResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	locations, err := handler.LocationsProvider.GetAllLocations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list probes: %w", err)
	}

	if len(locations) == 0 {
		return nil, fmt.Errorf("no avaiable probes to use.")
	}

	data, err := json.Marshal(locations)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize probe list: %w", err)
	}

	contents := make([]*mcp.ResourceContents, 0)
	content := &mcp.ResourceContents{
		URI:      handler.getResourceURI(),
		MIMEType: handler.getResourceMIMEType(),
		Text:     string(data),
	}
	contents = append(contents, content)

	return &mcp.ReadResourceResult{Contents: contents}, nil
}

func (handler *TracerouteHandler) RegisterResource(mcpsrv *mcp.Server) {
	resourceDecl := &mcp.Resource{
		URI:         handler.getResourceURI(),
		Name:        handler.getResourceName(),
		MIMEType:    handler.getResourceMIMEType(),
		Description: handler.getResourceDescription(),
	}

	mcpsrv.AddResource(resourceDecl, handler.HandleResource)
}
