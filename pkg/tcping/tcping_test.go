package tcping

import (
	"net"
	"testing"
)

func TestGetSrcIP_IPv4Loopback(t *testing.T) {
	dstIP := net.ParseIP("127.0.0.1")
	if dstIP == nil {
		t.Fatal("failed to parse 127.0.0.1")
	}

	srcIP, err := getSrcIP(dstIP)
	if err != nil {
		t.Fatalf("getSrcIP(%s) returned error: %v", dstIP, err)
	}
	if !srcIP.Equal(dstIP) {
		t.Fatalf("getSrcIP(%s) = %s, expected %s", dstIP, srcIP, dstIP)
	}
}

func TestGetSrcIP_IPv6Loopback(t *testing.T) {
	dstIP := net.ParseIP("::1")
	if dstIP == nil {
		t.Fatal("failed to parse ::1")
	}

	srcIP, err := getSrcIP(dstIP)
	if err != nil {
		t.Skipf("IPv6 loopback seems unavailable on this host: %v", err)
	}
	if !srcIP.Equal(dstIP) {
		t.Fatalf("getSrcIP(%s) = %s, expected %s", dstIP, srcIP, dstIP)
	}
}
