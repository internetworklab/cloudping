package raw

import (
	"log"
	"net"

	"context"

	pkgmyprom "example.com/rbmq-demo/pkg/myprom"
	pkgutils "example.com/rbmq-demo/pkg/utils"
	"github.com/prometheus/client_golang/prometheus"
)

func getMaximumMTU() int {
	ifaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}
	maximumMTU := -1
	for _, iface := range ifaces {
		if iface.MTU > maximumMTU {
			maximumMTU = iface.MTU
		}
	}
	if maximumMTU == -1 {
		panic("can't determine maximum MTU")
	}
	return maximumMTU
}

func setDFBit(conn net.PacketConn) error {
	// deprecated, we now compose the entire ip packet, including ip header,
	// so, we no longer need this.
	return nil
}

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
