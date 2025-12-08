package throttle

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/btree"
)

const defaultTimeSlice time.Duration = 50 * time.Millisecond

type TimeSlicedEVLoopSched struct {
	evCh            chan chan TSSchedEVObject
	config          *TimeSlicedEVLoopSchedConfig
	outC            chan interface{}
	numEventsPassed *int
	nodeQueue       *btree.BTree

	// numbers of nodes added since the very beginning
	// we use this field for generating unique node id
	nodesAdded int

	defaultTimeSlice *time.Duration
}

type TimeSlicedEVLoopSchedConfig struct {
	DefaultTimeSlice *time.Duration
	BTreeOrder       *int
}

func NewTimeSlicedEVLoopSched(config *TimeSlicedEVLoopSchedConfig) (*TimeSlicedEVLoopSched, error) {
	if config == nil {
		return nil, fmt.Errorf("config is nil")
	}

	sched := &TimeSlicedEVLoopSched{
		evCh:            make(chan chan TSSchedEVObject),
		config:          config,
		outC:            make(chan interface{}),
		numEventsPassed: new(int),
		nodesAdded:      0,
	}
	btreeOrder := 2
	if config.BTreeOrder != nil {
		if *config.BTreeOrder < 2 {
			return nil, fmt.Errorf("btree order must be at least 2")
		}
		btreeOrder = *config.BTreeOrder
		sched.nodeQueue = btree.New(btreeOrder)
	}
	*(sched.numEventsPassed) = 0

	tsUsed := defaultTimeSlice
	if config.DefaultTimeSlice != nil {
		if config.DefaultTimeSlice.Milliseconds() < 10 {
			return nil, fmt.Errorf("default time slice must be at least 10 milliseconds")
		}
		tsUsed = *config.DefaultTimeSlice
		sched.defaultTimeSlice = &tsUsed
	}

	return sched, nil
}

func (tsSched *TimeSlicedEVLoopSched) GetOutput() <-chan interface{} {
	return tsSched.outC
}

func (tsSched *TimeSlicedEVLoopSched) Run(ctx context.Context) chan error {
	errorChan := make(chan error)
	outC := tsSched.outC
	evCh := tsSched.evCh
	numEventsPassed := tsSched.numEventsPassed
	nodeQueue := tsSched.nodeQueue

	go func() {
		log.Println("Event loop is started")
		defer log.Println("Event loop is stopped")
		defer close(errorChan)

		for {
			evRequestCh := make(chan TSSchedEVObject)
			select {
			case <-ctx.Done():
				return
			case evCh <- evRequestCh:
				evRequest := <-evRequestCh
				*numEventsPassed = *numEventsPassed + 1

				log.Printf("Event %s, Generation: %d", evRequest.Type, *numEventsPassed)

				switch evRequest.Type {
				case EVNewSource:
					inputChan, ok := evRequest.Payload.(<-chan interface{})
					if !ok {
						panic("unexpected ev payload, it's not of a type of <-chan interface{}")
					}

					newNodeId := tsSched.nodesAdded
					tsSched.nodesAdded++
					newNodeObject := &TSSchedSourceNode{
						Id:              newNodeId,
						InC:             inputChan,
						ScheduledTime:   0.0,
						queuePending:    make(chan interface{}, 1),
						NumItemsCopied:  make([]int, 3),
						CurrentDataBlob: nil,
					}
					log.Printf("Added node %d to evCenter", newNodeId)
					newNodeObject.RegisterDataEvent(evCh, nodeQueue)
					evRequest.Result <- nil
				case EVNewDataTask:

					evPayloadNode, ok := evRequest.Payload.(*TSSchedSourceNode)
					if !ok {
						panic("unexpected ev payload, it's not of a type of struct *Node")
					}

					if nodeQueue.ReplaceOrInsert(evPayloadNode) != nil {
						panic("the node is already in the queue, which is unexpected")
					}

					headItem := nodeQueue.DeleteMin()
					if headItem == nil {
						panic("head item in the queue shouldn't be nil")
					}

					nodeObject, ok := headItem.(*TSSchedSourceNode)
					if !ok {
						panic("head item in the queue is not a of type struct Node")
					}

					itemsCopied := <-nodeObject.Run(outC, nodeObject, *tsSched.defaultTimeSlice)
					nodeObject.ScheduledTime += 1.0
					nodeObject.NumItemsCopied[0] = nodeObject.NumItemsCopied[1]
					nodeObject.NumItemsCopied[1] = nodeObject.NumItemsCopied[2]
					nodeObject.NumItemsCopied[2] = itemsCopied
					evRequest.Result <- nil

				default:
					panic(fmt.Sprintf("unknown event type: %s", evRequest.Type))
				}

			}

		}
	}()
	return errorChan
}

func (tsSched *TimeSlicedEVLoopSched) AddInput(inputChan <-chan interface{}) error {
	evSubCh, ok := <-tsSched.evCh
	if !ok {
		return fmt.Errorf("ev channel is already closed")
	}
	defer close(evSubCh)

	evObj := TSSchedEVObject{
		Type:    EVNewSource,
		Payload: inputChan,
		Result:  make(chan error),
	}
	evSubCh <- evObj
	return <-evObj.Result
}

type TSSchedSourceNode struct {
	Id              int
	InC             <-chan interface{}
	ScheduledTime   float64
	queuePending    chan interface{}
	NumItemsCopied  []int
	CurrentDataBlob *TSSchedDataBlob
}

type TSSchedDataBlob struct {
	BufferedChan chan interface{}
	Size         int
	Remaining    int
}

const defaultChannelBufferSize = 1024

func (nd *TSSchedSourceNode) RegisterDataEvent(evCh <-chan chan TSSchedEVObject, nodeQueue *btree.BTree) {
	toBeSendFragments := make(chan *TSSchedDataBlob)

	go func() {
		for dataBlob := range toBeSendFragments {
			for dataBlob.Remaining > 0 {
				evSubCh := <-evCh
				nd.queuePending <- struct{}{}
				nd.CurrentDataBlob = dataBlob
				evObj := TSSchedEVObject{
					Type:    EVNewDataTask,
					Payload: nd,
					Result:  make(chan error),
				}

				evSubCh <- evObj
				<-evObj.Result

				<-nd.queuePending
			}
		}
	}()

	go func() {
		itemsLoaded := 0
		defer log.Printf("node %d is drained", nd.Id)

		totalItemsProcessed := 0

		staging := make(chan interface{}, defaultChannelBufferSize)
		temporaryBuf := make([]interface{}, 0)
		for item := range nd.InC {
			totalItemsProcessed++

			if len(temporaryBuf) > 0 {
				fmt.Println("flushing temporaryBuf: ", len(temporaryBuf))
				for _, item := range temporaryBuf {
					staging <- item
					itemsLoaded++
				}
				temporaryBuf = make([]interface{}, 0)
			}
			select {
			case staging <- item:
				itemsLoaded++
			default:
				temporaryBuf = append(temporaryBuf, item)
				toBeSendFragments <- &TSSchedDataBlob{
					BufferedChan: staging,
					Size:         itemsLoaded,
					Remaining:    itemsLoaded,
				}
				itemsLoaded = 0
				staging = make(chan interface{}, defaultChannelBufferSize)
			}
		}

		if itemsLoaded > 0 {
			// flush the remaining items in buffer
			toBeSendFragments <- &TSSchedDataBlob{
				BufferedChan: staging,
				Size:         itemsLoaded,
				Remaining:    itemsLoaded,
			}
			itemsLoaded = 0
		}

		fmt.Println("total items processed: ", totalItemsProcessed)
	}()
}

func (nd *TSSchedSourceNode) TotalCopied() int {
	sum := 0
	for _, v := range nd.NumItemsCopied {
		sum += v
	}
	return sum
}

func (n *TSSchedSourceNode) Less(item btree.Item) bool {
	if n == nil || item == nil {
		panic("comparing node against nil")
	}

	if nodeItem, ok := item.(*TSSchedSourceNode); ok {
		if n.TotalCopied() == nodeItem.TotalCopied() {
			return !(n.Id < nodeItem.Id)
		}
		return n.TotalCopied() < nodeItem.TotalCopied()
	}

	panic(fmt.Sprintf("comparing node with unexpected item type: %T", item))
}

type TSSchedEVType string

const (
	EVNewSource   TSSchedEVType = "new_source"
	EVNewDataTask TSSchedEVType = "new_data_task"
)

type TSSchedEVObject struct {
	Type    TSSchedEVType
	Payload interface{}
	Result  chan error
}

// the returning channel doesn't emit anything meaningful, it's simply for synchronization
func (nd *TSSchedSourceNode) Run(outC chan<- interface{}, nodeObject *TSSchedSourceNode, duration time.Duration) <-chan int {
	runCh := make(chan int)

	var itemsCopied *int = new(int)
	*itemsCopied = 0

	go func() {
		defer func() {
			n := *itemsCopied
			runCh <- n
		}()

		timeout := time.After(duration)

		for {
			select {
			case <-timeout:
				// todo: Some data may still in the buffer when timeout.
				return
			case item, ok := <-nodeObject.CurrentDataBlob.BufferedChan:
				if !ok {
					return
				}
				outC <- item
				*itemsCopied = *itemsCopied + 1
				nodeObject.CurrentDataBlob.Remaining--
			default:
				return
			}
		}
	}()
	return runCh
}
