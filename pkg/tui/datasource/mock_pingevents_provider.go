package datasource

import (
	"context"
	"strings"

	pkgtui "github.com/internetworklab/cloudping/pkg/tui"
)

// MockPingEventsProvider is an implementation
// of pkgtui.PingEventsProvider interface
type MockPingEventsProvider struct{}

func (provider *MockPingEventsProvider) GetEvents(ctx context.Context, pingRequest *pkgtui.PingRequestDescriptor) <-chan pkgtui.PingEvent {
	lcode := strings.ToLower(pingRequest.Sources[0])

	evsToEVChan := func(evs []pkgtui.PingEvent) <-chan pkgtui.PingEvent {
		evsChan := make(chan pkgtui.PingEvent, 0)
		go func(evs []pkgtui.PingEvent) {
			defer close(evsChan)
			for _, ev := range evs {
				evsChan <- ev
			}
		}(evs)
		return evsChan
	}

	if lcode == "hk-hkg1" {
		evs := []pkgtui.PingEvent{
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
		return evsToEVChan([]pkgtui.PingEvent{
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
		return evsToEVChan([]pkgtui.PingEvent{
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
		return evsToEVChan([]pkgtui.PingEvent{
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
		return evsToEVChan([]pkgtui.PingEvent{})
	}
}

func (provider *MockPingEventsProvider) GetAllLocations(ctx context.Context) ([]pkgtui.LocationDescriptor, error) {
	return []pkgtui.LocationDescriptor{
		{Id: "hk-hkg1", Label: "HKG1", Alpha2CountryCode: "HK", CityIATACode: "HKG"},
		{Id: "us-lax1", Label: "LAX1", Alpha2CountryCode: "US", CityIATACode: "LAX"},
		{Id: "jp-tyo1", Label: "TYO1", Alpha2CountryCode: "JP", CityIATACode: "TYO"},
		{Id: "de-fra1", Label: "FRA1", Alpha2CountryCode: "DE", CityIATACode: "FRA"},
	}, nil
}
