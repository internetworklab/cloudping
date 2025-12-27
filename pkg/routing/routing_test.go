package routing

import (
	"testing"
)

func TestRoute_Less(t *testing.T) {
	tests := []struct {
		name     string
		route1   *Route
		route2   *Route
		expected bool
	}{
		{
			name:     "route1 prefix less than route2",
			route1:   &Route{Prefix: []byte{192, 168, 1, 0}},
			route2:   &Route{Prefix: []byte{192, 168, 2, 0}},
			expected: true,
		},
		{
			name:     "route1 prefix greater than route2",
			route1:   &Route{Prefix: []byte{192, 168, 2, 0}},
			route2:   &Route{Prefix: []byte{192, 168, 1, 0}},
			expected: false,
		},
		{
			name:     "route1 prefix equal to route2",
			route1:   &Route{Prefix: []byte{192, 168, 1, 0}},
			route2:   &Route{Prefix: []byte{192, 168, 1, 0}},
			expected: false,
		},
		{
			name:     "IPv6 comparison",
			route1:   &Route{Prefix: []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}},
			route2:   &Route{Prefix: []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.route1.Less(tt.route2)
			if result != tt.expected {
				t.Errorf("Route.Less() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestRoute_Less_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when comparing Route with non-Route")
		}
	}()

	route := &Route{Prefix: []byte{192, 168, 1, 0}}
	route.Less(&RouteGroup{PrefixLen: 24})
}

func TestRouteGroup_Less(t *testing.T) {
	tests := []struct {
		name     string
		group1   *RouteGroup
		group2   *RouteGroup
		expected bool
	}{
		{
			name:     "group1 prefix length less than group2",
			group1:   &RouteGroup{PrefixLen: 24},
			group2:   &RouteGroup{PrefixLen: 32},
			expected: true,
		},
		{
			name:     "group1 prefix length greater than group2",
			group1:   &RouteGroup{PrefixLen: 32},
			group2:   &RouteGroup{PrefixLen: 24},
			expected: false,
		},
		{
			name:     "group1 prefix length equal to group2",
			group1:   &RouteGroup{PrefixLen: 24},
			group2:   &RouteGroup{PrefixLen: 24},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.group1.Less(tt.group2)
			if result != tt.expected {
				t.Errorf("RouteGroup.Less() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestRouteGroup_Less_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when comparing RouteGroup with non-RouteGroup")
		}
	}()

	group := &RouteGroup{PrefixLen: 24}
	group.Less(&Route{Prefix: []byte{192, 168, 1, 0}})
}

func TestNewRouteGroup(t *testing.T) {
	group := NewRouteGroup()
	if group == nil {
		t.Fatal("NewRouteGroup() returned nil")
	}
	if group.Routes == nil {
		t.Error("NewRouteGroup() Routes is nil")
	}
	if group.PrefixLen != 0 {
		t.Errorf("NewRouteGroup() PrefixLen = %d, expected 0", group.PrefixLen)
	}
}

func TestNewSimpleRouter(t *testing.T) {
	router := NewSimpleRouter()
	if router == nil {
		t.Fatal("NewSimpleRouter() returned nil")
	}
	if router.routes == nil {
		t.Error("NewSimpleRouter() routes is nil")
	}
	if router.routes6 == nil {
		t.Error("NewSimpleRouter() routes6 is nil")
	}
}

func TestSimpleRouter_AddRoute_IPv4(t *testing.T) {
	router := NewSimpleRouter()

	tests := []struct {
		name    string
		cidr    string
		value   interface{}
		wantErr bool
	}{
		{
			name:    "valid IPv4 CIDR",
			cidr:    "192.168.1.0/24",
			value:   "route1",
			wantErr: false,
		},
		{
			name:    "valid IPv4 single host",
			cidr:    "10.0.0.1/32",
			value:   "route2",
			wantErr: false,
		},
		{
			name:    "valid IPv4 default route",
			cidr:    "0.0.0.0/0",
			value:   "default",
			wantErr: false,
		},
		{
			name:    "invalid CIDR",
			cidr:    "invalid",
			value:   "route3",
			wantErr: true,
		},
		{
			name:    "invalid CIDR format",
			cidr:    "192.168.1.0",
			value:   "route4",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := router.AddRoute(tt.cidr, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SimpleRouter.AddRoute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSimpleRouter_AddRoute_IPv6(t *testing.T) {
	router := NewSimpleRouter()

	tests := []struct {
		name    string
		cidr    string
		value   interface{}
		wantErr bool
	}{
		{
			name:    "valid IPv6 CIDR",
			cidr:    "2001:db8::/32",
			value:   "route1",
			wantErr: false,
		},
		{
			name:    "valid IPv6 single host",
			cidr:    "2001:db8::1/128",
			value:   "route2",
			wantErr: false,
		},
		{
			name:    "valid IPv6 default route",
			cidr:    "::/0",
			value:   "default6",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := router.AddRoute(tt.cidr, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SimpleRouter.AddRoute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSimpleRouter_GetRoute_IPv4(t *testing.T) {
	router := NewSimpleRouter()

	// Add some routes
	routes := map[string]interface{}{
		"192.168.1.0/24": "network1",
		"192.168.1.1/32": "host1",
		"10.0.0.0/8":     "network2",
		"0.0.0.0/0":      "default",
	}

	for cidr, value := range routes {
		if err := router.AddRoute(cidr, value); err != nil {
			t.Fatalf("Failed to add route %s: %v", cidr, err)
		}
	}

	tests := []struct {
		name     string
		ip       string
		expected interface{}
		wantErr  bool
	}{
		{
			name:     "exact match /32",
			ip:       "192.168.1.1",
			expected: "host1",
			wantErr:  false,
		},
		{
			name:     "match within /24 network",
			ip:       "192.168.1.100",
			expected: "network1",
			wantErr:  false,
		},
		{
			name:     "match within /8 network",
			ip:       "10.1.2.3",
			expected: "network2",
			wantErr:  false,
		},
		{
			name:     "match default route",
			ip:       "172.16.0.1",
			expected: "default",
			wantErr:  false,
		},
		{
			name:     "no match - different network",
			ip:       "192.168.2.1",
			expected: "default", // Should match default route since 192.168.2.1 is not in 192.168.1.0/24
			wantErr:  false,
		},
		{
			name:     "invalid IP",
			ip:       "invalid",
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := router.GetRoute(tt.ip)
			if (err != nil) != tt.wantErr {
				t.Errorf("SimpleRouter.GetRoute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("SimpleRouter.GetRoute() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestSimpleRouter_GetRoute_IPv6(t *testing.T) {
	router := NewSimpleRouter()

	// Add some IPv6 routes
	routes := map[string]interface{}{
		"2001:db8::/32":   "network1",
		"2001:db8::1/128": "host1",
		"2001:db8:1::/48": "network2",
		"::/0":            "default6",
	}

	for cidr, value := range routes {
		if err := router.AddRoute(cidr, value); err != nil {
			t.Fatalf("Failed to add route %s: %v", cidr, err)
		}
	}

	tests := []struct {
		name     string
		ip       string
		expected interface{}
		wantErr  bool
	}{
		{
			name:     "exact match /128",
			ip:       "2001:db8::1",
			expected: "host1",
			wantErr:  false,
		},
		{
			name:     "match within /32 network",
			ip:       "2001:db8::100",
			expected: "network1",
			wantErr:  false,
		},
		{
			name:     "match within /48 network",
			ip:       "2001:db8:1::100",
			expected: "network2",
			wantErr:  false,
		},
		{
			name:     "match default route",
			ip:       "2001:db9::1", // Different network prefix, should match default
			expected: "default6",
			wantErr:  false,
		},
		{
			name:     "invalid IPv6",
			ip:       "invalid",
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := router.GetRoute(tt.ip)
			if (err != nil) != tt.wantErr {
				t.Errorf("SimpleRouter.GetRoute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("SimpleRouter.GetRoute() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestSimpleRouter_LongestPrefixMatch(t *testing.T) {
	router := NewSimpleRouter()

	// Add routes with different prefix lengths
	// The longest matching prefix should be returned
	routes := map[string]interface{}{
		"192.168.0.0/16": "network16",
		"192.168.1.0/24": "network24",
		"192.168.1.1/32": "host32",
	}

	for cidr, value := range routes {
		if err := router.AddRoute(cidr, value); err != nil {
			t.Fatalf("Failed to add route %s: %v", cidr, err)
		}
	}

	// 192.168.1.1 should match the /32 route (most specific)
	result, err := router.GetRoute("192.168.1.1")
	if err != nil {
		t.Fatalf("GetRoute() error = %v", err)
	}
	if result != "host32" {
		t.Errorf("GetRoute() = %v, expected host32 (longest prefix match)", result)
	}

	// 192.168.1.100 should match the /24 route
	result, err = router.GetRoute("192.168.1.100")
	if err != nil {
		t.Fatalf("GetRoute() error = %v", err)
	}
	if result != "network24" {
		t.Errorf("GetRoute() = %v, expected network24", result)
	}

	// 192.168.2.1 should match the /16 route
	result, err = router.GetRoute("192.168.2.1")
	if err != nil {
		t.Fatalf("GetRoute() error = %v", err)
	}
	if result != "network16" {
		t.Errorf("GetRoute() = %v, expected network16", result)
	}
}

func TestSimpleRouter_MultipleRoutesSamePrefix(t *testing.T) {
	router := NewSimpleRouter()

	// Add multiple routes with the same prefix but different values
	// The last one added should overwrite the previous one
	if err := router.AddRoute("192.168.1.0/24", "route1"); err != nil {
		t.Fatalf("Failed to add route: %v", err)
	}

	if err := router.AddRoute("192.168.1.0/24", "route2"); err != nil {
		t.Fatalf("Failed to add route: %v", err)
	}

	result, err := router.GetRoute("192.168.1.1")
	if err != nil {
		t.Fatalf("GetRoute() error = %v", err)
	}
	if result != "route2" {
		t.Errorf("GetRoute() = %v, expected route2 (last added)", result)
	}
}

func TestSimpleRouter_NoRouteFound(t *testing.T) {
	router := NewSimpleRouter()

	// Add a route
	if err := router.AddRoute("192.168.1.0/24", "route1"); err != nil {
		t.Fatalf("Failed to add route: %v", err)
	}

	// Try to get a route that doesn't match
	result, err := router.GetRoute("10.0.0.1")
	if err != nil {
		t.Fatalf("GetRoute() error = %v", err)
	}
	if result != nil {
		t.Errorf("GetRoute() = %v, expected nil (no route found)", result)
	}
}

func TestSimpleRouter_EmptyRouter(t *testing.T) {
	router := NewSimpleRouter()

	result, err := router.GetRoute("192.168.1.1")
	if err != nil {
		t.Fatalf("GetRoute() error = %v", err)
	}
	if result != nil {
		t.Errorf("GetRoute() = %v, expected nil (empty router)", result)
	}
}

func TestSimpleRouter_MixedIPv4IPv6(t *testing.T) {
	router := NewSimpleRouter()

	// Add both IPv4 and IPv6 routes
	if err := router.AddRoute("192.168.1.0/24", "ipv4route"); err != nil {
		t.Fatalf("Failed to add IPv4 route: %v", err)
	}

	if err := router.AddRoute("2001:db8::/32", "ipv6route"); err != nil {
		t.Fatalf("Failed to add IPv6 route: %v", err)
	}

	// Test IPv4 lookup
	result, err := router.GetRoute("192.168.1.1")
	if err != nil {
		t.Fatalf("GetRoute() error = %v", err)
	}
	if result != "ipv4route" {
		t.Errorf("GetRoute() = %v, expected ipv4route", result)
	}

	// Test IPv6 lookup
	result, err = router.GetRoute("2001:db8::1")
	if err != nil {
		t.Fatalf("GetRoute() error = %v", err)
	}
	if result != "ipv6route" {
		t.Errorf("GetRoute() = %v, expected ipv6route", result)
	}

	// IPv4 lookup should not return IPv6 route
	result, err = router.GetRoute("10.0.0.1")
	if err != nil {
		t.Fatalf("GetRoute() error = %v", err)
	}
	if result == "ipv6route" {
		t.Error("IPv4 lookup returned IPv6 route")
	}
}
