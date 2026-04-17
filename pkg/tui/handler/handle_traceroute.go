package handler

import (
	"fmt"
	"net/http"
	"slices"

	pkgtui "github.com/internetworklab/cloudping/pkg/tui"
	pkgtuitraceroute "github.com/internetworklab/cloudping/pkg/tui/traceroute"
)

type TracerouteCMD struct {
	From           string `name:"from" short:"s" help:"Set the source node, could be repeated multiple times"`
	Destination    string `arg:"" name:"destination" help:"Destination to traceroute."`
	PreferV4       bool   `name:"prefer-v4" short:"4" help:"Prefer IPv4 when tracing domain(s)"`
	PreferV6       bool   `name:"prefer-v6" short:"6" help:"Prefer IPv6 when tracing domain(s)"`
	Count          int    `name:"count" short:"c" help:"Number of packets to send" default:"24"`
	CustomResolver string `name:"custom-resolver" help:"To override the system's default resolver to use" default:""`
	ICMP           bool   `name:"icmp" help:"Send ICMP packets to probe target host(s)" default:"true"`
	UDP            bool   `name:"udp" help:"Send UDP packets to probe target host(s)" default:"false"`
}

func (cmd *TracerouteCMD) Run(globalCtx *CLICtx) error {
	w := globalCtx.ResponseWriter
	r := globalCtx.Request
	locsP := globalCtx.LocationsProvider
	pingEVsP := globalCtx.PingEventsProvider

	if cmd.From == "" {
		writeErrorResponse(w, "From is empty", http.StatusBadRequest)
		return nil
	}

	ctx := r.Context()
	allLocs, err := locsP.GetAllLocations(ctx)
	if err != nil {
		writeErrorResponse(w, "Can't list all locations", http.StatusInternalServerError)
		return nil
	}

	if idx := slices.IndexFunc(allLocs, func(loc pkgtui.LocationDescriptor) bool { return loc.Id == cmd.From }); idx == -1 {
		writeErrorResponse(w, "From location not found: "+cmd.From, http.StatusBadRequest)
		return nil
	}

	statsBuilder := pkgtuitraceroute.NewTraceStatsBuilder()

	evsChan := pingEVsP.GetEvents(ctx, &pkgtui.PingRequestDescriptor{
		Sources:      []string{cmd.From},
		Destinations: []string{cmd.Destination},
		Traceroute:   true,
		Count:        cmd.Count,
		PreferV4:     cmd.PreferV4,
		PreferV6:     cmd.PreferV6,
		Resolver:     cmd.CustomResolver,
		ICMP:         cmd.ICMP,
		UDP:          cmd.UDP,
	})

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-evsChan:
			if !ok {
				outputTable := statsBuilder.ToTable()
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprint(w, outputTable.GetReadableHTMLTable())
				return nil
			}
			statsBuilder.WriteEvent(ev)
		}
	}
}
