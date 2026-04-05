package utils

import (
	"encoding/binary"
	"log"
	"net"
)

// GetOffset computes the numerical offset of a host address relative to its
// parent subnet's network (base) address. Conceptually, it is host minus network,
// treating both as integers and masking out the prefix portion.
//
// Examples:
//   - network=1.0.0.0/24, host=1.0.0.3 → offset=3
//   - network=fd00::/64,  host=fd00::7  → offset=7
//
// For IPv4, up to 32 host bits are supported (subnets as broad as /0).
// For IPv6, up to 64 host bits are supported (subnets as narrow as /64);
// prefix lengths shorter than /64 are not supported and will cause a panic.
func GetOffset(network net.IPNet, host net.IP) uint64 {
	// IPv4 case: normalize both IPs to 4-byte form, then XOR and mask
	// using native uint32 arithmetic loaded via binary.BigEndian.
	if v4 := host.To4(); v4 != nil {
		netIP := network.IP.To4()
		hostVal := binary.BigEndian.Uint32(v4)
		netVal := binary.BigEndian.Uint32(netIP)
		maskVal := binary.BigEndian.Uint32(network.Mask)
		return uint64((hostVal ^ netVal) &^ maskVal)
	}

	// IPv6 case: for prefixes >= /64 the upper 8 bytes are fully masked out,
	// so only the lower 8 bytes contribute to the offset. Load them as uint64
	// via binary.BigEndian, then XOR and mask with native arithmetic.
	prefixLen, _ := network.Mask.Size()
	if prefixLen < 64 {
		log.Panicf("GetOffset: IPv6 prefix /%d is shorter than /64, offset would overflow uint64", prefixLen)
	}
	v6 := host.To16()
	netIP := network.IP.To16()
	hostLower := binary.BigEndian.Uint64(v6[8:])
	netLower := binary.BigEndian.Uint64(netIP[8:])
	maskLower := binary.BigEndian.Uint64(network.Mask[8:])
	return (hostLower ^ netLower) &^ maskLower
}
