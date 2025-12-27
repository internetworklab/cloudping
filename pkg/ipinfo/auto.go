package ipinfo

import (
	"context"
	"fmt"
	"net"

	pkgrouting "example.com/rbmq-demo/pkg/routing"
	"github.com/google/btree"
)

type AutoIPInfoDispatcher struct {
	router *pkgrouting.SimpleRouter
}

func NewAutoIPInfoDispatcher() *AutoIPInfoDispatcher {
	return &AutoIPInfoDispatcher{
		router: pkgrouting.NewSimpleRouter(),
	}
}

type AutoIPInfoRoute struct {
	IPNet          net.IPNet
	IPInfoProvider GeneralIPInfoAdapter
}

func (route *AutoIPInfoRoute) Less(other btree.Item) bool {
	otherRoute, ok := other.(*AutoIPInfoRoute)
	if !ok {
		panic("other is not an AutoIPInfoRoute")
	}
	ones1, _ := route.IPNet.Mask.Size()
	ones2, _ := otherRoute.IPNet.Mask.Size()

	return ones1 < ones2
}

func (autoProvider *AutoIPInfoDispatcher) SetUpDefaultRoutes(
	dn42Provider GeneralIPInfoAdapter,
	internetIPInfoProvider GeneralIPInfoAdapter,
) {
	dn42Net := "172.20.0.0/14"
	dn42Net6 := "fd00::/8"
	neoNet := "10.127.0.0/16"
	ianaNet := "0.0.0.0/0"
	ianaNet6 := "::/0"

	autoProvider.AddRoute(dn42Net, dn42Provider)
	autoProvider.AddRoute(dn42Net6, dn42Provider)
	autoProvider.AddRoute(neoNet, dn42Provider)
	autoProvider.AddRoute(ianaNet, internetIPInfoProvider)
	autoProvider.AddRoute(ianaNet6, internetIPInfoProvider)
}

func (autoProvider *AutoIPInfoDispatcher) AddRoute(prefix string, provider GeneralIPInfoAdapter) {
	autoProvider.router.AddRoute(prefix, provider)
}

func (autoProvider *AutoIPInfoDispatcher) GetIPInfo(ctx context.Context, ipAddr string) (*BasicIPInfo, error) {
	ipinfoProviderRaw, err := autoProvider.router.GetRoute(ipAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get ipinfo for %s: %v", ipAddr, err)
	}

	ipinfoProvider, ok := ipinfoProviderRaw.(GeneralIPInfoAdapter)
	if !ok {
		panic(fmt.Sprintf("ipinfo provider for %s is not a GeneralIPInfoAdapter", ipAddr))
	}

	return ipinfoProvider.GetIPInfo(ctx, ipAddr)
}

func (autoProvider *AutoIPInfoDispatcher) GetName() string {
	return "auto"
}
