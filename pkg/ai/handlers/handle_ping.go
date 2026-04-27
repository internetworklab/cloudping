package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"slices"

	pkgutils "github.com/internetworklab/cloudping/pkg/utils"

	pkgtui "github.com/internetworklab/cloudping/pkg/tui"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type PingSample struct {
	From  string   `json:"from" jsonschema:"The name of the ping provider, that is, the source location."`
	Dest  string   `json:"dest" jsonschema:"The destination of the pinging host, could be a domain, an IP address or an IPv4 address."`
	RTTMs *float64 `json:"rttMs" jsonschema:"RTT in milliseconds, could be nil if the ping was timed out"`
	Seq   int      `json:"seq" jsonschema:"The Seq of the sent ICMP packet"`
}

type PingResult struct {
	Error string       `json:"error,omitempty"`
	Data  []PingSample `json:"data" jsonschema:"Ping samples data"`
}

func floatPtr(v float64) *float64 {
	return &v
}

type PingHandler struct {
	ToolName           string
	ToolDescription    string
	LocationsProvider  pkgtui.LocationsProvider
	PingEventsProvider pkgtui.PingEventsProvider
}

// Using the generic AddTool automatically populates the the input and output
// schema of the tool.
type PingArgs struct {
	Destinations []string `json:"destinations" jsonschema:"The list of destination host(s) to ping, each entry could be a domain, ip or ipv6 address."`
	PreferV4     bool     `json:"prefer-ipv4" jsonschema:"Prefer to use IPv4 when there is a chance to resolve domain to address"`
	PreferV6     bool     `json:"prefer-ipv6" jsonschema:"Prefer to use IPv6 when there is a chance to resolve domain to address"`
	From         []string `json:"from" jsonschema:"The name of ping providers to select, or use all available ones if leave it empty."`
}

func (handler *PingHandler) GetName() string {
	if name := handler.ToolName; name != "" {
		return name
	}
	return "ping"
}

func (handler *PingHandler) GetDescription() string {
	if description := handler.ToolDescription; description != "" {
		return description
	}
	return "Ping the destination host to see if it responds and how quick is the respond."
}

func (handler *PingHandler) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        handler.GetName(),
		Description: handler.GetDescription(),
	}
}

func getSortedSourcesAndDests(providedSources []string, providedDests []string, locationProvider pkgtui.LocationsProvider) ([]string, []string, error) {
	allLocs, err := locationProvider.GetAllLocations(context.Background())
	if err != nil {
		return nil, nil, fmt.Errorf("can't list all locations: %w", err)
	}

	if len(allLocs) == 0 {
		return nil, nil, fmt.Errorf("no locations available")
	}

	// Resolve sources: use provided ones or default to all locations
	sortedFrom := make([]string, 0)
	if len(providedSources) > 0 {
		sortedFrom = append(sortedFrom, providedSources...)
	} else {
		for _, loc := range allLocs {
			sortedFrom = append(sortedFrom, loc.Id)
		}
	}
	slices.Sort(sortedFrom)
	if pkgutils.CheckSortedStringsDup(sortedFrom) {
		sortedFrom = pkgutils.Dedup(sortedFrom)
	}

	// Validate destinations
	sortedDests := make([]string, 0, len(providedDests))
	sortedDests = append(sortedDests, providedDests...)
	slices.Sort(sortedDests)
	if pkgutils.CheckSortedStringsDup(sortedDests) {
		return nil, nil, fmt.Errorf("duplicated destinations")
	}

	// Validate that source locations exist
	for _, from := range sortedFrom {
		if idx := slices.IndexFunc(allLocs, func(loc pkgtui.LocationDescriptor) bool { return loc.Id == from }); idx == -1 {
			return nil, nil, fmt.Errorf("from location not found: %s", from)
		}
	}

	return sortedFrom, sortedDests, nil
}

func (handler *PingHandler) HandleToolRequest(ctx context.Context, req *mcp.CallToolRequest, args PingArgs) (*mcp.CallToolResult, PingResult, error) {
	logRequest(ctx, args, handler.GetName())

	sortedFrom, sortedDests, err := getSortedSourcesAndDests(args.From, args.Destinations, handler.LocationsProvider)
	if err != nil {
		return nil, PingResult{}, fmt.Errorf("can not sort out sources and destinations: %w", err)
	}

	// Kick off real ping via PingEventsProvider
	evDataCh := handler.PingEventsProvider.GetEvents(ctx, &pkgtui.PingRequestDescriptor{
		Sources:      sortedFrom,
		Destinations: sortedDests,
		Count:        1,
		ICMP:         true,
		PreferV4:     args.PreferV4,
		PreferV6:     args.PreferV6,
	})

	// Collect ping samples asynchronously and synchronize via channel
	collectCh := make(chan collectResult, 1)
	go handler.collectSamples(evDataCh, collectCh)

	var result PingResult
	select {
	case <-ctx.Done():
		result = PingResult{Error: "request cancelled"}
	case cr := <-collectCh:
		result = PingResult{Data: cr.samples}
		if cr.err != "" {
			result.Error = cr.err
		}
	}

	j, err := json.Marshal(result)
	if err != nil {
		log.Printf("[err] failed to marshal results: %v", err)
		return nil, PingResult{}, err
	}

	mcpToolCallResult := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(j)},
		},
	}

	return mcpToolCallResult, result, nil
}

// collectResult holds the outcome of asynchronously collecting ping events.
type collectResult struct {
	samples []PingSample
	err     string
}

// collectSamples drains the ping event channel, converts each event into a PingSample,
// and sends the aggregated result to doneCh. Intended to be run as a goroutine.
func (handler *PingHandler) collectSamples(evCh <-chan pkgtui.PingEvent, doneCh chan<- collectResult) {
	samples := make([]PingSample, 0)
	var pingErr string
	for ev := range evCh {
		if ev.Err != "" {
			pingErr = ev.Err
			continue
		}
		var rttMs *float64
		if v := ev.RttMsFlt; v != nil {
			rttMs = v
		} else if ev.Timeout {
			rttMs = nil
		} else {
			rttMs = floatPtr(float64(ev.RTTMs))
		}
		samples = append(samples, PingSample{
			From:  ev.From,
			Dest:  ev.To,
			RTTMs: rttMs,
			Seq:   ev.Seq,
		})
	}
	doneCh <- collectResult{samples: samples, err: pingErr}
}

func (handler *PingHandler) RegisterTool(mcpsrv *mcp.Server) {
	mcp.AddTool(mcpsrv, handler.GetTool(), handler.HandleToolRequest)
}

func (handler *PingHandler) getResourceURI() string {
	return "config://ping-providers"
}

func (handler *PingHandler) getResourceMIMEType() string {
	return "application/json"
}

func (handler *PingHandler) getResourceName() string {
	return "PingProviders"
}

func (handler *PingHandler) getResourceDescription() string {
	return `The list of probes that are capable of doing ping to hosts.
To initiate a ping task, the user might specify a list of providers to use,
or leave it empty to use all available providers.`
}

func (handler *PingHandler) HandleResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
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

func (handler *PingHandler) RegisterResource(mcpsrv *mcp.Server) {
	resourceDecl := &mcp.Resource{
		URI:         handler.getResourceURI(),
		Name:        handler.getResourceName(),
		MIMEType:    handler.getResourceMIMEType(),
		Description: handler.getResourceDescription(),
	}

	mcpsrv.AddResource(resourceDecl, handler.HandleResource)
}
