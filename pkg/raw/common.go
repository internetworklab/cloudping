package raw

import (
	"log"
	"math"
	"strings"

	"context"

	pkgmyprom "example.com/rbmq-demo/pkg/myprom"
	pkgutils "example.com/rbmq-demo/pkg/utils"
	"github.com/google/gopacket/layers"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

func markAsSentBytes(ctx context.Context, n int) {
	commonLabels := ctx.Value(pkgutils.CtxKeyPromCommonLabels).(prometheus.Labels)
	if commonLabels == nil {
		log.Println("commonLabels is nil, wont record sent bytes a prometheus metrics")
		return
	}

	counterStore := ctx.Value(pkgutils.CtxKeyPrometheusCounterStore).(*pkgmyprom.CounterStore)
	if counterStore == nil {
		log.Println("counterStore is nil, wont record sent bytes as prometheus metrics")
		return
	}
	counterStore.NumBytesSent.With(commonLabels).Add(float64(n))
}

func markAsReceivedBytes(ctx context.Context, n int) {
	commonLabels := ctx.Value(pkgutils.CtxKeyPromCommonLabels).(prometheus.Labels)
	if commonLabels == nil {
		log.Println("commonLabels is nil, wont record received bytes as prometheus metrics")
		return
	}
	counterStore := ctx.Value(pkgutils.CtxKeyPrometheusCounterStore).(*pkgmyprom.CounterStore)
	if counterStore == nil {
		log.Println("counterStore is nil, wont record received bytes as prometheus metrics")
		return
	}
	counterStore.NumBytesReceived.With(commonLabels).Add(float64(n))
}

// ipVersion: 4 or 6
// ipprotoNum: for IPv4, it's the iana ipprotocol number, for IPv6, it's the NextHeader field value
func getMaxPayloadLen(ipVersion int, ipprotoNum int, pmtu *int) int {
	minMTU := pkgutils.GetMinimumMTU()
	if pmtu != nil {
		if *pmtu >= 0 && *pmtu < minMTU {
			minMTU = *pmtu
		}
	}

	switch ipVersion {
	case ipv4.Version:
		switch ipprotoNum {
		case int(layers.IPProtocolICMPv4):
			return int(math.Max(0, float64(minMTU-ipv4.HeaderLen-headerSizeICMP)))
		case int(layers.IPProtocolUDP):
			return int(math.Max(0, float64(minMTU-ipv4.HeaderLen-udpHeaderLen)))
		default:
			log.Printf("unknown ip protocol number: %d", ipprotoNum)
			return 0
		}
	case ipv6.Version:
		switch ipprotoNum {
		case int(layers.IPProtocolICMPv6):
			return int(math.Max(0, float64(minMTU-ipv6.HeaderLen-headerSizeICMP)))
		case int(layers.IPProtocolUDP):
			return int(math.Max(0, float64(minMTU-ipv6.HeaderLen-udpHeaderLen)))
		default:
			log.Printf("unknown ip protocol number: %d", ipprotoNum)
			return 0
		}
	default:
		log.Printf("unknown ip version: %d", ipVersion)
		return 0
	}
}

func isFatalErr(err error) bool {
	errStr := err.Error()
	if strings.Contains(errStr, "message too long") {
		return false
	}

	return true
}
