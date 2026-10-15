//go:build !linux

package raw

// setSockOptPMTUProbe is a no-op on non-Linux platforms: IP_MTU_DISCOVER and
// IPV6_MTU_DISCOVER/IPV6_PMTUDISC_PROBE are Linux-specific socket options.
// The socket simply falls back to the platform's default fragmentation
// behavior there.
func setSockOptPMTUProbe(fd uintptr, ipv6 bool) error {
	return nil
}
