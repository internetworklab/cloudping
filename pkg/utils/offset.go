package utils

import (
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
	// IPv4 case: normalize both IPs to 4-byte form.
	if v4 := host.To4(); v4 != nil {
		netIP := network.IP.To4()
		var offset uint64
		for i := range net.IPv4len {
			offset = (offset << 8) | uint64((v4[i]^netIP[i])&^network.Mask[i])
		}
		return offset
	}

	// IPv6 case: normalize both IPs to 16-byte form.
	// For prefixes of /64 or longer, the upper 8 bytes are fully masked out,
	// so only the lower 8 bytes contribute to the offset, fitting in uint64.
	prefixLen, _ := network.Mask.Size()
	if prefixLen < 64 {
		log.Panicf("GetOffset: IPv6 prefix /%d is shorter than /64, offset would overflow uint64", prefixLen)
	}
	v6 := host.To16()
	netIP := network.IP.To16()
	var offset uint64
	for i := range net.IPv6len {
		offset = (offset << 8) | uint64((v6[i]^netIP[i])&^network.Mask[i])
	}
	return offset
}
