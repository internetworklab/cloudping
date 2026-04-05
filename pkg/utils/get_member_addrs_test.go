package utils

import (
	"context"
	"math/big"
	"net"
	"slices"
	"testing"
)

// mustParseCIDR is a test helper that parses a CIDR string or fatals the test.
func mustParseCIDR(t *testing.T, cidr string) net.IPNet {
	t.Helper()
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("failed to parse CIDR %q: %v", cidr, err)
	}
	return *ipNet
}

// collect drains all IPs from the channel and returns them as a slice.
func collect(t *testing.T, ch <-chan net.IP) []net.IP {
	t.Helper()
	var addrs []net.IP
	for ip := range ch {
		addrs = append(addrs, ip)
	}
	return addrs
}

// --- IPv4 tests ---

func TestGetMemberAddresses32_IPv4_Slash30(t *testing.T) {
	// 10.1.2.0/30 → 10.1.2.0, 10.1.2.1, 10.1.2.2, 10.1.2.3
	ipNet := mustParseCIDR(t, "10.1.2.0/30")
	addrs := collect(t, GetMemberAddresses32(context.Background(), ipNet))

	want := []string{
		"10.1.2.0",
		"10.1.2.1",
		"10.1.2.2",
		"10.1.2.3",
	}

	if len(addrs) != len(want) {
		t.Fatalf("got %d addresses, want %d", len(addrs), len(want))
	}

	for i, addr := range addrs {
		got := addr.String()
		if got != want[i] {
			t.Errorf("address[%d]: got %s, want %s", i, got, want[i])
		}
	}
}

func TestGetMemberAddresses32_IPv4_Slash32(t *testing.T) {
	// Single-host subnet → exactly one address.
	ipNet := mustParseCIDR(t, "192.168.13.103/32")
	addrs := collect(t, GetMemberAddresses32(context.Background(), ipNet))

	if len(addrs) != 1 {
		t.Fatalf("got %d addresses, want 1", len(addrs))
	}
	if addrs[0].String() != "192.168.13.103" {
		t.Errorf("got %s, want 192.168.13.103", addrs[0])
	}
}

func TestGetMemberAddresses32_IPv4_Slash28(t *testing.T) {
	// 172.20.0.0/28 → 172.20.0.0 … 172.20.0.15 (16 addresses).
	ipNet := mustParseCIDR(t, "172.20.0.0/28")
	addrs := collect(t, GetMemberAddresses32(context.Background(), ipNet))

	if len(addrs) != 16 {
		t.Fatalf("got %d addresses, want 16", len(addrs))
	}

	for i, addr := range addrs {
		expected := net.ParseIP("172.20.0.0").To4()
		expected[3] = byte(i)
		if !addr.Equal(expected) {
			t.Errorf("address[%d]: got %s, want %s", i, addr, expected)
		}
	}
}

func TestGetMemberAddresses32_IPv4_Slash24(t *testing.T) {
	// /24 → 256 addresses, verify count and first/last.
	ipNet := mustParseCIDR(t, "192.168.0.0/24")
	addrs := collect(t, GetMemberAddresses32(context.Background(), ipNet))

	if len(addrs) != 256 {
		t.Fatalf("got %d addresses, want 256", len(addrs))
	}

	first := net.ParseIP("192.168.0.0")
	last := net.ParseIP("192.168.0.255")

	if !addrs[0].Equal(first) {
		t.Errorf("first address: got %s, want %s", addrs[0], first)
	}
	if !addrs[255].Equal(last) {
		t.Errorf("last address: got %s, want %s", addrs[255], last)
	}
}

// --- IPv6 tests ---

func TestGetMemberAddresses32_IPv6_Slash126(t *testing.T) {
	// fd00::/126 → fd00::, fd00::1, fd00::2, fd00::3
	ipNet := mustParseCIDR(t, "fd00::/126")
	addrs := collect(t, GetMemberAddresses32(context.Background(), ipNet))

	want := []string{
		"fd00::",
		"fd00::1",
		"fd00::2",
		"fd00::3",
	}

	if len(addrs) != len(want) {
		t.Fatalf("got %d addresses, want %d", len(addrs), len(want))
	}

	for i, addr := range addrs {
		got := addr.String()
		if got != want[i] {
			t.Errorf("address[%d]: got %s, want %s", i, got, want[i])
		}
	}
}

func TestGetMemberAddresses32_IPv6_Slash128(t *testing.T) {
	// Single-host IPv6 subnet → exactly one address.
	ipNet := mustParseCIDR(t, "fd00::1/128")
	addrs := collect(t, GetMemberAddresses32(context.Background(), ipNet))

	if len(addrs) != 1 {
		t.Fatalf("got %d addresses, want 1", len(addrs))
	}
	if addrs[0].String() != "fd00::1" {
		t.Errorf("got %s, want fd00::1", addrs[0])
	}
}

func TestGetMemberAddresses32_IPv6_Slash120(t *testing.T) {
	// fd00::/120 → fd00:: through fd00::ff (256 addresses).
	ipNet := mustParseCIDR(t, "fd00::/120")
	addrs := collect(t, GetMemberAddresses32(context.Background(), ipNet))

	if len(addrs) != 256 {
		t.Fatalf("got %d addresses, want 256", len(addrs))
	}

	first := net.ParseIP("fd00::")
	last := net.ParseIP("fd00::ff")

	if !addrs[0].Equal(first) {
		t.Errorf("first address: got %s, want %s", addrs[0], first)
	}
	if !addrs[255].Equal(last) {
		t.Errorf("last address: got %s, want %s", addrs[255], last)
	}
}

func TestGetMemberAddresses32_IPv6_Slash96(t *testing.T) {
	// fd00:0:0:1::/96 → 2^32 addresses; verify first few and last.
	// We don't collect all 4 billion, so instead range over the channel
	// and spot-check the first address, then drain the rest counting total.
	//
	// That would be too slow, so instead let's just verify a /96 with a
	// non-zero prefix produces the correct first address by reading one value.
	//
	// Actually, /96 produces 2^32 addresses which is far too many to collect.
	// Instead we'll just read the first value and verify it.
	ipNet := mustParseCIDR(t, "fd00:0:0:1::/96")
	ch := GetMemberAddresses32(context.Background(), ipNet)

	first := <-ch
	if !first.Equal(net.ParseIP("fd00:0:0:1::")) {
		t.Errorf("first address: got %s, want fd00:0:0:1::", first)
	}
}

// --- Property / correctness tests ---

func TestGetMemberAddresses32_ChannelCloses(t *testing.T) {
	// After all addresses are emitted, the channel must be closed so that
	// `for range ch` terminates naturally.
	ipNet := mustParseCIDR(t, "10.0.0.0/30")
	ch := GetMemberAddresses32(context.Background(), ipNet)

	count := 0
	for range ch {
		count++
	}
	// If the channel were not closed, this line would never be reached.
	if count != 4 {
		t.Errorf("got %d addresses, want 4", count)
	}
}

func TestGetMemberAddresses32_FreshAllocations(t *testing.T) {
	// Each net.IP must be an independent allocation so that retaining
	// references doesn't cause aliasing bugs.
	ipNet := mustParseCIDR(t, "10.0.0.0/30")
	addrs := collect(t, GetMemberAddresses32(context.Background(), ipNet))

	// All underlying byte slices must be distinct pointers.
	for i := 0; i < len(addrs); i++ {
		for j := i + 1; j < len(addrs); j++ {
			if &addrs[i][0] == &addrs[j][0] {
				t.Errorf("address[%d] and address[%d] share the same backing array", i, j)
			}
		}
	}
}

func TestGetMemberAddresses32_SequentialOrder(t *testing.T) {
	// Addresses must come out in ascending numerical order.
	ipNet := mustParseCIDR(t, "10.0.0.0/28")
	addrs := collect(t, GetMemberAddresses32(context.Background(), ipNet))

	for i := 1; i < len(addrs); i++ {
		prev := big.NewInt(0).SetBytes(addrs[i-1])
		curr := big.NewInt(0).SetBytes(addrs[i])
		if curr.Cmp(prev) <= 0 {
			t.Errorf("address[%d]=%s is not greater than address[%d]=%s", i, addrs[i], i-1, addrs[i-1])
		}
	}
}

func TestGetMemberAddresses32_IPv4_AllWithinSubnet(t *testing.T) {
	// Every emitted address must belong to the original subnet.
	ipNet := mustParseCIDR(t, "192.168.1.0/28")
	addrs := collect(t, GetMemberAddresses32(context.Background(), ipNet))

	for i, addr := range addrs {
		if !ipNet.Contains(addr) {
			t.Errorf("address[%d] %s is not contained in %s", i, addr, ipNet.String())
		}
	}
}

func TestGetMemberAddresses32_IPv6_AllWithinSubnet(t *testing.T) {
	ipNet := mustParseCIDR(t, "fd00::/120")
	addrs := collect(t, GetMemberAddresses32(context.Background(), ipNet))

	for i, addr := range addrs {
		if !ipNet.Contains(addr) {
			t.Errorf("address[%d] %s is not contained in %s", i, addr, ipNet.String())
		}
	}
}

func TestGetMemberAddresses32_NoDuplicates(t *testing.T) {
	// No address should appear more than once.
	ipNet := mustParseCIDR(t, "10.0.0.0/28")
	addrs := collect(t, GetMemberAddresses32(context.Background(), ipNet))

	seen := make(map[string]bool, len(addrs))
	for _, addr := range addrs {
		key := addr.String()
		if seen[key] {
			t.Errorf("duplicate address: %s", key)
		}
		seen[key] = true
	}
}

func TestGetMemberAddresses32_IPv4_NonZeroHost(t *testing.T) {
	// Starting from a non-network address should still mask correctly.
	// 10.1.2.99/30 masks to 10.1.2.96, producing .96 .97 .98 .99
	ipNet := mustParseCIDR(t, "10.1.2.99/30")
	addrs := collect(t, GetMemberAddresses32(context.Background(), ipNet))

	want := []string{
		"10.1.2.96",
		"10.1.2.97",
		"10.1.2.98",
		"10.1.2.99",
	}

	if len(addrs) != len(want) {
		t.Fatalf("got %d addresses, want %d", len(addrs), len(want))
	}

	for i, addr := range addrs {
		if addr.String() != want[i] {
			t.Errorf("address[%d]: got %s, want %s", i, addr, want[i])
		}
	}
}

func TestGetMemberAddresses32_IPv4_SlicesSorted(t *testing.T) {
	// Quick sanity that the output is sorted using slices.CompareFunc.
	ipNet := mustParseCIDR(t, "172.20.0.0/28")
	addrs := collect(t, GetMemberAddresses32(context.Background(), ipNet))

	if !slices.IsSortedFunc(addrs, func(a, b net.IP) int {
		return big.NewInt(0).SetBytes(a).Cmp(big.NewInt(0).SetBytes(b))
	}) {
		t.Error("addresses are not sorted in ascending order")
	}
}
