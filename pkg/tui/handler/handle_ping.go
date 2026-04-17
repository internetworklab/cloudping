package handler

import (
	"fmt"
	"net/http"
	"slices"

	pkgtui "github.com/internetworklab/cloudping/pkg/tui"
	pkgtuirenderer "github.com/internetworklab/cloudping/pkg/tui/renderer"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

type PingCMD struct {
	From           []string                             `name:"from" short:"s" help:"Set the source node(s), could be repeated multiple times"`
	Destinations   []string                             `arg:"" name:"destination" help:"Destination(s) to ping."`
	PreferV4       bool                                 `name:"prefer-v4" short:"4" help:"Prefer IPv4 when pinging domain(s)"`
	PreferV6       bool                                 `name:"prefer-v6" short:"6" help:"Prefer IPv6 when pinging domain(s)"`
	Orientation    pkgtuirenderer.PingMatrixOrientation `name:"table-orientation" help:"Specify the orientation of the output ping matrix" default:"src_tgt"`
	CustomResolver string                               `name:"custom-resolver" help:"To override the system's default resolver to use" default:""`
	ICMP           bool                                 `name:"icmp" help:"Send ICMP packets to probe target host(s)" default:"true"`
	UDP            bool                                 `name:"udp" help:"Send UDP packets to probe target host(s)" default:"false"`
	TCP            bool                                 `name:"tcp" help:"Send TCP SYN packets to probe target host(s)" default:"false"`
}

func (cmd *PingCMD) Run(globalCtx *CLICtx) error {
	w := globalCtx.ResponseWriter
	r := globalCtx.Request
	locsP := globalCtx.LocationsProvider
	pingEVsP := globalCtx.PingEventsProvider

	ctx := r.Context()
	allLocs, err := locsP.GetAllLocations(ctx)
	if err != nil {
		writeErrorResponse(w, "Can't list all locations", http.StatusInternalServerError)
		return nil
	}

	if len(allLocs) == 0 {
		writeErrorResponse(w, "No locations available", http.StatusBadRequest)
		return nil
	}

	sortedFrom := make([]string, 0)
	if len(cmd.From) > 0 {
		sortedFrom = append(sortedFrom, cmd.From...)
	} else {
		for _, loc := range allLocs {
			sortedFrom = append(sortedFrom, loc.Id)
		}
	}
	slices.Sort(sortedFrom)
	if pkgutils.CheckSortedStringsDup(sortedFrom) {
		sortedFrom = pkgutils.Dedup(sortedFrom)
	}

	sortedDestinations := make([]string, 0)
	sortedDestinations = append(sortedDestinations, cmd.Destinations...)
	slices.Sort(sortedDestinations)
	if pkgutils.CheckSortedStringsDup(sortedDestinations) {
		writeErrorResponse(w, "Duplicated Destinations", http.StatusBadRequest)
		return nil
	}

	for _, from := range sortedFrom {
		if idx := slices.IndexFunc(allLocs, func(loc pkgtui.LocationDescriptor) bool { return loc.Id == from }); idx == -1 {
			writeErrorResponse(w, "From location not found: "+from, http.StatusBadRequest)
			return nil
		}
	}

	evsChan := pingEVsP.GetEvents(ctx, &pkgtui.PingRequestDescriptor{
		Sources:      sortedFrom,
		Destinations: sortedDestinations,
		Traceroute:   false,
		Count:        1,
		PreferV4:     cmd.PreferV4,
		PreferV6:     cmd.PreferV4,
		Resolver:     cmd.CustomResolver,
		ICMP:         cmd.ICMP,
		UDP:          cmd.UDP,
		TCP:          cmd.TCP,
	})

	mat, err := pkgtuirenderer.NewPingMatrix(
		sortedFrom,
		sortedDestinations,
		cmd.Orientation,
	)
	if err != nil {
		writeErrorResponse(w, fmt.Sprintf("Failed to initialize ping matrix: %s", err.Error()), http.StatusBadRequest)
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			// the client can't wait for it to be done
			return nil
		case ev, ok := <-evsChan:
			if !ok {
				matRenderer := &pkgtuirenderer.PingMatrixRenderer{}
				outputTable := matRenderer.Render(mat)
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprint(w, outputTable.GetReadableHTMLTable())
				return nil
			}
			if v := ev.RttMsFlt; v != nil {
				mat.WriteSample(ev.From, ev.To, *v)
			} else {
				mat.WriteSample(ev.From, ev.To, float64(ev.RTTMs))
			}
		}
	}
}
