package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/btree"
)

type Node struct {
	Id                int
	Name              string
	InC               chan interface{}
	Dead              bool
	ItemsCopied       int
	ScheduledTime     float64
	CurrentGeneration *int
	BirthGeneration   int
	staging           chan interface{}
}

func (nd *Node) GetAge() float64 {
	return float64(*nd.CurrentGeneration - nd.BirthGeneration)
}

type DataTask struct {
	Id      int
	NodeRef *Node
	Segment chan interface{}
}

func (dt *DataTask) Less(item btree.Item) bool {
	if dataTaskItem, ok := item.(*DataTask); ok {
		if dt.NodeRef.Id == dataTaskItem.NodeRef.Id {
			return dt.Id < dataTaskItem.Id
		}
		return dt.NodeRef.Less(dataTaskItem.NodeRef)
	}
	panic("comparing data task with unexpected item type")
}

func (node *Node) PrintWithIdx(idx int) {
	fmt.Printf("[%d] ID=%d, Name=%s, ScheduledTime=%f, Age=%f\n", idx, node.Id, node.Name, node.ScheduledTime, node.GetAge())
}

const defaultChannelBufferSize = 1024

func (nd *Node) RegisterDataEvent(evCh <-chan chan EVObject, taskQueue *btree.BTree) {
	go func() {

		defer close(nd.staging)
		defer log.Printf("node %s is drained", nd.Name)

		toBeSendFragments := make(chan chan interface{})
		taskSeq := 0
		go func() {
			defer close(toBeSendFragments)
			for fragment := range toBeSendFragments {

				// whenever there is a segment to be sent,
				// create a data task and notify the event loop center
				dataTask := &DataTask{
					Id:      nd.Id*65536 + taskSeq,
					NodeRef: nd,
					Segment: fragment,
				}
				taskQueue.ReplaceOrInsert(dataTask)
				evObj := EVObject{
					Type:    EVNewDataTask,
					Payload: nil,
					Result:  make(chan error),
				}
				evSubCh := <-evCh
				evSubCh <- evObj
				<-evObj.Result
				taskSeq++
			}
		}()

		staging := make(chan interface{}, defaultChannelBufferSize)
		itemsLoaded := 0
		for item := range nd.InC {
			select {
			case staging <- item:
				itemsLoaded++
			default:
				toBeSendFragments <- staging
				itemsLoaded = 0
				staging = make(chan interface{}, defaultChannelBufferSize)
			}
		}

		if itemsLoaded > 0 {
			// flush the remaining items in buffer
			itemsLoaded = 0
			toBeSendFragments <- staging
		}
	}()
}

func (nd *Node) schedDensity() float64 {
	return float64(nd.ScheduledTime) / math.Max(1.0, nd.GetAge())
}

func (n *Node) Less(item btree.Item) bool {
	if nodeItem, ok := item.(*Node); ok {

		delta := n.schedDensity() - nodeItem.schedDensity()
		if math.Abs(delta) < 0.01 {
			return !(n.Id < nodeItem.Id)
		}
		return delta < 0

	}
	panic("comparing node with unexpected item type")
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
func (nd *Node) Run(outC chan<- interface{}) <-chan interface{} {
	runCh := make(chan interface{})
	headNode := nd
	go func() {
		defer close(runCh)
		defer log.Println("runCh closed", "node", headNode.Name)

		timeout := time.After(defaultTimeSlice)

		for {
			select {
			case <-timeout:
				log.Println("timeout", "node", headNode.Name)
				return
			case item, ok := <-headNode.staging:
				if !ok {
					headNode.Dead = true
					return
				}
				outC <- item
				headNode.ItemsCopied = headNode.ItemsCopied + 1
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
	taskQueue := btree.New(2)

	outC := make(chan interface{})
	evCh := make(chan chan EVObject)

	allNodes := make(map[int]*Node)

	var numEventsPassed *int = new(int)
	*numEventsPassed = 0

	go func() {
		log.Println("evCenter started")

		defer close(evCh)

		for {
			evRequestCh := make(chan EVObject)
			select {
			case <-ctx.Done():
				return
			case evCh <- evRequestCh:
				*numEventsPassed++

				evRequest := <-evRequestCh
				switch evRequest.Type {
				case EVNodeAdded:
					newNode, ok := evRequest.Payload.(*Node)
					if !ok {
						panic("unexpected node type")
					}
					log.Printf("Added node %s to evCenter", newNode.Name)
					newNode.RegisterDataEvent(evCh, taskQueue)
				case EVNewDataTask:
					headTaskItem := taskQueue.DeleteMin()
					if headTaskItem == nil {
						panic("head task item shouldn't be nil")
					}

					taskObject, ok := headTaskItem.(*DataTask)
					if !ok {
						panic("unexpected task item type")
					}

					// badness for running NodeRef.Run() in synchronous way:
					// if the outC is slow, they would slow down the event loop as well.
					nrCopiedPreRun := taskObject.NodeRef.ItemsCopied
					<-taskObject.NodeRef.Run(outC)
					nrCopiedPostRun := taskObject.NodeRef.ItemsCopied
					if nrCopiedPostRun == nrCopiedPreRun {
						// extra penalty for idling
						taskObject.NodeRef.ScheduledTime += 1.0
					}
					taskObject.NodeRef.ScheduledTime += 1.0

					if taskObject.NodeRef.Dead {
						delete(allNodes, taskObject.NodeRef.Id)
					}
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
			fmt.Println("muxedItem: ", muxedItem)
			stat[muxedItem.(string)]++
			total++
			if total%1000 == 0 {
				for k, v := range stat {
					fmt.Printf("%s: %d, %.2f%%\n", k, v, 100*float64(v)/float64(total))
				}
				stat = make(map[string]int)
			}
		}
	}()

	add := func(name string) *Node {
		newNodeId := len(allNodes)
		node := &Node{
			Id:                newNodeId,
			Name:              name,
			ScheduledTime:     0.0,
			CurrentGeneration: numEventsPassed,
			BirthGeneration:   *numEventsPassed,
			InC:               anonymousSource(ctx, name),
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
