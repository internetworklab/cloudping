package throttle

// It's a hub, shared by multiple users, each user access the hub through a proxy,
// the hub throttles the traffic of user's write and read, and * all users are subjected
// to the same quota zone * .
// that is to say: max_speed( all users ) <= defined_shared_quota

import (
	"context"
	"fmt"
	"log"
)

type ServiceRequest struct {
	Func   func(ctx context.Context) error
	Result chan error
}

type SharedThrottleHub struct {
	serviceChan       chan chan ServiceRequest
	numProxiesCreated int
	tsSched           *TimeSlicedEVLoopSched
	mimoSched         *MIMOScheduler
}

type SharedThrottleHubConfig struct {
	TSSched  *TimeSlicedEVLoopSched
	Throttle *TokenBasedThrottle
	Smoother *BurstSmoother
}

func NewICMPTransceiveHub(config *SharedThrottleHubConfig) *SharedThrottleHub {
	hub := &SharedThrottleHub{
		serviceChan:       make(chan chan ServiceRequest),
		numProxiesCreated: 0,
		tsSched:           config.TSSched,
	}
	mimoSched := NewMIMOScheduler(&MIMOSchedConfig{
		Muxer:       config.TSSched,
		Middlewares: []SISOPipe{config.Throttle, config.Smoother},
	})
	hub.mimoSched = mimoSched

	return hub
}

func (hub *SharedThrottleHub) Run(ctx context.Context) {
	hub.mimoSched.Run(ctx)

	go func() {
		defer close(hub.serviceChan)

		for {
			requestCh := make(chan ServiceRequest)
			select {
			case <-ctx.Done():
				return
			case hub.serviceChan <- requestCh:
				request := <-requestCh
				err := request.Func(ctx)
				request.Result <- err
				close(request.Result)
			}
		}
	}()
}

// Note:
// inChan: user sends, we read
// outChan: we send, user reads
// ctx: serve as a control handle so that user can stop the goroutines and reclaim the resources
// (ctx is better to be cancallable)
func (hub *SharedThrottleHub) CreateProxy(ctx context.Context, inChan <-chan interface{}) (outChan chan interface{}, err error) {
	requestCh, ok := <-hub.serviceChan
	if !ok {
		// the hub is already shutdown
		return nil, fmt.Errorf("the hub is already shutdown")
	}

	defer close(requestCh)

	var handlerId *int = new(int)

	fn := func(ctx context.Context) error {
		newId := hub.numProxiesCreated
		*handlerId = newId
		hub.numProxiesCreated++

		return nil
	}
	request := ServiceRequest{
		Func:   fn,
		Result: make(chan error),
	}
	requestCh <- request
	err = <-request.Result
	if err != nil {
		return nil, fmt.Errorf("failed to create proxy to the hub: %v", err)
	}

	labelKey := fmt.Sprintf("%d", *handlerId)

	wrappedInChan := make(chan interface{})

	smoothedInChan := make(chan interface{})
	err = hub.mimoSched.AddOutput(smoothedInChan, labelKey)
	if err != nil {
		return nil, fmt.Errorf("failed to add output to mimo scheduler: %v", err)
	}

	outChan = make(chan interface{})
	go func() {
		defer close(outChan)
		defer func() {
			hub.mimoSched.RemoveOutput(labelKey)
			close(smoothedInChan)
			log.Printf("[DBG] closed smoothedInChan for label %s", labelKey)
		}()

		for {
			select {
			case <-ctx.Done():
				// the smoothedInChan is never gonna be closed it self
				// because it is the output of a long-running muxer
				//
				// usually, it is expected that the user cancels the context only when everything is settled,
				// for example, when the last packet of a sequence is comfirmed to be received.
				return
			case pktIn, ok := <-smoothedInChan:
				if !ok {
					// but still consider such case might happen
					return
				}
				outChan <- pktIn
			}
		}
	}()

	go func() {
		// this goroutine would be automatically closed once the inChan is depleted
		defer log.Printf("[DBG] closed wrappedInChan for label %s", labelKey)
		defer close(wrappedInChan)

		opaqueSourceId, err := hub.mimoSched.AddInput(ctx, wrappedInChan, labelKey)
		if err != nil {
			panic(fmt.Errorf("failed to add input to the mimo scheduler: %v", err))
		}

		log.Printf("[DBG] wrappedInChan for label %s is running", labelKey)

		defer func() {
			log.Printf("[DBG] removing input from the mimo scheduler for label %s", labelKey)
			err := hub.mimoSched.RemoveInput(context.Background(), opaqueSourceId)
			if err != nil {
				panic(fmt.Errorf("failed to remove input from the mimo scheduler: %v", err))
			}
			log.Printf("[DBG] input from the mimo scheduler for label %s has been removed", labelKey)
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case pktIn, ok := <-inChan:
				if !ok {
					return
				}
				wrappedInChan <- pktIn
			}
		}

	}()

	return outChan, nil
}
