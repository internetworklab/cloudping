package utils

import (
	"context"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strings"
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

func CheckDomainInRange(domain string, rangeDomains []regexp.Regexp) bool {
	if len(rangeDomains) == 0 {
		panic("CheckDomainInRange must be called with a non-empty rangeDomains")
	}

	for _, rangeDomain := range rangeDomains {
		if rangeDomain.MatchString(domain) {
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

func GetHostFromIP(ipstr string) (net.IP, error) {
	ip := net.ParseIP(ipstr)
	if ip == nil {
		return nil, fmt.Errorf("failed to parse IP from string: %s", ipstr)
	}
	return ip, nil
}

func GetHost(addrport string) (net.IP, error) {
	urlObj, err := url.Parse(addrport)
	if err == nil && urlObj != nil {
		addrport = urlObj.Host
		if strings.HasPrefix(addrport, "[") && strings.HasSuffix(addrport, "]") {
			addrport, _ = strings.CutPrefix(addrport, "[")
			addrport, _ = strings.CutSuffix(addrport, "]")
		}
	}
	host, _, err := net.SplitHostPort(addrport)
	if err != nil {
		return GetHostFromIP(addrport)
	}
	return GetHostFromIP(host)
}

// ipRange represents a contiguous range of IP addresses as [start, end] inclusive,
// using big.Int so that both IPv4 and IPv6 values can be compared arithmetically.
type ipRange struct {
	start, end *big.Int
}

// ipNetToRange converts a net.IPNet into an ipRange by computing the network
// (first) and broadcast (last) addresses of the subnet.
func ipNetToRange(n net.IPNet) ipRange {
	startIP := n.IP.Mask(n.Mask)
	if startIP == nil {
		// Mask is incompatible with IP length; treat as a single host.
		startIP = n.IP
	}

	endIP := make(net.IP, len(startIP))
	copy(endIP, startIP)
	for i := range n.Mask {
		if i < len(endIP) {
			endIP[i] |= ^n.Mask[i]
		}
	}

	return ipRange{
		start: new(big.Int).SetBytes(startIP),
		end:   new(big.Int).SetBytes(endIP),
	}
}

// canMergeWith returns true when the receiver and rhs overlap or are
// consecutive (i.e. their union forms a single contiguous interval).
func (r *ipRange) canMergeWith(rhs *ipRange) bool {
	endPlusOne := new(big.Int).Add(r.end, big.NewInt(1))
	return rhs.start.Cmp(endPlusOne) <= 0
}

// MergeWith returns a new ipRange that is the union of the receiver and rhs.
// The caller must ensure the two ranges are overlapping or consecutive
// (e.g. by checking canMergeWith first).
func (r *ipRange) MergeWith(rhs *ipRange) *ipRange {
	merged := ipRange{
		start: new(big.Int).Set(r.start),
		end:   new(big.Int).Set(r.end),
	}
	if rhs.start.Cmp(merged.start) < 0 {
		merged.start.Set(rhs.start)
	}
	if rhs.end.Cmp(merged.end) > 0 {
		merged.end.Set(rhs.end)
	}
	return &merged
}

// returns true if and only if the addresses set of subnet A (`subnetA`) is a subset of the union set of `allowedSets`
func IsSubset(subnetA net.IPNet, allowedSets []net.IPNet) bool {
	// Determine the address family of subnetA.
	isIPv4A := subnetA.IP.To4() != nil

	// Convert subnetA to a big-int interval.
	rangeA := ipNetToRange(subnetA)

	// Collect intervals from allowedSets that share the same address family.
	var ranges []ipRange
	for _, allowed := range allowedSets {
		if (allowed.IP.To4() != nil) != isIPv4A {
			continue
		}
		r := ipNetToRange(allowed)
		ranges = append(ranges, r)
	}

	if len(ranges) == 0 {
		return false
	}

	// Sort intervals by their start address.
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].start.Cmp(ranges[j].start) < 0
	})

	// Merge overlapping and consecutive intervals.
	merged := make([]ipRange, 0, len(ranges))
	cur := ranges[0]
	for i := 1; i < len(ranges); i++ {
		if cur.canMergeWith(&ranges[i]) {
			cur = *cur.MergeWith(&ranges[i])
		} else {
			merged = append(merged, cur)
			cur = ranges[i]
		}
	}
	merged = append(merged, cur)

	// Check whether any single merged interval fully covers [startA, endA].
	for _, r := range merged {
		if r.start.Cmp(rangeA.start) <= 0 && r.end.Cmp(rangeA.end) >= 0 {
			return true
		}
	}

	return false
}
