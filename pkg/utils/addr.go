package utils

import (
	"context"
	"fmt"
	"net"
)

func SelectDstIP(ctx context.Context, resolver *net.Resolver, host string, preferV4 *bool, preferV6 *bool, respondRange []net.IPNet) (*net.IPAddr, error) {
	familyPrefer := "ip"
	if preferV6 != nil && *preferV6 {
		familyPrefer = "ip6"
	} else if preferV4 != nil && *preferV4 {
		familyPrefer = "ip4"
	}

	ips, err := resolver.LookupIP(ctx, familyPrefer, host)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup IP: %v", err)
	}

	if len(respondRange) > 0 {
		ips = filterIPs(ips, respondRange)
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no IP available for the host: %s", host)
	}

	dst := net.IPAddr{IP: ips[0]}
	return &dst, nil
}

func filterIPs(ips []net.IP, respondRange []net.IPNet) []net.IP {
	filteredIPs := make([]net.IP, 0)
	for _, ip := range ips {
		for _, rangeCIDR := range respondRange {
			if rangeCIDR.Contains(ip) {
				filteredIPs = append(filteredIPs, ip)
			}
		}
	}
	return filteredIPs
}

func CheckIntersectIP(dstIP net.IP, rangeCIDRs []net.IPNet) bool {
	if len(rangeCIDRs) == 0 {
		panic("CheckIntersectIP must be called with a non-empty rangeCIDRs")
	}

	for _, rangeCIDR := range rangeCIDRs {
		if rangeCIDR.Contains(dstIP) {
			return true
		}
	}
	return false
}

func CheckIntersect(dstIPs []net.IP, rangeCIDRs []net.IPNet) bool {

	if len(rangeCIDRs) == 0 {
		panic("CheckIntersect must be called with a non-empty rangeCIDRs")
	}

	for _, dstIP := range dstIPs {
		if !CheckIntersectIP(dstIP, rangeCIDRs) {
			return false
		}
	}
	return true
}
