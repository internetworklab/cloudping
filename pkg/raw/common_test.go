package raw

import (
	"testing"

	"github.com/google/gopacket/layers"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

func intPtr(v int) *int {
	return &v
}

func TestGetMaxPayloadLen(t *testing.T) {
	const maxLen = 65535 // maximum size of an IP (or IPv6) packet, header included

	testCases := []struct {
		name      string
		ipVersion int
		protoNum  int
		pmtu      *int
		mtuCap    int
		expected  int
	}{
		{"IPv4_ICMP_NilPMTU_UsesCap", ipv4.Version, int(layers.IPProtocolICMPv4), nil, maxLen, maxLen - ipv4.HeaderLen - headerSizeICMP},
		{"IPv4_UDP_NilPMTU_UsesCap", ipv4.Version, int(layers.IPProtocolUDP), nil, maxLen, maxLen - ipv4.HeaderLen - udpHeaderLen},
		{"IPv6_ICMP_NilPMTU_UsesCap", ipv6.Version, int(layers.IPProtocolICMPv6), nil, maxLen, maxLen - ipv6.HeaderLen - headerSizeICMP},
		{"IPv6_UDP_NilPMTU_UsesCap", ipv6.Version, int(layers.IPProtocolUDP), nil, maxLen, maxLen - ipv6.HeaderLen - udpHeaderLen},

		{"IPv4_ICMP_SmallerPMTUWins", ipv4.Version, int(layers.IPProtocolICMPv4), intPtr(1500), maxLen, 1500 - ipv4.HeaderLen - headerSizeICMP},
		{"IPv6_ICMP_SmallerPMTUWins", ipv6.Version, int(layers.IPProtocolICMPv6), intPtr(1500), maxLen, 1500 - ipv6.HeaderLen - headerSizeICMP},
		{"IPv4_UDP_LargerPMTUIgnored", ipv4.Version, int(layers.IPProtocolUDP), intPtr(70000), maxLen, maxLen - ipv4.HeaderLen - udpHeaderLen},
		{"IPv4_ICMP_NegativePMTUIgnored", ipv4.Version, int(layers.IPProtocolICMPv4), intPtr(-1), maxLen, maxLen - ipv4.HeaderLen - headerSizeICMP},

		{"IPv4_ICMP_TinyCapClampsToZero", ipv4.Version, int(layers.IPProtocolICMPv4), nil, ipv4.HeaderLen, 0},
		{"IPv4_TCP_UnknownProtoIsZero", ipv4.Version, int(layers.IPProtocolTCP), nil, maxLen, 0},
		{"UnknownIPVersion_IsZero", 5, int(layers.IPProtocolICMPv4), nil, maxLen, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := GetMaxPayloadLen(tc.ipVersion, tc.protoNum, tc.pmtu, tc.mtuCap); got != tc.expected {
				t.Fatalf("GetMaxPayloadLen(%d, %d, %v, %d) = %d, expected %d", tc.ipVersion, tc.protoNum, tc.pmtu, tc.mtuCap, got, tc.expected)
			}
		})
	}
}
