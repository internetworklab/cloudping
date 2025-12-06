package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/btree"
)

type DataBlob struct {
	BufferedChan chan interface{}
	Size         int
}

type Node struct {
	Id            int
	Name          string
	InC           chan interface{}
	ScheduledTime float64
	Mux           sync.Mutex
	queuePending  chan interface{}
}

const defaultChannelBufferSize = 1024

func (nd *Node) RegisterDataEvent(evCh <-chan chan EVObject, nodeQueue *btree.BTree) {
	toBeSendFragments := make(chan DataBlob)
	taskSeq := 0
	go func() {
		defer close(toBeSendFragments)
		for dataBlob := range toBeSendFragments {

			evSubCh := <-evCh
			nd.queuePending <- struct{}{}
			nodeQueue.ReplaceOrInsert(nd)
			evObj := EVObject{
				Type:    EVNewDataTask,
				Payload: dataBlob,
				Result:  make(chan error),
			}

			evSubCh <- evObj
			<-evObj.Result
			taskSeq++
		}
	}()

	go func() {
		defer log.Printf("node %s is drained", nd.Name)

		staging := make(chan interface{}, defaultChannelBufferSize)
		itemsLoaded := 0
		for item := range nd.InC {
			select {
			case staging <- item:
				itemsLoaded++
			default:
				toBeSendFragments <- DataBlob{
					BufferedChan: staging,
					Size:         itemsLoaded,
				}
				itemsLoaded = 0
				staging = make(chan interface{}, defaultChannelBufferSize)
			}
		}

		if itemsLoaded > 0 {
			// flush the remaining items in buffer
			itemsLoaded = 0
			toBeSendFragments <- DataBlob{
				BufferedChan: staging,
				Size:         itemsLoaded,
			}
		}
	}()
}

func (n *Node) Less(item btree.Item) bool {
	if n == nil || item == nil {
		return true
	}

	if nodeItem, ok := item.(*Node); ok {
		if math.Abs(n.ScheduledTime-nodeItem.ScheduledTime) < 0.01 {
			return !(n.Id < nodeItem.Id)
		}
		return n.ScheduledTime < nodeItem.ScheduledTime
	}

	panic(fmt.Sprintf("comparing node with unexpected item type: %T", item))
}

type EVType string

const (
	EVNodeAdded   EVType = "node_added"
	EVNewDataTask EVType = "new_data_task"
)

type EVObject struct {
	Type    EVType
	Payload interface{}
	Result  chan error
}

// the returning channel doesn't emit anything meaningful, it's simply for synchronization
func (nd *Node) Run(outC chan<- interface{}, nodeObject *Node, dataBlob DataBlob) <-chan interface{} {
	runCh := make(chan interface{})

	go func() {
		defer close(runCh)

		timeout := time.After(defaultTimeSlice)

		for {
			select {
			case <-timeout:
				return
			case item, ok := <-dataBlob.BufferedChan:
				if !ok {
					return
				}
				outC <- item
			default:
				return
			}
		}
	}()
	return runCh
}

func anonymousSource(ctx context.Context, content string) chan interface{} {
	outC := make(chan interface{})
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				outC <- content
			}
		}
	}()
	return outC
}

const defaultTimeSlice time.Duration = 50 * time.Millisecond

func main() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nodeQueue := btree.New(2)

	outC := make(chan interface{})
	evCh := make(chan chan EVObject)

	allNodes := make(map[int]*Node)

	numEventsPassed := 0

	go func() {
		log.Println("evCenter started")

		defer close(evCh)

		for {
			evRequestCh := make(chan EVObject)
			select {
			case <-ctx.Done():
				return
			case evCh <- evRequestCh:
				evRequest := <-evRequestCh
				numEventsPassed++

				log.Printf("Event %s, Generation: %d", evRequest.Type, numEventsPassed)

				switch evRequest.Type {
				case EVNodeAdded:
					newNode, ok := evRequest.Payload.(*Node)
					if !ok {
						panic("unexpected node type")
					}
					log.Printf("Added node %s to evCenter", newNode.Name)
					newNode.RegisterDataEvent(evCh, nodeQueue)
					evRequest.Result <- nil
				case EVNewDataTask:
					headItem := nodeQueue.DeleteMin()
					if headItem == nil {
						panic("head item in the queue shouldn't be nil")
					}

					nodeObject, ok := headItem.(*Node)
					if !ok {
						panic("head item in the queue is not a of type struct Node")
					}

					dataBlob, ok := evRequest.Payload.(DataBlob)
					if !ok {
						panic("payload of event is not a of type struct DataBlob")
					}

					<-nodeObject.Run(outC, nodeObject, dataBlob)
					nodeObject.ScheduledTime += 1.0
					evRequest.Result <- nil

					// release the queue lock, to enable the node be re-queued again
					<-nodeObject.queuePending

				default:
					panic(fmt.Sprintf("unknown event type: %s", evRequest.Type))
				}

			}

		}
	}()

	// consumer goroutine
	go func() {
		stat := make(map[string]int)
		total := 0
		for muxedItem := range outC {
			stat[muxedItem.(string)]++
			total++
			if total%1000 == 0 {
				for k, v := range stat {
					fmt.Printf("%s: %d, %.2f%%\n", k, v, 100*float64(v)/float64(total))
				}
			}
		}
	}()

	add := func(name string) *Node {
		newNodeId := len(allNodes)
		node := &Node{
			Id:   newNodeId,
			Name: name,
			InC:  anonymousSource(ctx, name),
			Mux:  sync.Mutex{},

			// the buffer size of this channel `queuePending` must be 1,
			// to ensure that, for every moment, at most 1 node instance is in-queue
			queuePending: make(chan interface{}, 1),
		}
		allNodes[newNodeId] = node
		return node
	}

	addToEvCenter := func(node *Node) {
		evSubCh, ok := <-evCh
		if !ok {
			panic("evCh is closed")
		}
		evObj := EVObject{
			Type:    EVNodeAdded,
			Payload: node,
			Result:  make(chan error),
		}
		evSubCh <- evObj
		<-evObj.Result
	}

	nodeA := add("A")
	nodeB := add("B")
	nodeC := add("C")

	addToEvCenter(nodeA)
	addToEvCenter(nodeB)
	addToEvCenter(nodeC)

	sig := <-sigs
	fmt.Println("signal received: ", sig, " exitting...")
}
