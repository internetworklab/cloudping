package utils

import (
	"context"
	"encoding/binary"
	"net"
)

// GetMemberAddresses32 enumerates all host addresses within the given IP network
// and streams them through the returned channel. Both IPv4 and IPv6 subnets are
// supported. The channel is closed automatically once all addresses have been
// emitted.
//
// Each net.IP sent on the channel is a fresh allocation, so callers may safely
// retain references without copying.
//
// Subnets up to /0 for IPv4 and up to /96 for IPv6 are supported. The caller is
// responsible for ensuring the subnet is small enough to iterate over in a
// reasonable time (e.g. /30 or /28 for IPv4, /120 or /126 for IPv6).
//
// For IPv4 all 32 bits of the address are varied using uint32 arithmetic.
// For IPv6 the upper 96 bits are kept fixed as a prefix and only the low 32
// bits are varied, which means subnets must be /96 or larger.
//
// Examples:
//
//	10.1.2.0/30        → 10.1.2.0, 10.1.2.1, 10.1.2.2, 10.1.2.3
//	192.168.13.103/32  → 192.168.13.103
//	172.20.0.0/28      → 172.20.0.0, 172.20.0.1, …, 172.20.0.15
//	fd00::/120         → fd00::, fd00::1, …, fd00::ff
const bufferSize = 4

// Compile-time assertion: bufferSize must not exceed the length of an IPv4 address.
var _ [net.IPv4len - bufferSize]byte

func GetMemberAddresses32(ctx context.Context, ipNet net.IPNet) <-chan net.IP {
	ch := make(chan net.IP)

	go func() {
		defer close(ch)

		// Apply the mask to get the network address.
		ip := ipNet.IP.Mask(ipNet.Mask)
		ones, bits := ipNet.Mask.Size()

		var prefix []byte // fixed upper bytes that don't vary
		var addr uint32   // starting value of the varying portion
		var resultLen int // bufferSize for IPv4, net.IPv6len for IPv6

		if bits == 32 {
			// IPv4: vary all 32 bits.
			ipBytes := ip.To4()
			resultLen = bufferSize
			addr = binary.BigEndian.Uint32(ipBytes)
		} else {
			// IPv6: upper 96 bits stay fixed, vary only the low 32 bits.
			ipBytes := ip.To16()
			resultLen = net.IPv6len
			prefix = make([]byte, net.IPv6len-bufferSize)
			copy(prefix, ipBytes[:net.IPv6len-bufferSize])
			addr = binary.BigEndian.Uint32(ipBytes[net.IPv6len-bufferSize:])
		}

		// Total number of addresses: 2^(bits - ones).
		hostBits := uint64(bits - ones)
		total := uint64(1) << hostBits

		for range total {
			select {
			case <-ctx.Done():
				return
			default:
			}

			result := make(net.IP, resultLen)
			if bits == 32 {
				binary.BigEndian.PutUint32(result, addr)
			} else {
				copy(result, prefix)
				binary.BigEndian.PutUint32(result[net.IPv6len-bufferSize:], addr)
			}

			select {
			case ch <- result:
			case <-ctx.Done():
				return
			}
			addr++
		}
	}()

	return ch
}
