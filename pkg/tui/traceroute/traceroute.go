package traceroute

import (
	"fmt"
	"sort"

	pkgtable "github.com/internetworklab/cloudping/pkg/table"
	pkgtui "github.com/internetworklab/cloudping/pkg/tui"
)

// PeerStats holds statistics and events for a single peer (IP address) at a hop
type PeerStats struct {
	Peer          string
	PeerRDNS      string
	ASN           string
	ISP           string
	City          string
	CountryAlpha2 string
	Events        []pkgtui.PingEvent // sorted by seq

	// Calculated stats
	ReceivedCount int
	LossCount     int
	MinRTT        int
	MaxRTT        int
	TotalRTT      int
}

// AvgRTT returns the average RTT for this peer
func (ps *PeerStats) AvgRTT() int {
	if ps.ReceivedCount == 0 {
		return 0
	}
	return ps.TotalRTT / ps.ReceivedCount
}

// HopGroup holds statistics for a single hop (TTL level)
type HopGroup struct {
	TTL       int
	Peers     map[string]*PeerStats // keyed by peer IP address
	PeerOrder []string              // order of peers for consistent output
}

// TraceStats holds the complete traceroute statistics
type TraceStats struct {
	Hops     map[int]*HopGroup // keyed by OriginTTL
	HopOrder []int             // sorted order of TTLs for output
}

// TraceStatsBuilder builds TraceStats from ping events
type TraceStatsBuilder struct {
	stats *TraceStats
}

// NewTraceStatsBuilder creates a new TraceStatsBuilder
func NewTraceStatsBuilder() *TraceStatsBuilder {
	return &TraceStatsBuilder{
		stats: &TraceStats{
			Hops: make(map[int]*HopGroup),
		},
	}
}

// WriteEvent adds a ping event to the traceroute statistics
func (statsBuilder *TraceStatsBuilder) WriteEvent(ev pkgtui.PingEvent) {
	stats := statsBuilder.stats

	// Get or create hop group
	hopTTL := ev.OriginTTL
	if hopTTL <= 0 {
		// Fallback for events without OriginTTL (shouldn't happen in traceroute)
		hopTTL = 1
	}

	hop, exists := stats.Hops[hopTTL]
	if !exists {
		hop = &HopGroup{
			TTL:       hopTTL,
			Peers:     make(map[string]*PeerStats),
			PeerOrder: []string{},
		}
		stats.Hops[hopTTL] = hop
		// Update hop order
		stats.HopOrder = append(stats.HopOrder, hopTTL)
		// Keep hops sorted by TTL
		sort.Ints(stats.HopOrder)
	}

	// Determine peer key (use "*" for timeouts)
	peerKey := ev.Peer
	if ev.Timeout || peerKey == "" {
		peerKey = "*"
	}

	// Get or create peer stats
	peerStats, exists := hop.Peers[peerKey]
	if !exists {
		peerStats = &PeerStats{
			Peer:   ev.Peer,
			Events: []pkgtui.PingEvent{},
			MinRTT: -1, // -1 indicates not set yet
			MaxRTT: -1,
		}
		hop.Peers[peerKey] = peerStats
		hop.PeerOrder = append(hop.PeerOrder, peerKey)
	}

	// Add event to peer stats
	peerStats.Events = append(peerStats.Events, ev)

	// Update peer metadata (use latest non-timeout event's data)
	if !ev.Timeout {
		// Update stats
		peerStats.ReceivedCount++
		peerStats.TotalRTT += ev.RTTMs

		if peerStats.MinRTT == -1 || ev.RTTMs < peerStats.MinRTT {
			peerStats.MinRTT = ev.RTTMs
		}
		if peerStats.MaxRTT == -1 || ev.RTTMs > peerStats.MaxRTT {
			peerStats.MaxRTT = ev.RTTMs
		}

		// Update metadata
		if ev.PeerRDNS != "" {
			peerStats.PeerRDNS = ev.PeerRDNS
		}
		if ev.ASN != "" {
			peerStats.ASN = ev.ASN
		}
		if ev.ISP != "" {
			peerStats.ISP = ev.ISP
		}
		if ev.City != "" {
			peerStats.City = ev.City
		}
		if ev.CountryAlpha2 != "" {
			peerStats.CountryAlpha2 = ev.CountryAlpha2
		}
	} else {
		peerStats.LossCount++
	}

	// Sort events by seq
	sort.Slice(peerStats.Events, func(i, j int) bool {
		return peerStats.Events[i].Seq < peerStats.Events[j].Seq
	})

	// Sort PeerOrder by max seq (descending) - peer with latest packets first
	sort.Slice(hop.PeerOrder, func(i, j int) bool {
		peerI := hop.Peers[hop.PeerOrder[i]]
		peerJ := hop.Peers[hop.PeerOrder[j]]
		if peerI == nil || len(peerI.Events) == 0 {
			return false
		}
		if peerJ == nil || len(peerJ.Events) == 0 {
			return true
		}
		// Max seq is the last event (events are already sorted by seq)
		maxSeqI := peerI.Events[len(peerI.Events)-1].Seq
		maxSeqJ := peerJ.Events[len(peerJ.Events)-1].Seq
		return maxSeqI > maxSeqJ // descending order
	})

	// If this is the last hop, delete all higher hops
	if ev.LastHop {
		newHopOrder := make([]int, 0, hopTTL)
		for _, ttl := range stats.HopOrder {
			if ttl <= hopTTL {
				newHopOrder = append(newHopOrder, ttl)
			} else {
				// Delete the hop from the map
				delete(stats.Hops, ttl)
			}
		}
		stats.HopOrder = newHopOrder
	}
}

// GetTraceStats returns the current traceroute statistics
func (statsBuilder *TraceStatsBuilder) GetTraceStats() *TraceStats {
	return statsBuilder.stats
}

// ToTable converts the traceroute statistics to a Table struct
func (statsBuilder *TraceStatsBuilder) ToTable() *pkgtable.Table {
	stats := statsBuilder.stats
	table := &pkgtable.Table{Rows: []pkgtable.Row{}}

	// Add header rows
	table.Rows = append(table.Rows,
		pkgtable.Row{Cells: []string{"Hop", "Peer", "RTTs (Last Min/Avg/Max)", "Stats (Rx/Tx/Loss)"}},
		pkgtable.Row{Cells: []string{"", "(IP address)", "ASN Network", "City,Country"}},
		pkgtable.Row{Cells: []string{}},
	)

	if len(stats.HopOrder) == 0 {
		return table
	}

	for hopIdx, hopTTL := range stats.HopOrder {
		// Add blank row between hops
		if hopIdx > 0 {
			table.Rows = append(table.Rows, pkgtable.Row{Cells: []string{}})
		}

		hop := stats.Hops[hopTTL]
		if hop == nil {
			continue
		}

		isFirstPeer := true
		for _, peerKey := range hop.PeerOrder {
			peerStats := hop.Peers[peerKey]
			if peerStats == nil {
				continue
			}

			// Row 1: Hop number, Peer name, RTT stats, Packet stats
			hopCell := ""
			if isFirstPeer {
				hopCell = fmt.Sprintf("%d.", hopTTL)
				isFirstPeer = false
			}

			// Peer name: [TIMEOUT] for timed out peers, RDNS or IP otherwise
			isTimeout := peerStats.ReceivedCount == 0 && peerStats.LossCount > 0
			peerName := ""
			if isTimeout {
				peerName = "[TIMEOUT]"
			} else {
				peerName = peerStats.PeerRDNS
				if peerName == "" {
					peerName = peerStats.Peer
				}
				if peerName == "" || peerName == "*" {
					peerName = "*"
				}
			}

			// RTT stats: last_rtt min/avg/max
			rttCell := ""
			if isTimeout {
				// No RTT data for timeout peers
			} else if peerStats.ReceivedCount > 0 && len(peerStats.Events) > 0 {
				lastRTT := 0
				for i := len(peerStats.Events) - 1; i >= 0; i-- {
					if !peerStats.Events[i].Timeout {
						lastRTT = peerStats.Events[i].RTTMs
						break
					}
				}
				avgRTT := peerStats.TotalRTT / peerStats.ReceivedCount
				rttCell = fmt.Sprintf("%dms %dms/%dms/%dms", lastRTT, peerStats.MinRTT, avgRTT, peerStats.MaxRTT)
			} else {
				rttCell = "* */*/*"
			}

			// Packet stats: received/total/loss%
			statsCell := ""
			if !isTimeout {
				totalPkts := peerStats.ReceivedCount + peerStats.LossCount
				lossPercent := 0.0
				if totalPkts > 0 {
					lossPercent = float64(peerStats.LossCount) / float64(totalPkts) * 100
				}
				statsCell = fmt.Sprintf("%d/%d/%.0f%%", peerStats.ReceivedCount, totalPkts, lossPercent)
			}

			table.Rows = append(table.Rows, pkgtable.Row{
				Cells: []string{hopCell, peerName, rttCell, statsCell},
			})

			// Row 2: IP address, ASN/ISP, Location
			ipCell := "(*)"
			if peerStats.Peer != "" && peerStats.Peer != "*" && peerStats.ReceivedCount > 0 {
				ipCell = fmt.Sprintf("(%s)", peerStats.Peer)
			}

			// ASN/ISP info (column 3, row 2)
			asnIspCell := ""
			if peerStats.ASN != "" || peerStats.ISP != "" {
				if peerStats.ASN != "" {
					asnIspCell = peerStats.ASN
				}
				if peerStats.ISP != "" {
					if asnIspCell != "" {
						asnIspCell += " " + peerStats.ISP
					} else {
						asnIspCell = peerStats.ISP
					}
				}
			}

			// Location info (column 4, row 2)
			locationCell := ""
			if peerStats.City != "" || peerStats.CountryAlpha2 != "" {
				if peerStats.City != "" {
					locationCell = peerStats.City
				}
				if peerStats.CountryAlpha2 != "" {
					if locationCell != "" {
						locationCell += "," + peerStats.CountryAlpha2
					} else {
						locationCell = peerStats.CountryAlpha2
					}
				}
			}

			table.Rows = append(table.Rows, pkgtable.Row{
				Cells: []string{"", ipCell, asnIspCell, locationCell},
			})
		}
	}

	return table
}

// Design:
//
// ```
// Hop  Peer           RTTs (Last Min/Avg/Max)  Stats (Rx/Tx/Loss)
//      (IP address)   ASN Network              City,Country
//
// 1.   homelab.local  1ms 1ms/2ms/3ms          2/3/33%
//      (192.168.1.1)
//
// 2.   a.example.com  10ms 10ms/10ms/10ms      3/3/0%
//      (17.18.19.20)  AS12345 Example LLC      HongKong,HK
//
// 3.   [TIMEOUT]
//      (*)
//
// 4.   google.com     100ms 100ms/100ms/100ms  1/1/0%
// ```
//
// Note:
//
// 1. If RDNS is empty string, use IP address as RDNS
// 2. A one-line space is between each hop

// GetHumanReadableText returns a formatted traceroute report
func (statsBuilder *TraceStatsBuilder) GetHumanReadableText() string {
	table := statsBuilder.ToTable()
	// *table = getExampleTable()

	return table.GetHumanReadableText(pkgtui.DefaultColGap, pkgtui.DefaultRowGap, pkgtui.DefaultMaxColWidth)
}
