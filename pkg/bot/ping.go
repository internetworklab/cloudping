package bot

import (
	"context"
	"fmt"
	"strings"
)

// Bot text display models

// String returns a formatted string representation of the ping event
func (e *PingEvent) String() string {
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

type MockPingEventsProvider struct{}

func (provider *MockPingEventsProvider) GetEventsByLocationCodeAndDestination(ctx context.Context, code string, destination string) <-chan PingEvent {
	lcode := strings.ToLower(code)

	evsToEVChan := func(evs []PingEvent) <-chan PingEvent {
		evsChan := make(chan PingEvent, 0)
		go func(evs []PingEvent) {
			defer close(evsChan)
			for _, ev := range evs {
				evsChan <- ev
			}
		}(evs)
		return evsChan
	}

	if lcode == "hk-hkg1" {
		evs := []PingEvent{
			{Seq: 0, RTTMs: 12, Peer: "10.0.1.1", PeerRDNS: "server-a1.local", IPPacketSize: 64, Timeout: false},
			{Seq: 1, RTTMs: 15, Peer: "10.0.1.2", PeerRDNS: "server-a2.local", IPPacketSize: 64, Timeout: false},
			{Seq: 2, RTTMs: 11, Peer: "10.0.1.3", PeerRDNS: "server-a3.local", IPPacketSize: 64, Timeout: false},
			{Seq: 3, RTTMs: 0, Peer: "10.0.1.4", PeerRDNS: "server-a4.local", IPPacketSize: 64, Timeout: true},
			{Seq: 4, RTTMs: 14, Peer: "10.0.1.5", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{Seq: 5, RTTMs: 20, Peer: "10.0.1.6", PeerRDNS: "server-a6.local", IPPacketSize: 64, Timeout: false},
			{Seq: 6, RTTMs: 16, Peer: "10.0.1.7", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{Seq: 7, RTTMs: 13, Peer: "10.0.1.8", PeerRDNS: "server-a8.local", IPPacketSize: 64, Timeout: false},
			{Seq: 8, RTTMs: 0, Peer: "10.0.1.9", PeerRDNS: "server-a9.local", IPPacketSize: 64, Timeout: true},
			{Seq: 9, RTTMs: 19, Peer: "10.0.1.10", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
		}
		return evsToEVChan(evs)
	} else if lcode == "us-lax1" {
		return evsToEVChan([]PingEvent{
			{Seq: 10, RTTMs: 65, Peer: "10.0.2.1", PeerRDNS: "node-b1.example.com", IPPacketSize: 64, Timeout: false},
			{Seq: 11, RTTMs: 72, Peer: "10.0.2.2", PeerRDNS: "node-b2.example.com", IPPacketSize: 64, Timeout: false},
			{Seq: 12, RTTMs: 58, Peer: "10.0.2.3", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{Seq: 13, RTTMs: 0, Peer: "10.0.2.4", PeerRDNS: "node-b4.example.com", IPPacketSize: 64, Timeout: true},
			{Seq: 14, RTTMs: 94, Peer: "10.0.2.5", PeerRDNS: "node-b5.example.com", IPPacketSize: 64, Timeout: false},
			{Seq: 15, RTTMs: 76, Peer: "10.0.2.6", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{Seq: 16, RTTMs: 0, Peer: "10.0.2.7", PeerRDNS: "node-b7.example.com", IPPacketSize: 64, Timeout: true},
			{Seq: 17, RTTMs: 85, Peer: "10.0.2.8", PeerRDNS: "node-b8.example.com", IPPacketSize: 64, Timeout: false},
			{Seq: 18, RTTMs: 68, Peer: "10.0.2.9", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{Seq: 19, RTTMs: 103, Peer: "10.0.2.10", PeerRDNS: "node-b10.example.com", IPPacketSize: 64, Timeout: false},
		})
	} else if lcode == "jp-tyo1" {
		return evsToEVChan([]PingEvent{
			{Seq: 20, RTTMs: 145, Peer: "192.168.100.1", PeerRDNS: "host-c1.remote.net", IPPacketSize: 64, Timeout: false},
			{Seq: 21, RTTMs: 187, Peer: "192.168.100.2", PeerRDNS: "host-c2.remote.net", IPPacketSize: 64, Timeout: false},
			{Seq: 22, RTTMs: 0, Peer: "192.168.100.3", PeerRDNS: "", IPPacketSize: 64, Timeout: true},
			{Seq: 23, RTTMs: 203, Peer: "192.168.100.4", PeerRDNS: "host-c4.remote.net", IPPacketSize: 64, Timeout: false},
			{Seq: 24, RTTMs: 178, Peer: "192.168.100.5", PeerRDNS: "host-c5.remote.net", IPPacketSize: 64, Timeout: false},
			{Seq: 25, RTTMs: 134, Peer: "192.168.100.6", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{Seq: 26, RTTMs: 0, Peer: "192.168.100.7", PeerRDNS: "host-c7.remote.net", IPPacketSize: 64, Timeout: true},
			{Seq: 27, RTTMs: 167, Peer: "192.168.100.8", PeerRDNS: "host-c8.remote.net", IPPacketSize: 64, Timeout: false},
			{Seq: 28, RTTMs: 198, Peer: "192.168.100.9", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{Seq: 29, RTTMs: 0, Peer: "192.168.100.10", PeerRDNS: "host-c10.remote.net", IPPacketSize: 64, Timeout: true},
		})
	} else if lcode == "de-fra1" {
		return evsToEVChan([]PingEvent{
			{Seq: 30, RTTMs: 312, Peer: "172.16.50.1", PeerRDNS: "far-d1.distant.io", IPPacketSize: 64, Timeout: false},
			{Seq: 31, RTTMs: 0, Peer: "172.16.50.2", PeerRDNS: "far-d2.distant.io", IPPacketSize: 64, Timeout: true},
			{Seq: 32, RTTMs: 378, Peer: "172.16.50.3", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{Seq: 33, RTTMs: 534, Peer: "172.16.50.4", PeerRDNS: "far-d4.distant.io", IPPacketSize: 64, Timeout: false},
			{Seq: 34, RTTMs: 0, Peer: "172.16.50.5", PeerRDNS: "far-d5.distant.io", IPPacketSize: 64, Timeout: true},
			{Seq: 35, RTTMs: 467, Peer: "172.16.50.6", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{Seq: 36, RTTMs: 398, Peer: "172.16.50.7", PeerRDNS: "far-d7.distant.io", IPPacketSize: 64, Timeout: false},
			{Seq: 37, RTTMs: 0, Peer: "172.16.50.8", PeerRDNS: "far-d8.distant.io", IPPacketSize: 64, Timeout: true},
			{Seq: 38, RTTMs: 423, Peer: "172.16.50.9", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{Seq: 39, RTTMs: 0, Peer: "172.16.50.10", PeerRDNS: "far-d10.distant.io", IPPacketSize: 64, Timeout: true},
		})
	} else {
		return evsToEVChan([]PingEvent{})
	}
}

type LocationDescriptor struct {
	Id                string
	Label             string
	Alpha2CountryCode string
	CityIATACode      string
}

func (provider *MockPingEventsProvider) GetAllLocations(ctx context.Context) []LocationDescriptor {
	return []LocationDescriptor{
		{Id: "hk-hkg1", Label: "HKG1", Alpha2CountryCode: "HK", CityIATACode: "HKG"},
		{Id: "us-lax1", Label: "LAX1", Alpha2CountryCode: "US", CityIATACode: "LAX"},
		{Id: "jp-tyo1", Label: "TYO1", Alpha2CountryCode: "JP", CityIATACode: "TYO"},
		{Id: "de-fra1", Label: "FRA1", Alpha2CountryCode: "DE", CityIATACode: "FRA"},
	}
}

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
	pingEvs          []PingEvent
	receivedPktCount int
	lossPktCount     int
	minRTT           int
	maxRTT           int
	totalRTT         int
}

func (statsBuilder *PingStatisticsBuilder) WriteEvent(ev PingEvent) {
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

func FormatPingCallbackData(locationCode string) string {
	return fmt.Sprintf("ping_location_%s", locationCode)
}

func ParseLocationCodeFromPingCallbackData(pingCallbackData string) string {
	if suffix, found := strings.CutPrefix(pingCallbackData, "ping_location_"); found {
		return suffix
	}
	return ""
}
