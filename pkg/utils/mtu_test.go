package utils

import "testing"

func TestMaxIPPacketLen_IsMaxIPPacketSize(t *testing.T) {
	// 65535 is the maximum size of an IP (or IPv6) packet, header included
	if MaxIPPacketLen != 65535 {
		t.Fatalf("expected MaxIPPacketLen to be 65535, got %d", MaxIPPacketLen)
	}
}

func TestGetMaximumMTU_Positive(t *testing.T) {
	mtu := GetMaximumMTU()
	if mtu <= 0 {
		t.Fatalf("expected a positive maximum MTU, got %d", mtu)
	}
}
