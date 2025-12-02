package main

import (
	"context"
	"log"
	"net"
	"os"
	"runtime"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

type ICMPTrackerEntry struct {
	SentAt     time.Time
	ReceivedAt []time.Time
}

func (itEnt *ICMPTrackerEntry) HasReceived() bool {
	return len(itEnt.ReceivedAt) > 0
}

func (itEnt *ICMPTrackerEntry) HasDup() bool {
	return len(itEnt.ReceivedAt) > 1
}

func (itEnt *ICMPTrackerEntry) RTT() []time.Duration {
	deltas := make([]time.Duration, 0)
	for _, receivedAt := range itEnt.ReceivedAt {
		deltas = append(deltas, receivedAt.Sub(itEnt.SentAt))
	}
	return deltas
}

type ServiceRequest struct {
	Func   func(ctx context.Context) error
	Result chan error
}

type ICMPTracker struct {
	id          int
	initSeq     int
	latestSeq   int
	store       map[int]*ICMPTrackerEntry
	serviceChan chan chan ServiceRequest
	closeCh     chan interface{}
	nrUnAck     int
	nrMaxCount  *int
}

type ICMPTrackerConfig struct {
	ID         int
	InitialSeq int
	MaxCount   *int
}

func NewICMPTracker(config *ICMPTrackerConfig) *ICMPTracker {
	it := &ICMPTracker{
		id:          config.ID,
		initSeq:     config.InitialSeq,
		latestSeq:   config.InitialSeq,
		nrMaxCount:  config.MaxCount,
		store:       make(map[int]*ICMPTrackerEntry),
		serviceChan: make(chan chan ServiceRequest),
	}
	return it
}

func (it *ICMPTracker) Run(ctx context.Context) {
	it.closeCh = make(chan interface{})
	defer close(it.serviceChan)

	for {

		serviceSubCh := make(chan ServiceRequest)

		select {
		case <-it.closeCh:
			return
		case it.serviceChan <- serviceSubCh:
			serviceReq := <-serviceSubCh
			err := serviceReq.Func(ctx)
			serviceReq.Result <- err
			close(serviceReq.Result)
		}
	}
}

func (it *ICMPTracker) MarkSent(seq int) error {
	requestCh := <-it.serviceChan
	defer close(requestCh)

	fn := func(ctx context.Context) error {

		it.store[seq] = &ICMPTrackerEntry{
			SentAt: time.Now(),
		}
		it.nrUnAck++

		return nil
	}

	resultCh := make(chan error)

	requestCh <- ServiceRequest{
		Func:   fn,
		Result: resultCh,
	}

	return <-resultCh
}

func (it *ICMPTracker) MarkTimeout(seq int) error {
	requestCh := <-it.serviceChan
	defer close(requestCh)

	fn := func(ctx context.Context) error {
		if ent, ok := it.store[seq]; ok {
			if ent.ReceivedAt == nil {
				it.nrUnAck--
			}
			var zeroTime time.Time
			ent.ReceivedAt = append(ent.ReceivedAt, zeroTime)
		}
		return nil
	}

	resultCh := make(chan error)

	requestCh <- ServiceRequest{
		Func:   fn,
		Result: resultCh,
	}

	return <-resultCh
}

func (it *ICMPTracker) MarkReceived(seq int) error {
	requestCh := <-it.serviceChan
	defer close(requestCh)

	fn := func(ctx context.Context) error {
		if ent, ok := it.store[seq]; ok {
			if ent.ReceivedAt == nil {
				it.nrUnAck--
			}
			ent.ReceivedAt = append(ent.ReceivedAt, time.Now())
		}
		return nil
	}

	resultCh := make(chan error)

	requestCh <- ServiceRequest{
		Func:   fn,
		Result: resultCh,
	}

	return <-resultCh
}

func (it *ICMPTracker) GetNrUnAck() int {
	var nrUnAck *int = new(int)
	serviceCh := <-it.serviceChan
	defer close(serviceCh)

	fn := func(ctx context.Context) error {
		*nrUnAck = it.nrUnAck
		return nil
	}
	resultCh := make(chan error)
	serviceCh <- ServiceRequest{
		Func:   fn,
		Result: resultCh,
	}
	return *nrUnAck
}

func (it *ICMPTracker) GetID() int {
	return it.id
}

func (it *ICMPTracker) IterateSeq() int {
	requestCh := <-it.serviceChan
	defer close(requestCh)

	var seqPtr *int = new(int)

	fn := func(ctx context.Context) error {
		seq := it.latestSeq
		it.latestSeq++
		*seqPtr = seq
		return nil
	}

	resultCh := make(chan error)
	requestCh <- ServiceRequest{
		Func:   fn,
		Result: resultCh,
	}

	return *seqPtr
}

func (it *ICMPTracker) IsNotDone() bool {
	requestCh := <-it.serviceChan
	defer close(requestCh)

	var contPtr *bool = new(bool)
	fn := func(ctx context.Context) error {
		var cont bool
		if it.nrMaxCount != nil {
			cont = (it.latestSeq - it.initSeq) < *it.nrMaxCount
		} else {
			cont = true
		}
		*contPtr = cont
		return nil
	}
	resultCh := make(chan error)
	requestCh <- ServiceRequest{
		Func:   fn,
		Result: resultCh,
	}

	return *contPtr
}

func (it *ICMPTracker) Close() {
	if it.closeCh == nil {
		panic("ICMPTracker is not started yet")
	}
	close(it.closeCh)
}

func (it *ICMPTracker) IsRelevant(msg *icmp.Message) (*icmp.Echo, bool) {
	icmpEcho, ok := msg.Body.(*icmp.Echo)
	if ok {
		if icmpEcho.ID == it.id {
			return icmpEcho, true
		}
	}
	return nil, false
}

func main() {
	if runtime.GOOS != "linux" {
		log.Fatal("This program only runs on Linux")
	}

	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		log.Fatalf("failed to listen on packet:icmp: %v", err)
	}

	defer conn.Close()

	icmpID := os.Getpid() & 0xffff
	maxCount := 10
	icmpTrackerConfig := &ICMPTrackerConfig{
		ID:         icmpID,
		InitialSeq: 0,
		MaxCount:   &maxCount,
	}
	tracker := NewICMPTracker(icmpTrackerConfig)

	receiveCh := make(chan interface{})
	go func() {
		defer close(receiveCh)
		stdandardMTU := 1500
		for tracker.IsNotDone() || tracker.GetNrUnAck() > 0 {
			log.Println("Receiving")
			receivBuf := make([]byte, stdandardMTU)
			n, peer, err := conn.ReadFrom(receivBuf)
			if err != nil {
				log.Fatalf("failed to read from connection: %v", err)
			}

			receivMsg, err := icmp.ParseMessage(ipv4.ICMPTypeEchoReply.Protocol(), receivBuf[:n])
			if err != nil {
				log.Fatalf("failed to parse icmp message: %v", err)
			}

			if icmpEcho, ok := tracker.IsRelevant(receivMsg); ok {
				log.Printf("received echo packet, type: %v, id: %v, seq: %v, code: %v, body length: %v, peer: %v", receivMsg.Type, icmpEcho.ID, icmpEcho.Seq, receivMsg.Code, receivMsg.Body.Len(int(receivMsg.Type.Protocol())), peer.String())
			}
		}
	}()

	intv := 500 * time.Millisecond
	for tracker.IsNotDone() {
		writeMsg := icmp.Message{
			Type: ipv4.ICMPTypeEcho,
			Body: &icmp.Echo{
				ID:  icmpID,
				Seq: tracker.IterateSeq(),
			},
		}
		writeBuff, err := writeMsg.Marshal(nil)
		if err != nil {
			log.Fatalf("failed to marshal icmp message: %v", err)
		}

		if _, err := conn.WriteTo(writeBuff, &net.IPAddr{IP: net.ParseIP("8.8.4.4")}); err != nil {
			log.Fatalf("failed to write to connection: %v", err)
		}
		time.Sleep(intv)
	}

	<-receiveCh
}
