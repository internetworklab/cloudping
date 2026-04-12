package utils

import "net"

// Return the diff between the host address and the network address.
// For example:
// network=1.0.0.0/24, host=1.0.0.3, diff=3
// network=fd00::/64, host=fd00::7, diff=7
// For IPv4, up to 32 bits of host addresses are supported, i.e. the subnet can be as large as /0.
// For IPv6, up to 64 bits of host addresses are supported, i.e. the subnet can be as large as /64,
// /64, /65, ..., /128 are all valid IPv6 subnet while /64 or broader are not.
func GetOffset(network net.IPNet, host net.IP) uint64 {
	return 0
}
