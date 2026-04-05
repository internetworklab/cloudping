package utils

import (
	"net"
	"testing"
)

// helper to parse an IP string into a net.IP.
func mustParseIP(t *testing.T, s string) net.IP {
	t.Helper()
	ip := net.ParseIP(s)
	if ip == nil {
		t.Fatalf("failed to parse IP %q", s)
	}
	return ip
}

func TestGetOffset_IPv4(t *testing.T) {
	tests := []struct {
		name    string
		network string
		host    string
		want    uint64
	}{
		{
			name:    "doc example: /24 offset 3",
			network: "1.0.0.0/24",
			host:    "1.0.0.3",
			want:    3,
		},
		{
			name:    "offset zero: host equals network",
			network: "10.0.0.0/24",
			host:    "10.0.0.0",
			want:    0,
		},
		{
			name:    "/24 last host",
			network: "192.168.1.0/24",
			host:    "192.168.1.255",
			want:    255,
		},
		{
			name:    "/16 offset in third octet",
			network: "10.0.0.0/16",
			host:    "10.0.5.0",
			want:    5 << 8,
		},
		{
			name:    "/16 large offset",
			network: "10.0.0.0/16",
			host:    "10.0.2.3",
			want:    (2 << 8) | 3,
		},
		{
			name:    "/8 full range",
			network: "10.0.0.0/8",
			host:    "10.0.1.2",
			want:    (0 << 16) | (1 << 8) | 2,
		},
		{
			name:    "/0 entire address space",
			network: "0.0.0.0/0",
			host:    "1.2.3.4",
			want:    (1 << 24) | (2 << 16) | (3 << 8) | 4,
		},
		{
			name:    "/32 single host subnet",
			network: "10.0.0.1/32",
			host:    "10.0.0.1",
			want:    0,
		},
		{
			name:    "/30 offset 2",
			network: "192.168.0.0/30",
			host:    "192.168.0.2",
			want:    2,
		},
		{
			name:    "host at 127.0.0.1 in /24",
			network: "127.0.0.0/24",
			host:    "127.0.0.1",
			want:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			network := mustParseCIDR(t, tt.network)
			host := mustParseIP(t, tt.host)
			got := GetOffset(network, host)
			if got != tt.want {
				t.Errorf("GetOffset(%s, %s) = %d, want %d", tt.network, tt.host, got, tt.want)
			}
		})
	}
}

func TestGetOffset_IPv6(t *testing.T) {
	tests := []struct {
		name    string
		network string
		host    string
		want    uint64
	}{
		{
			name:    "doc example: /64 offset 7",
			network: "fd00::/64",
			host:    "fd00::7",
			want:    7,
		},
		{
			name:    "offset zero: host equals network",
			network: "fd00::/64",
			host:    "fd00::",
			want:    0,
		},
		{
			name:    "/64 small offset",
			network: "2001:db8::/64",
			host:    "2001:db8::1",
			want:    1,
		},
		{
			name:    "/64 larger offset",
			network: "2001:db8::/64",
			host:    "2001:db8::100",
			want:    0x100,
		},
		{
			name:    "/64 offset across hextets",
			network: "fd00::/64",
			host:    "fd00::1:0:0:1",
			want:    (1 << 48) | 1,
		},
		{
			name:    "/128 single host subnet",
			network: "fd00::1/128",
			host:    "fd00::1",
			want:    0,
		},
		{
			name:    "/120 offset in last byte",
			network: "fd00::/120",
			host:    "fd00::ff",
			want:    0xff,
		},
		{
			name:    "/112 offset in last two bytes",
			network: "fd00::/112",
			host:    "fd00::102",
			want:    (0x01 << 8) | 0x02,
		},
		{
			name:    "/65 offset across multiple hextets",
			network: "fd00::/65",
			host:    "fd00::abcd:ef01",
			want:    0xABCDEF01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			network := mustParseCIDR(t, tt.network)
			host := mustParseIP(t, tt.host)
			got := GetOffset(network, host)
			if got != tt.want {
				t.Errorf("GetOffset(%s, %s) = %d, want %d", tt.network, tt.host, got, tt.want)
			}
		})
	}
}

func TestGetOffset_IPv6ShortPrefixPanics(t *testing.T) {
	tests := []struct {
		name    string
		network string
		host    string
	}{
		{
			name:    "/63 should panic",
			network: "fd00::/63",
			host:    "fd00::1",
		},
		{
			name:    "/48 should panic",
			network: "fd00::/48",
			host:    "fd00::1",
		},
		{
			name:    "/0 should panic",
			network: "::/0",
			host:    "fd00::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			network := mustParseCIDR(t, tt.network)
			host := mustParseIP(t, tt.host)

			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("GetOffset(%s, %s) expected panic, but did not panic", tt.network, tt.host)
				}
			}()

			GetOffset(network, host)
		})
	}
}
