package throttle

import (
	"context"
)

// A generic MISO scheduler doesn't care about the label at all,
// it purely focus on scheduling packets from multiple inputs to a single output.
// However, since the type for packets we used here is interface{},
// the user is free to design its own struct for wrapping packets to make them tagged.
// For example, a packet of interface{} could be a MySimpleWrappedPacket{OriginPacket: xxx, Label: yyy}
// As for the type of input and ouput items of the channels, they are the same, i.e. the type of output items
// is exactly the same as the type of the input items.
type GenericMISOScheduler interface {
	AddInput(ctx context.Context, inputChan <-chan interface{}) (opaqueSourceId interface{}, err error)

	// Simply rely on source channel close to remove the input is un-reliable.
	RemoveInput(ctx context.Context, opaqueSourceId interface{}) error
	GetOutput() <-chan interface{}
}

type SISOPipe interface {
	GetInput() chan<- interface{}
	GetOutput() <-chan interface{}
}

type EventResult struct {
	Error error
}
