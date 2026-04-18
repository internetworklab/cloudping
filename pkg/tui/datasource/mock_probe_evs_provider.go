package datasource

import (
	"context"
	"time"

	pkgtui "github.com/internetworklab/cloudping/pkg/tui"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

type MockedProbeEventsProvider struct {
	SampleIntv time.Duration
}

func (provider *MockedProbeEventsProvider) getSampleIntv() time.Duration {
	if provider.SampleIntv > 0 {
		return provider.SampleIntv
	}
	return time.Second
}

func (provider *MockedProbeEventsProvider) GetProbeEvents(ctx context.Context, request pkgtui.ProbeRequestDescriptor) <-chan pkgtui.ProbeEvent {
	ch := make(chan pkgtui.ProbeEvent)

	go func() {
		defer close(ch)

		idx := 0
		first := true

		for ip := range pkgutils.GetMemberAddresses32(ctx, request.TargetCIDR) {
			// Wait 1 second between samples, but not before the first one
			if !first {
				select {
				case <-time.After(provider.getSampleIntv()):
				case <-ctx.Done():
					return
				}
			}
			first = false

			var rttMs int
			switch idx % 3 {
			case 0:
				rttMs = -1
			case 1, 2:
				rttMs = 1
			}

			select {
			case ch <- pkgtui.ProbeEvent{
				IP:    ip,
				RTTMs: rttMs,
			}:
			case <-ctx.Done():
				return
			}

			idx++
		}
	}()

	return ch
}
