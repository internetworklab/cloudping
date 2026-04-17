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

	mockedRttFlt := func(v float64) *float64 {
		vptr := new(float64)
		*vptr = v
		return vptr
	}

	if lcode == "hk-hkg1" {
		evs := []pkgtui.PingEvent{
			{From: "hk-hkg1", To: pingRequest.Destinations[0], Seq: 0, RttMsFlt: mockedRttFlt(12.345), RTTMs: 12, Peer: "10.0.1.1", PeerRDNS: "server-a1.local", IPPacketSize: 64, Timeout: false},
			{From: "hk-hkg1", To: pingRequest.Destinations[0], Seq: 1, RttMsFlt: mockedRttFlt(15.678), RTTMs: 15, Peer: "10.0.1.2", PeerRDNS: "server-a2.local", IPPacketSize: 64, Timeout: false},
			{From: "hk-hkg1", To: pingRequest.Destinations[0], Seq: 2, RttMsFlt: mockedRttFlt(11.234), RTTMs: 11, Peer: "10.0.1.3", PeerRDNS: "server-a3.local", IPPacketSize: 64, Timeout: false},
			{From: "hk-hkg1", To: pingRequest.Destinations[0], Seq: 3, RttMsFlt: nil, RTTMs: 0, Peer: "10.0.1.4", PeerRDNS: "server-a4.local", IPPacketSize: 64, Timeout: true},
			{From: "hk-hkg1", To: pingRequest.Destinations[0], Seq: 4, RttMsFlt: mockedRttFlt(14.567), RTTMs: 14, Peer: "10.0.1.5", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{From: "hk-hkg1", To: pingRequest.Destinations[0], Seq: 5, RttMsFlt: mockedRttFlt(20.891), RTTMs: 20, Peer: "10.0.1.6", PeerRDNS: "server-a6.local", IPPacketSize: 64, Timeout: false},
			{From: "hk-hkg1", To: pingRequest.Destinations[0], Seq: 6, RttMsFlt: mockedRttFlt(16.432), RTTMs: 16, Peer: "10.0.1.7", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{From: "hk-hkg1", To: pingRequest.Destinations[0], Seq: 7, RttMsFlt: mockedRttFlt(13.765), RTTMs: 13, Peer: "10.0.1.8", PeerRDNS: "server-a8.local", IPPacketSize: 64, Timeout: false},
			{From: "hk-hkg1", To: pingRequest.Destinations[0], Seq: 8, RttMsFlt: nil, RTTMs: 0, Peer: "10.0.1.9", PeerRDNS: "server-a9.local", IPPacketSize: 64, Timeout: true},
			{From: "hk-hkg1", To: pingRequest.Destinations[0], Seq: 9, RttMsFlt: mockedRttFlt(19.098), RTTMs: 19, Peer: "10.0.1.10", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
		}
		return evsToEVChan(evs)
	} else if lcode == "us-lax1" {
		return evsToEVChan([]pkgtui.PingEvent{
			{From: "us-lax1", To: pingRequest.Destinations[0], Seq: 10, RttMsFlt: mockedRttFlt(65.234), RTTMs: 65, Peer: "10.0.2.1", PeerRDNS: "node-b1.example.com", IPPacketSize: 64, Timeout: false},
			{From: "us-lax1", To: pingRequest.Destinations[0], Seq: 11, RttMsFlt: mockedRttFlt(72.567), RTTMs: 72, Peer: "10.0.2.2", PeerRDNS: "node-b2.example.com", IPPacketSize: 64, Timeout: false},
			{From: "us-lax1", To: pingRequest.Destinations[0], Seq: 12, RttMsFlt: mockedRttFlt(58.891), RTTMs: 58, Peer: "10.0.2.3", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{From: "us-lax1", To: pingRequest.Destinations[0], Seq: 13, RttMsFlt: nil, RTTMs: 0, Peer: "10.0.2.4", PeerRDNS: "node-b4.example.com", IPPacketSize: 64, Timeout: true},
			{From: "us-lax1", To: pingRequest.Destinations[0], Seq: 14, RttMsFlt: mockedRttFlt(94.123), RTTMs: 94, Peer: "10.0.2.5", PeerRDNS: "node-b5.example.com", IPPacketSize: 64, Timeout: false},
			{From: "us-lax1", To: pingRequest.Destinations[0], Seq: 15, RttMsFlt: mockedRttFlt(76.456), RTTMs: 76, Peer: "10.0.2.6", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{From: "us-lax1", To: pingRequest.Destinations[0], Seq: 16, RttMsFlt: nil, RTTMs: 0, Peer: "10.0.2.7", PeerRDNS: "node-b7.example.com", IPPacketSize: 64, Timeout: true},
			{From: "us-lax1", To: pingRequest.Destinations[0], Seq: 17, RttMsFlt: mockedRttFlt(85.789), RTTMs: 85, Peer: "10.0.2.8", PeerRDNS: "node-b8.example.com", IPPacketSize: 64, Timeout: false},
			{From: "us-lax1", To: pingRequest.Destinations[0], Seq: 18, RttMsFlt: mockedRttFlt(68.012), RTTMs: 68, Peer: "10.0.2.9", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{From: "us-lax1", To: pingRequest.Destinations[0], Seq: 19, RttMsFlt: mockedRttFlt(103.345), RTTMs: 103, Peer: "10.0.2.10", PeerRDNS: "node-b10.example.com", IPPacketSize: 64, Timeout: false},
		})
	} else if lcode == "jp-tyo1" {
		return evsToEVChan([]pkgtui.PingEvent{
			{From: "jp-tyo1", To: pingRequest.Destinations[0], Seq: 20, RttMsFlt: mockedRttFlt(145.234), RTTMs: 145, Peer: "192.168.100.1", PeerRDNS: "host-c1.remote.net", IPPacketSize: 64, Timeout: false},
			{From: "jp-tyo1", To: pingRequest.Destinations[0], Seq: 21, RttMsFlt: mockedRttFlt(187.567), RTTMs: 187, Peer: "192.168.100.2", PeerRDNS: "host-c2.remote.net", IPPacketSize: 64, Timeout: false},
			{From: "jp-tyo1", To: pingRequest.Destinations[0], Seq: 22, RttMsFlt: nil, RTTMs: 0, Peer: "192.168.100.3", PeerRDNS: "", IPPacketSize: 64, Timeout: true},
			{From: "jp-tyo1", To: pingRequest.Destinations[0], Seq: 23, RttMsFlt: mockedRttFlt(203.891), RTTMs: 203, Peer: "192.168.100.4", PeerRDNS: "host-c4.remote.net", IPPacketSize: 64, Timeout: false},
			{From: "jp-tyo1", To: pingRequest.Destinations[0], Seq: 24, RttMsFlt: mockedRttFlt(178.123), RTTMs: 178, Peer: "192.168.100.5", PeerRDNS: "host-c5.remote.net", IPPacketSize: 64, Timeout: false},
			{From: "jp-tyo1", To: pingRequest.Destinations[0], Seq: 25, RttMsFlt: mockedRttFlt(134.456), RTTMs: 134, Peer: "192.168.100.6", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{From: "jp-tyo1", To: pingRequest.Destinations[0], Seq: 26, RttMsFlt: nil, RTTMs: 0, Peer: "192.168.100.7", PeerRDNS: "host-c7.remote.net", IPPacketSize: 64, Timeout: true},
			{From: "jp-tyo1", To: pingRequest.Destinations[0], Seq: 27, RttMsFlt: mockedRttFlt(167.789), RTTMs: 167, Peer: "192.168.100.8", PeerRDNS: "host-c8.remote.net", IPPacketSize: 64, Timeout: false},
			{From: "jp-tyo1", To: pingRequest.Destinations[0], Seq: 28, RttMsFlt: mockedRttFlt(198.012), RTTMs: 198, Peer: "192.168.100.9", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{From: "jp-tyo1", To: pingRequest.Destinations[0], Seq: 29, RttMsFlt: nil, RTTMs: 0, Peer: "192.168.100.10", PeerRDNS: "host-c10.remote.net", IPPacketSize: 64, Timeout: true},
		})
	} else if lcode == "de-fra1" {
		return evsToEVChan([]pkgtui.PingEvent{
			{From: "de-fra1", To: pingRequest.Destinations[0], Seq: 30, RttMsFlt: mockedRttFlt(312.456), RTTMs: 312, Peer: "172.16.50.1", PeerRDNS: "far-d1.distant.io", IPPacketSize: 64, Timeout: false},
			{From: "de-fra1", To: pingRequest.Destinations[0], Seq: 31, RttMsFlt: nil, RTTMs: 0, Peer: "172.16.50.2", PeerRDNS: "far-d2.distant.io", IPPacketSize: 64, Timeout: true},
			{From: "de-fra1", To: pingRequest.Destinations[0], Seq: 32, RttMsFlt: mockedRttFlt(378.789), RTTMs: 378, Peer: "172.16.50.3", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{From: "de-fra1", To: pingRequest.Destinations[0], Seq: 33, RttMsFlt: mockedRttFlt(534.012), RTTMs: 534, Peer: "172.16.50.4", PeerRDNS: "far-d4.distant.io", IPPacketSize: 64, Timeout: false},
			{From: "de-fra1", To: pingRequest.Destinations[0], Seq: 34, RttMsFlt: nil, RTTMs: 0, Peer: "172.16.50.5", PeerRDNS: "far-d5.distant.io", IPPacketSize: 64, Timeout: true},
			{From: "de-fra1", To: pingRequest.Destinations[0], Seq: 35, RttMsFlt: mockedRttFlt(467.345), RTTMs: 467, Peer: "172.16.50.6", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{From: "de-fra1", To: pingRequest.Destinations[0], Seq: 36, RttMsFlt: mockedRttFlt(398.678), RTTMs: 398, Peer: "172.16.50.7", PeerRDNS: "far-d7.distant.io", IPPacketSize: 64, Timeout: false},
			{From: "de-fra1", To: pingRequest.Destinations[0], Seq: 37, RttMsFlt: nil, RTTMs: 0, Peer: "172.16.50.8", PeerRDNS: "far-d8.distant.io", IPPacketSize: 64, Timeout: true},
			{From: "de-fra1", To: pingRequest.Destinations[0], Seq: 38, RttMsFlt: mockedRttFlt(423.901), RTTMs: 423, Peer: "172.16.50.9", PeerRDNS: "", IPPacketSize: 64, Timeout: false},
			{From: "de-fra1", To: pingRequest.Destinations[0], Seq: 39, RttMsFlt: nil, RTTMs: 0, Peer: "172.16.50.10", PeerRDNS: "far-d10.distant.io", IPPacketSize: 64, Timeout: true},
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
