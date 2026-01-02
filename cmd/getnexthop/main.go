package main

import (
	"log"
	"net"

	"github.com/vishvananda/netlink"
)

func main() {
	destinations := []string{
		"1.1.1.1",
		"192.168.7.2",
		"192.168.4.2",
	}
	handle, err := netlink.NewHandle()
	if err != nil {
		log.Fatalf("failed to create netlink handle: %v", err)
	}

	for _, destination := range destinations {
		routes, err := handle.RouteGet(net.ParseIP(destination))
		if err != nil {
			log.Printf("failed to get route for %s: %v", destination, err)
			continue
		}
		for _, route := range routes {
			link, err := netlink.LinkByIndex(route.LinkIndex)
			if err != nil {
				log.Printf("failed to get link by index %d: %v", route.LinkIndex, err)
				continue
			}
			log.Printf("Destination: %s, Gateway: %s, Interface: %s, Interface MTU: %d", destination, route.Gw.String(), link.Attrs().Name, link.Attrs().MTU)
		}
	}

}
