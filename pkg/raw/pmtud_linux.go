//go:build linux

package raw

import "syscall"

// setSockOptPMTUProbe puts the socket into PMTU "probe" mode: the kernel won't
// fragment outgoing packets nor enforce the (cached) path MTU on them, while
// ICMP Fragmentation-Needed / Packet-Too-Big errors are still reported back,
// so PMTU probing can be driven from userspace.
func setSockOptPMTUProbe(fd uintptr, ipv6 bool) error {
	if !ipv6 {
		return syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_MTU_DISCOVER, syscall.IP_PMTUDISC_PROBE)
	}

	if err := syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IPV6, syscall.IPV6_MTU_DISCOVER, syscall.IPV6_PMTUDISC_PROBE); err != nil {
		return err
	}

	// see rfc3542, section 11.2 "Sending without Fragmentation"
	const IPV6_DONTFRAG int = 62
	return syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IPV6, IPV6_DONTFRAG, 1)
}
