package utils

import (
	"context"
	"net"
	"time"
)

func NewCustomResolver(resolverAddress *string, resolveTimeout time.Duration) *net.Resolver {
	var resolver *net.Resolver = net.DefaultResolver
	if resolverAddress != nil && *resolverAddress != "" {
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{
					Timeout: resolveTimeout,
				}
				return d.DialContext(ctx, network, *resolverAddress)
			},
		}
	}
	return resolver
}
