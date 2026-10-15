package utils

import (
	"net"
)

// MaxIPPacketLen is the maximum length of an IP (or IPv6) packet in bytes,
// including the IP header. It is used as the initial guess of PMTU, before
// any ICMP feedback (Fragmentation Needed / Packet Too Big) refines it.
const MaxIPPacketLen = 65535

func GetMaximumMTU() int {
	ifaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}
	maximumMTU := -1
	for _, iface := range ifaces {
		if iface.MTU > maximumMTU {
			maximumMTU = iface.MTU
		}
	}
	if maximumMTU == -1 {
		panic("can't determine maximum MTU")
	}
	return maximumMTU
}
