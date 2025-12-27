package routing

import (
	"bytes"
	"fmt"
	"net"

	"github.com/google/btree"
)

type Route struct {
	Prefix net.IP
	Value  interface{}
}

func (route *Route) Less(other btree.Item) bool {
	otherRoute, ok := other.(*Route)
	if !ok {
		panic("other is not a Route")
	}
	return bytes.Compare(route.Prefix, otherRoute.Prefix) < 0
}

type RouteGroup struct {
	PrefixLen int
	Routes    *btree.BTree
}

func (routeGroup *RouteGroup) Less(other btree.Item) bool {
	otherRouteGroup, ok := other.(*RouteGroup)
	if !ok {
		panic("other is not a RouteGroup")
	}
	return routeGroup.PrefixLen < otherRouteGroup.PrefixLen
}

func NewRouteGroup() *RouteGroup {
	return &RouteGroup{
		Routes: btree.New(2),
	}
}

type SimpleRouter struct {
	// Routes is basically a collection of RouteGroups
	routes  *btree.BTree
	routes6 *btree.BTree
}

func NewSimpleRouter() *SimpleRouter {
	router := &SimpleRouter{
		routes:  btree.New(2),
		routes6: btree.New(2),
	}
	return router
}

func (router *SimpleRouter) doAddRoute(ipNet *net.IPNet, value interface{}) error {
	ones, _ := ipNet.Mask.Size()
	if !router.routes.Has(&RouteGroup{PrefixLen: ones}) {
		newRouteGroup := NewRouteGroup()
		newRouteGroup.PrefixLen = ones
		router.routes.ReplaceOrInsert(newRouteGroup)
	}

	routeGroup, ok := router.routes.Get(&RouteGroup{PrefixLen: ones}).(*RouteGroup)
	if !ok {
		panic("item is not a RouteGroup")
	}

	routeGroup.Routes.ReplaceOrInsert(&Route{Prefix: ipNet.IP, Value: value})
	return nil
}

func (router *SimpleRouter) doAddRoute6(ipNet *net.IPNet, value interface{}) error {
	ones, _ := ipNet.Mask.Size()
	if !router.routes6.Has(&RouteGroup{PrefixLen: ones}) {
		newRouteGroup := NewRouteGroup()
		newRouteGroup.PrefixLen = ones
		router.routes6.ReplaceOrInsert(newRouteGroup)
	}

	routeGroup, ok := router.routes6.Get(&RouteGroup{PrefixLen: ones}).(*RouteGroup)
	if !ok {
		panic("item is not a RouteGroup")
	}

	routeGroup.Routes.ReplaceOrInsert(&Route{Prefix: ipNet.IP, Value: value})
	return nil
}

func (router *SimpleRouter) AddRoute(cidr string, value interface{}) error {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("failed to parse CIDR: %v", err)
	}

	if ipNet.IP.To4() != nil {
		return router.doAddRoute(ipNet, value)
	} else {
		return router.doAddRoute6(ipNet, value)
	}
}

func (router *SimpleRouter) GetRoute(ip string) (interface{}, error) {
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return nil, fmt.Errorf("invalid ip address: %s", ip)
	}
	result := new(Route)

	var table *btree.BTree
	var maskTotalLen int

	if ipAddr.To4() != nil {
		table = router.routes
		maskTotalLen = 32
	} else {
		table = router.routes6
		maskTotalLen = 128
	}

	table.Descend(func(item btree.Item) bool {
		routeGroup, ok := item.(*RouteGroup)
		if !ok {
			panic("item is not a RouteGroup")
		}
		maskedIp := ipAddr.Mask(net.CIDRMask(routeGroup.PrefixLen, maskTotalLen))
		if route := routeGroup.Routes.Get(&Route{Prefix: maskedIp}); route != nil {
			result.Value = route.(*Route).Value
			return false
		}
		return true
	})

	return result.Value, nil
}
