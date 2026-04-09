package utils

import (
	"net"
	"testing"
)

// mustParseCIDR is defined in get_member_addrs_test.go (same package).

// --- Exact match ---

func TestIsSubset_ExactMatch(t *testing.T) {
	subnetA := mustParseCIDR(t, "10.0.0.0/24")
	allowed := []net.IPNet{mustParseCIDR(t, "10.0.0.0/24")}

	if !IsSubset(subnetA, allowed) {
		t.Error("expected true: subnetA exactly matches an allowed set")
	}
}

// --- Broader allowed (allowed is a superset) ---

func TestIsSubset_BroaderAllowed_IPv4(t *testing.T) {
	subnetA := mustParseCIDR(t, "10.0.1.0/24")
	allowed := []net.IPNet{mustParseCIDR(t, "10.0.0.0/16")}

	if !IsSubset(subnetA, allowed) {
		t.Error("expected true: /24 is within /16")
	}
}

func TestIsSubset_BroaderAllowed_IPv6(t *testing.T) {
	subnetA := mustParseCIDR(t, "fd00::1/128")
	allowed := []net.IPNet{mustParseCIDR(t, "fd00::/64")}

	if !IsSubset(subnetA, allowed) {
		t.Error("expected true: /128 host is within /64")
	}
}

// --- Multiple allowed sets whose union covers subnetA ---

func TestIsSubset_MultipleAllowed_CoveredByUnion(t *testing.T) {
	// subnetA: 10.0.0.0/23 => 10.0.0.0 – 10.0.1.255
	subnetA := mustParseCIDR(t, "10.0.0.0/23")
	// Two /24s that together cover the entire /23
	allowed := []net.IPNet{
		mustParseCIDR(t, "10.0.0.0/24"),
		mustParseCIDR(t, "10.0.1.0/24"),
	}

	if !IsSubset(subnetA, allowed) {
		t.Error("expected true: union of two /24s covers /23")
	}
}

func TestIsSubset_MultipleAllowed_NotFullyCovered(t *testing.T) {
	// subnetA: 10.0.0.0/23 => 10.0.0.0 – 10.0.1.255
	subnetA := mustParseCIDR(t, "10.0.0.0/23")
	// Only one /24 — missing 10.0.1.0/24
	allowed := []net.IPNet{
		mustParseCIDR(t, "10.0.0.0/24"),
	}

	if IsSubset(subnetA, allowed) {
		t.Error("expected false: single /24 does not cover /23")
	}
}

// --- Overlapping allowed sets ---

func TestIsSubset_OverlappingAllowed(t *testing.T) {
	subnetA := mustParseCIDR(t, "192.168.1.0/24")
	allowed := []net.IPNet{
		mustParseCIDR(t, "192.168.0.0/23"),   // covers 192.168.0.0 – 192.168.1.255
		mustParseCIDR(t, "192.168.1.128/25"), // overlaps with the above
	}

	if !IsSubset(subnetA, allowed) {
		t.Error("expected true: /23 alone covers the /24")
	}
}

// --- Non-overlapping ---

func TestIsSubset_NonOverlapping(t *testing.T) {
	subnetA := mustParseCIDR(t, "10.0.0.0/24")
	allowed := []net.IPNet{
		mustParseCIDR(t, "192.168.0.0/24"),
		mustParseCIDR(t, "172.16.0.0/16"),
	}

	if IsSubset(subnetA, allowed) {
		t.Error("expected false: subnetA does not overlap any allowed set")
	}
}

// --- Mixed address families (IPv4 subnetA with only IPv6 allowed and vice versa) ---

func TestIsSubset_MixedFamily_IPv4SubnetWithIPv6Allowed(t *testing.T) {
	subnetA := mustParseCIDR(t, "10.0.0.0/24")
	allowed := []net.IPNet{
		mustParseCIDR(t, "fd00::/64"),
	}

	if IsSubset(subnetA, allowed) {
		t.Error("expected false: IPv4 subnet cannot be a subset of IPv6 allowed sets")
	}
}

func TestIsSubset_MixedFamily_IPv6SubnetWithIPv4Allowed(t *testing.T) {
	subnetA := mustParseCIDR(t, "fd00::/64")
	allowed := []net.IPNet{
		mustParseCIDR(t, "10.0.0.0/8"),
	}

	if IsSubset(subnetA, allowed) {
		t.Error("expected false: IPv6 subnet cannot be a subset of IPv4 allowed sets")
	}
}

func TestIsSubset_MixedFamily_IPv4SubnetWithBothFamilies(t *testing.T) {
	// IPv4 subnetA with mixed allowed sets — should only consider IPv4 entries.
	subnetA := mustParseCIDR(t, "10.0.0.0/24")
	allowed := []net.IPNet{
		mustParseCIDR(t, "fd00::/64"),   // IPv6 — ignored
		mustParseCIDR(t, "10.0.0.0/16"), // IPv4 — covers subnetA
	}

	if !IsSubset(subnetA, allowed) {
		t.Error("expected true: IPv4 /24 is within the IPv4 /16 in allowed sets")
	}
}

func TestIsSubset_MixedFamily_IPv6SubnetWithBothFamilies(t *testing.T) {
	// IPv6 subnetA with mixed allowed sets — should only consider IPv6 entries.
	subnetA := mustParseCIDR(t, "fd00:1::/48")
	allowed := []net.IPNet{
		mustParseCIDR(t, "10.0.0.0/8"),  // IPv4 — ignored
		mustParseCIDR(t, "fd00:1::/32"), // IPv6 — covers subnetA
	}

	if !IsSubset(subnetA, allowed) {
		t.Error("expected true: IPv6 /48 is within the IPv6 /32 in allowed sets")
	}
}

func TestIsSubset_MixedFamily_IPv4NotCovered_IPV6EntriesIgnored(t *testing.T) {
	// IPv4 subnetA: IPv6 allowed entries are abundant but irrelevant.
	subnetA := mustParseCIDR(t, "10.0.0.0/24")
	allowed := []net.IPNet{
		mustParseCIDR(t, "fd00::/16"),     // IPv6 — ignored
		mustParseCIDR(t, "2001:db8::/32"), // IPv6 — ignored
		mustParseCIDR(t, "::1/128"),       // IPv6 — ignored
	}

	if IsSubset(subnetA, allowed) {
		t.Error("expected false: no IPv4 entries in allowed sets, IPv6 entries are ignored")
	}
}

func TestIsSubset_MixedFamily_IPv6NotCovered_IPV4EntriesIgnored(t *testing.T) {
	// IPv6 subnetA: IPv4 allowed entries are abundant but irrelevant.
	subnetA := mustParseCIDR(t, "fd00:abcd::/48")
	allowed := []net.IPNet{
		mustParseCIDR(t, "10.0.0.0/8"),     // IPv4 — ignored
		mustParseCIDR(t, "172.16.0.0/12"),  // IPv4 — ignored
		mustParseCIDR(t, "192.168.0.0/16"), // IPv4 — ignored
	}

	if IsSubset(subnetA, allowed) {
		t.Error("expected false: no IPv6 entries in allowed sets, IPv4 entries are ignored")
	}
}

func TestIsSubset_MixedFamily_BothFamiliesCoveredIndependently_IPv4Query(t *testing.T) {
	// allowedSets contains both IPv4 and IPv6 ranges that each fully cover
	// their respective family's subnetA. Querying with IPv4 subnetA —
	// only the IPv4 portion of allowedSets matters.
	subnetA := mustParseCIDR(t, "192.168.1.0/24")
	allowed := []net.IPNet{
		mustParseCIDR(t, "fd00::/16"),      // IPv6 — ignored
		mustParseCIDR(t, "fd00:1::/48"),    // IPv6 — ignored
		mustParseCIDR(t, "192.168.0.0/16"), // IPv4 — covers subnetA
	}

	if !IsSubset(subnetA, allowed) {
		t.Error("expected true: IPv4 /24 is covered by IPv4 /16 in allowed sets")
	}
}

func TestIsSubset_MixedFamily_BothFamiliesCoveredIndependently_IPv6Query(t *testing.T) {
	// Symmetric to the above: querying with IPv6 subnetA.
	subnetA := mustParseCIDR(t, "fd00:1:2::/48")
	allowed := []net.IPNet{
		mustParseCIDR(t, "10.0.0.0/8"),     // IPv4 — ignored
		mustParseCIDR(t, "192.168.0.0/16"), // IPv4 — ignored
		mustParseCIDR(t, "fd00:1::/32"),    // IPv6 — covers subnetA
	}

	if !IsSubset(subnetA, allowed) {
		t.Error("expected true: IPv6 /48 is covered by IPv6 /32 in allowed sets")
	}
}

func TestIsSubset_MixedFamily_ManyEntriesOnlyWrongFamily(t *testing.T) {
	// Large number of allowed entries, but all the wrong family.
	subnetA := mustParseCIDR(t, "10.0.0.0/24")
	allowed := []net.IPNet{
		mustParseCIDR(t, "fd00::/48"),
		mustParseCIDR(t, "fd00:1::/48"),
		mustParseCIDR(t, "fd00:2::/48"),
		mustParseCIDR(t, "2001:db8::/32"),
		mustParseCIDR(t, "2001:db8:1::/48"),
	}

	if IsSubset(subnetA, allowed) {
		t.Error("expected false: many IPv6 allowed entries but subnetA is IPv4")
	}
}

func TestIsSubset_MixedFamily_ManyEntriesOneCorrectFamily_IPv4(t *testing.T) {
	// Many IPv6 entries plus one IPv4 entry that covers subnetA.
	subnetA := mustParseCIDR(t, "192.168.5.0/24")
	allowed := []net.IPNet{
		mustParseCIDR(t, "fd00::/48"),
		mustParseCIDR(t, "fd00:1::/48"),
		mustParseCIDR(t, "fd00:2::/48"),
		mustParseCIDR(t, "192.168.0.0/16"), // the only IPv4 entry
	}

	if !IsSubset(subnetA, allowed) {
		t.Error("expected true: one IPv4 /16 among many IPv6 entries covers the IPv4 /24")
	}
}

func TestIsSubset_MixedFamily_ManyEntriesOneCorrectFamily_IPv6(t *testing.T) {
	// Many IPv4 entries plus one IPv6 entry that covers subnetA.
	subnetA := mustParseCIDR(t, "fd00:99::/48")
	allowed := []net.IPNet{
		mustParseCIDR(t, "10.0.0.0/8"),
		mustParseCIDR(t, "172.16.0.0/12"),
		mustParseCIDR(t, "192.168.0.0/16"),
		mustParseCIDR(t, "fd00::/16"), // the only IPv6 entry
	}

	if !IsSubset(subnetA, allowed) {
		t.Error("expected true: one IPv6 /16 among many IPv4 entries covers the IPv6 /48")
	}
}

func TestIsSubset_MixedFamily_CrossFamilyDoesNotHelp_IPv4Partial(t *testing.T) {
	// IPv4 subnetA is only partially covered by IPv4 allowed entries.
	// Having a large IPv6 range should not change the result.
	subnetA := mustParseCIDR(t, "10.0.0.0/23")
	allowed := []net.IPNet{
		mustParseCIDR(t, "fd00::/8"),    // IPv6 — huge but irrelevant
		mustParseCIDR(t, "10.0.0.0/24"), // IPv4 — only covers half
	}

	if IsSubset(subnetA, allowed) {
		t.Error("expected false: IPv6 range doesn't help cover the missing IPv4 half")
	}
}

func TestIsSubset_MixedFamily_CrossFamilyDoesNotHelp_IPv6Partial(t *testing.T) {
	// IPv6 subnetA is only partially covered by IPv6 allowed entries.
	// Having a large IPv4 range should not change the result.
	subnetA := mustParseCIDR(t, "fd00:1::/48")
	allowed := []net.IPNet{
		mustParseCIDR(t, "10.0.0.0/8"),         // IPv4 — huge but irrelevant
		mustParseCIDR(t, "fd00:1:0:ffff::/64"), // IPv6 — only covers a /64 slice
	}

	if IsSubset(subnetA, allowed) {
		t.Error("expected false: IPv4 range doesn't help cover the missing IPv6 portion")
	}
}

func TestIsSubset_MixedFamily_InterleavedOrder(t *testing.T) {
	// IPv4 and IPv6 allowed entries are interleaved; ordering must not matter.
	subnetA := mustParseCIDR(t, "172.16.10.0/24")
	allowed := []net.IPNet{
		mustParseCIDR(t, "fd00::/32"),     // IPv6
		mustParseCIDR(t, "172.16.0.0/16"), // IPv4 — covers subnetA
		mustParseCIDR(t, "2001:db8::/32"), // IPv6
		mustParseCIDR(t, "10.0.0.0/8"),    // IPv4 — doesn't cover
	}

	if !IsSubset(subnetA, allowed) {
		t.Error("expected true: IPv4 /24 covered by /16 despite interleaved IPv6 entries")
	}
}

func TestIsSubset_MixedFamily_ConsecutiveIPv4WithIPv6Noise(t *testing.T) {
	// Two consecutive IPv4 /24s that merge to cover a /23,
	// with IPv6 entries interspersed that must be ignored.
	subnetA := mustParseCIDR(t, "192.168.0.0/23")
	allowed := []net.IPNet{
		mustParseCIDR(t, "fd00:1::/64"),    // IPv6 noise
		mustParseCIDR(t, "192.168.0.0/24"), // IPv4 — lower half
		mustParseCIDR(t, "fd00:2::/64"),    // IPv6 noise
		mustParseCIDR(t, "192.168.1.0/24"), // IPv4 — upper half
	}

	if !IsSubset(subnetA, allowed) {
		t.Error("expected true: two IPv4 /24s merge to cover /23, IPv6 entries ignored")
	}
}

func TestIsSubset_MixedFamily_ConsecutiveIPv6WithIPv4Noise(t *testing.T) {
	// Two consecutive IPv6 /128s that merge to cover a /127,
	// with IPv4 entries interspersed that must be ignored.
	subnetA := mustParseCIDR(t, "fd00:99::/127")
	allowed := []net.IPNet{
		mustParseCIDR(t, "10.0.0.0/24"),    // IPv4 noise
		mustParseCIDR(t, "fd00:99::0/128"), // IPv6
		mustParseCIDR(t, "192.168.0.0/24"), // IPv4 noise
		mustParseCIDR(t, "fd00:99::1/128"), // IPv6
	}

	if !IsSubset(subnetA, allowed) {
		t.Error("expected true: two IPv6 /128s merge to cover /127, IPv4 entries ignored")
	}
}

func TestIsSubset_MixedFamily_SingleIPv4HostWithMixedAllowed(t *testing.T) {
	// Single IPv4 /32 host, allowedSets has both families.
	subnetA := mustParseCIDR(t, "10.0.0.42/32")
	allowed := []net.IPNet{
		mustParseCIDR(t, "fd00::/64"),
		mustParseCIDR(t, "2001:db8::/32"),
		mustParseCIDR(t, "10.0.0.0/24"), // IPv4 — covers the host
	}

	if !IsSubset(subnetA, allowed) {
		t.Error("expected true: single IPv4 host is within the IPv4 /24 allowed entry")
	}
}

func TestIsSubset_MixedFamily_SingleIPv6HostWithMixedAllowed(t *testing.T) {
	// Single IPv6 /128 host, allowedSets has both families.
	subnetA := mustParseCIDR(t, "fd00:dead:beef::1/128")
	allowed := []net.IPNet{
		mustParseCIDR(t, "10.0.0.0/8"),
		mustParseCIDR(t, "172.16.0.0/12"),
		mustParseCIDR(t, "fd00:dead:beef::/64"), // IPv6 — covers the host
	}

	if !IsSubset(subnetA, allowed) {
		t.Error("expected true: single IPv6 host is within the IPv6 /64 allowed entry")
	}
}

func TestIsSubset_MixedFamily_IPv4NotCovered_OnlyIPv4IsWrongSubnet(t *testing.T) {
	// IPv4 subnetA, allowedSets has both families, but the IPv4 entry covers a different subnet.
	subnetA := mustParseCIDR(t, "10.1.0.0/24")
	allowed := []net.IPNet{
		mustParseCIDR(t, "fd00::/32"),      // IPv6 — ignored
		mustParseCIDR(t, "192.168.0.0/16"), // IPv4 — wrong subnet
	}

	if IsSubset(subnetA, allowed) {
		t.Error("expected false: the only IPv4 entry covers a different subnet")
	}
}

func TestIsSubset_MixedFamily_IPv6NotCovered_OnlyIPv6IsWrongSubnet(t *testing.T) {
	// IPv6 subnetA, allowedSets has both families, but the IPv6 entry covers a different subnet.
	subnetA := mustParseCIDR(t, "fd00:1::/48")
	allowed := []net.IPNet{
		mustParseCIDR(t, "10.0.0.0/8"),  // IPv4 — ignored
		mustParseCIDR(t, "fd00:2::/48"), // IPv6 — wrong subnet
	}

	if IsSubset(subnetA, allowed) {
		t.Error("expected false: the only IPv6 entry covers a different subnet")
	}
}

// --- Empty allowed sets ---

func TestIsSubset_EmptyAllowed(t *testing.T) {
	subnetA := mustParseCIDR(t, "10.0.0.0/24")

	if IsSubset(subnetA, nil) {
		t.Error("expected false: nil allowed sets")
	}

	if IsSubset(subnetA, []net.IPNet{}) {
		t.Error("expected false: empty allowed sets")
	}
}

// --- Single host (/32 for IPv4, /128 for IPv6) ---

func TestIsSubset_SingleHost_IPv4(t *testing.T) {
	subnetA := mustParseCIDR(t, "10.0.0.5/32")
	allowed := []net.IPNet{mustParseCIDR(t, "10.0.0.0/24")}

	if !IsSubset(subnetA, allowed) {
		t.Error("expected true: single host /32 is within /24")
	}
}

func TestIsSubset_SingleHost_IPv6(t *testing.T) {
	subnetA := mustParseCIDR(t, "fd00::1/128")
	allowed := []net.IPNet{mustParseCIDR(t, "fd00::/64")}

	if !IsSubset(subnetA, allowed) {
		t.Error("expected true: single host /128 is within /64")
	}
}

func TestIsSubset_SingleHost_NotInAllowed(t *testing.T) {
	subnetA := mustParseCIDR(t, "10.0.0.5/32")
	allowed := []net.IPNet{mustParseCIDR(t, "10.0.1.0/24")}

	if IsSubset(subnetA, allowed) {
		t.Error("expected false: single host not in allowed /24")
	}
}

// --- Consecutive allowed ranges that merge ---

func TestIsSubset_ConsecutiveRanges_Merge(t *testing.T) {
	// subnetA: 10.0.0.0/23 => 10.0.0.0 – 10.0.1.255
	// allowed: 10.0.0.0/24 and 10.0.1.0/24 are consecutive; they merge into /23
	subnetA := mustParseCIDR(t, "10.0.0.0/23")
	allowed := []net.IPNet{
		mustParseCIDR(t, "10.0.0.0/24"),
		mustParseCIDR(t, "10.0.1.0/24"),
	}

	if !IsSubset(subnetA, allowed) {
		t.Error("expected true: consecutive /24s merge to cover /23")
	}
}

func TestIsSubset_ConsecutiveRanges_IPv6(t *testing.T) {
	// fd00::/127 => fd00:: and fd00::1
	// Two /128s that are consecutive
	subnetA := mustParseCIDR(t, "fd00::/127")
	allowed := []net.IPNet{
		mustParseCIDR(t, "fd00::0/128"),
		mustParseCIDR(t, "fd00::1/128"),
	}

	if !IsSubset(subnetA, allowed) {
		t.Error("expected true: two consecutive /128s merge to cover /127")
	}
}

// --- Partial overlap (subnetA partially overlaps allowed but is not fully covered) ---

func TestIsSubset_PartialOverlap(t *testing.T) {
	// subnetA: 10.0.0.0/23 => 10.0.0.0 – 10.0.1.255
	// allowed: 10.0.1.0/24 => 10.0.1.0 – 10.0.1.255 (only upper half)
	subnetA := mustParseCIDR(t, "10.0.0.0/23")
	allowed := []net.IPNet{
		mustParseCIDR(t, "10.0.1.0/24"),
	}

	if IsSubset(subnetA, allowed) {
		t.Error("expected false: /24 only covers upper half of /23")
	}
}

func TestIsSubset_PartialOverlap_LowerHalf(t *testing.T) {
	// subnetA: 10.0.0.0/23 => 10.0.0.0 – 10.0.1.255
	// allowed: 10.0.0.0/24 => 10.0.0.0 – 10.0.0.255 (only lower half)
	subnetA := mustParseCIDR(t, "10.0.0.0/23")
	allowed := []net.IPNet{
		mustParseCIDR(t, "10.0.0.0/24"),
	}

	if IsSubset(subnetA, allowed) {
		t.Error("expected false: /24 only covers lower half of /23")
	}
}

func TestIsSubset_PartialOverlap_TwoRangesWithGap(t *testing.T) {
	// subnetA: 10.0.0.0/22 => 10.0.0.0 – 10.0.3.255
	// allowed: 10.0.0.0/24 and 10.0.2.0/24 — gap at 10.0.1.0/24
	subnetA := mustParseCIDR(t, "10.0.0.0/22")
	allowed := []net.IPNet{
		mustParseCIDR(t, "10.0.0.0/24"),
		mustParseCIDR(t, "10.0.2.0/24"),
	}

	if IsSubset(subnetA, allowed) {
		t.Error("expected false: gap at 10.0.1.0/24 means /22 is not fully covered")
	}
}

// --- SubnetA is broader than any individual allowed set ---

func TestIsSubset_SubnetABroaderThanAnyAllowed(t *testing.T) {
	// subnetA: 10.0.0.0/16
	// allowed: 10.0.0.0/24 (much smaller)
	subnetA := mustParseCIDR(t, "10.0.0.0/16")
	allowed := []net.IPNet{
		mustParseCIDR(t, "10.0.0.0/24"),
	}

	if IsSubset(subnetA, allowed) {
		t.Error("expected false: /16 is broader than /24")
	}
}

// --- SubnetA exactly at the boundary of an allowed set ---

func TestIsSubset_BoundaryExact(t *testing.T) {
	subnetA := mustParseCIDR(t, "10.0.0.255/32")
	allowed := []net.IPNet{mustParseCIDR(t, "10.0.0.0/24")}

	if !IsSubset(subnetA, allowed) {
		t.Error("expected true: last host of /24 is within /24")
	}
}

func TestIsSubset_BoundaryJustOutside(t *testing.T) {
	subnetA := mustParseCIDR(t, "10.0.1.0/32")
	allowed := []net.IPNet{mustParseCIDR(t, "10.0.0.0/24")}

	if IsSubset(subnetA, allowed) {
		t.Error("expected false: 10.0.1.0 is just outside 10.0.0.0/24")
	}
}

// --- Large IPv6 ---

func TestIsSubset_IPv6_LargeSubnet(t *testing.T) {
	subnetA := mustParseCIDR(t, "2001:db8::/32")
	allowed := []net.IPNet{mustParseCIDR(t, "2001:db8::/16")}

	if !IsSubset(subnetA, allowed) {
		t.Error("expected true: 2001:db8::/32 is within 2001:db8::/16")
	}
}

func TestIsSubset_IPv6_NotSubset(t *testing.T) {
	subnetA := mustParseCIDR(t, "2001:db8:abcd::/48")
	allowed := []net.IPNet{mustParseCIDR(t, "2001:db8:1234::/48")}

	if IsSubset(subnetA, allowed) {
		t.Error("expected false: different /48 prefixes")
	}
}
