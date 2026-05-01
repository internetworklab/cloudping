package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
)

// The intended purpose of this design is to slow down the visitor's registration.
type TicketGenerator interface {
	// block until a ticket is generated or context be cancelled
	// hint: you can pass a timeout context to it.
	GetTicket(ctx context.Context) (string, error)
}

type SharedTickingTicketGenerator struct {
	TickInterval  time.Duration
	ticksProducer chan string
}

func NewSharedTickingTicketGenerator(tickIntv time.Duration) *SharedTickingTicketGenerator {
	if tickIntv == 0 {
		log.Panicf("invalid tick duration: %v", tickIntv)
		return nil
	}

	return &SharedTickingTicketGenerator{TickInterval: tickIntv, ticksProducer: make(chan string, 1)}
}

func (gen *SharedTickingTicketGenerator) Run(ctx context.Context) {
	go func() {
		defer close(gen.ticksProducer)

		ticker := time.NewTicker(gen.TickInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				select {
				case gen.ticksProducer <- t.Format(time.RFC3339Nano):
				default:
				}
			}
		}
	}()
}

func (gen *SharedTickingTicketGenerator) GetTicket(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("tick generating cancelled: %w", ctx.Err())
	case tick, ok := <-gen.ticksProducer:
		if !ok {
			return "", errors.New("tick generator closed")
		}
		return tick, nil
	}
}
