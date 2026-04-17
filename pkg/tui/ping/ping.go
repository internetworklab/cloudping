package ping

import (
	"fmt"
	"strings"

	pkgtui "github.com/internetworklab/cloudping/pkg/tui"
)

// PingStatistics holds calculated statistics for a ping task
type PingStatistics struct {
	ReceivedPktCount int
	LossPktCount     int
	MinRTT           int
	MaxRTT           int
	AvgRTT           int
}

// String returns a formatted string representation of the ping statistics
func (s *PingStatistics) String() string {
	totalPkts := s.ReceivedPktCount + s.LossPktCount
	lossPercent := 0.0
	if totalPkts > 0 {
		lossPercent = float64(s.LossPktCount) / float64(totalPkts) * 100
	}
	return fmt.Sprintf("--- ping statistics ---\n"+
		"%d packets transmitted, %d packets received, %.1f%% packet loss\n"+
		"round-trip min/avg/max = %d/%d/%d ms",
		totalPkts, s.ReceivedPktCount, lossPercent, s.MinRTT, s.AvgRTT, s.MaxRTT)
}

type PingStatisticsBuilder struct {
	pingEvs          []pkgtui.PingEvent
	receivedPktCount int
	lossPktCount     int
	minRTT           int
	maxRTT           int
	totalRTT         int
}

func (statsBuilder *PingStatisticsBuilder) WriteEvent(ev pkgtui.PingEvent) {
	statsBuilder.pingEvs = append(statsBuilder.pingEvs, ev)

	// Update packet counts
	if ev.Timeout {
		statsBuilder.lossPktCount++
	} else {
		statsBuilder.receivedPktCount++
		// Update RTT statistics for non-timeout packets
		// For the first received packet, initialize min and max RTT
		if statsBuilder.receivedPktCount == 1 {
			statsBuilder.minRTT = ev.RTTMs
			statsBuilder.maxRTT = ev.RTTMs
		} else {
			if ev.RTTMs < statsBuilder.minRTT {
				statsBuilder.minRTT = ev.RTTMs
			}
			if ev.RTTMs > statsBuilder.maxRTT {
				statsBuilder.maxRTT = ev.RTTMs
			}
		}
		statsBuilder.totalRTT += ev.RTTMs
	}
}

// getPingStatistics calculates and returns statistics for a given ping task.
// Returns nil if no events found for the task.
func (statsBuilder *PingStatisticsBuilder) GetPingStatistics() *PingStatistics {
	if len(statsBuilder.pingEvs) == 0 {
		return nil
	}

	// Calculate average RTT
	avgRTT := 0
	if statsBuilder.receivedPktCount > 0 {
		avgRTT = statsBuilder.totalRTT / statsBuilder.receivedPktCount
	}

	return &PingStatistics{
		ReceivedPktCount: statsBuilder.receivedPktCount,
		LossPktCount:     statsBuilder.lossPktCount,
		MinRTT:           statsBuilder.minRTT,
		MaxRTT:           statsBuilder.maxRTT,
		AvgRTT:           avgRTT,
	}
}

// getFormattedPingEvents returns a formatted string of ping events for a given ping task,
// similar to the output of a ping command (individual replies, not statistics).
// Returns an empty string if no events found for the ping task.
func (statsBuilder *PingStatisticsBuilder) GetFormattedPingEvents() string {
	pingEvs := statsBuilder.pingEvs

	if len(pingEvs) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, event := range pingEvs {
		sb.WriteString(event.String() + "\n")
	}

	return sb.String()
}

func (statsBuilder *PingStatisticsBuilder) GetHumanReadableText() string {
	stats := ""
	if s := statsBuilder.GetPingStatistics(); s != nil {
		stats = s.String()
	}

	pingEvents := statsBuilder.GetFormattedPingEvents()
	txt := pingEvents + "\n" + stats
	return txt
}
