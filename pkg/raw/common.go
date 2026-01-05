package raw

import (
	"log"
	"math"
	"strings"

	"github.com/google/gopacket/layers"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// ipVersion: 4 or 6
// ipprotoNum: for IPv4, it's the iana ipprotocol number, for IPv6, it's the NextHeader field value
func GetMaxPayloadLen(ipVersion int, ipprotoNum int, pmtu *int, nexthopMTU int) int {
	minMTU := nexthopMTU
	if pmtu != nil {
		if *pmtu >= 0 && *pmtu < minMTU {
			minMTU = *pmtu
		}
	}

	switch ipVersion {
	case ipv4.Version:
		switch ipprotoNum {
		case int(layers.IPProtocolICMPv4):
			return int(math.Max(0, float64(minMTU-ipv4.HeaderLen-headerSizeICMP)))
		case int(layers.IPProtocolUDP):
			return int(math.Max(0, float64(minMTU-ipv4.HeaderLen-udpHeaderLen)))
		default:
			log.Printf("unknown ip protocol number: %d", ipprotoNum)
			return 0
		}
	case ipv6.Version:
		switch ipprotoNum {
		case int(layers.IPProtocolICMPv6):
			return int(math.Max(0, float64(minMTU-ipv6.HeaderLen-headerSizeICMP)))
		case int(layers.IPProtocolUDP):
			return int(math.Max(0, float64(minMTU-ipv6.HeaderLen-udpHeaderLen)))
		default:
			log.Printf("unknown ip protocol number: %d", ipprotoNum)
			return 0
		}
	default:
		log.Printf("unknown ip version: %d", ipVersion)
		return 0
	}
}

func isFatalErr(err error) bool {
	errStr := err.Error()
	if strings.Contains(errStr, "message too long") {
		return false
	}

	return true
}
