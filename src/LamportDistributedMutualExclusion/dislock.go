package LamportDistributedMutualExclusion

import (
	msgp3 "LamportDistributedMutualExclusion/msg"
	netq2 "RicartAgrawala/netq"
	"container/list"
	"encoding/json"
	// "errors"
	// "fmt"
	"container/heap"
	"log"
	"strconv"
	// "os"
	"sync"
)

const (
	MSG_BUFFERED_SIZE = 100
	CHAN_SIZE         = 1
)

type dislock struct {
	// net
	tp       netq2.TcpPeer
	nodeID   int         // regard it as process/node/nodeID id.
	nodeConn map[int]int // not include itself
	nodePort map[int]int
	peerCnt  int

	// algorithim
	waiting         bool
	requestTS       msgp3.TimeStamp
	replyProSet     *list.List   // then Message.Sender is the key.
	deferProSet     *list.List   // then Message.Sender is the key.
	msgHeap         *MessageHeap // then Message.Sender and Message.TS are the keys.
	mu              sync.Mutex
	chanRcvMsg      chan msgp3.Message
	chanSendMsg     chan *msgp3.Message
	chanAcquireLock chan bool

	// process handler
	p *process

	// sata info
	readCnt  int
	writeCnt int

	// log
	logger *log.Logger
}

func NewDislock(peerCnt, nodeID int, nodePort map[int]int, p *process) (*dislock, error) {
	dl := &dislock{
		nodeID:          nodeID,
		nodePort:        nodePort,
		peerCnt:         peerCnt,
		chanRcvMsg:      make(chan msgp3.Message, MSG_BUFFERED_SIZE),
		chanSendMsg:     make(chan *msgp3.Message, MSG_BUFFERED_SIZE),
		chanAcquireLock: make(chan bool, CHAN_SIZE),
		replyProSet:     list.New(),
		deferProSet:     list.New(),
		p:               p,
		msgHeap:         &MessageHeap{}, // request lock message priority queue.
	}
	heap.Init(dl.msgHeap) // init the queue.
	dl.logger = CreateLog("log/dislock_"+strconv.Itoa(nodeID)+".log", "[dislock] ")
	tp, err := netq2.NewTcpPeer(dl.peerCnt, dl.nodeID, dl.nodePort)
	if err != nil {
		dl.logger.Printf("dislock(%v) create error: %v.\n", dl.nodeID, err.Error())
		return nil, err
	}
	dl.tp = tp
	dl.logger.Printf("dislock(%v) created.\n", dl.nodeID)
	go dl.receiveMessage()
	go dl.lamportDistributedMutualExclusion()
	return dl, nil
}

func (dl *dislock) BuildAllConn() error {
	for i := 0; i < dl.peerCnt; i++ {
		if nodeConnMap, err := dl.tp.BuildAllConn(); err != nil {
			nodeID := i // + 1000
			dl.logger.Printf("dislock(%v) build rpc connection error at peer(%v): %v.\n", dl.nodeID, nodeID, err.Error())
			return err
		} else {
			dl.logger.Printf("dislock(%v) build rpc connection successfully.\n", dl.nodeID)
			dl.nodeConn = nodeConnMap
			return nil
		}
	}
	return nil
}

func (dl *dislock) receiveMessage() {
	for {
		_, lgBytes, err := dl.tp.ReadData()
		if err != nil {
			dl.logger.Printf("lock(%v) receive message error: %v.\n", dl.nodeID, err.Error())
			// return err // leave the error for the sender to handle.
			continue // TODO: any better solution?
		}
		dl.mu.Lock()
		dl.readCnt++
		dl.mu.Unlock()
		var lg msgp3.Message
		json.Unmarshal(lgBytes, &lg)
		// dl.logger.Printf("lock(%v) receive message(%v) from connection(%v).\n", dl.nodeID, lg.String(), rcvConnID)
		dl.chanRcvMsg <- lg
	}
}

func (dl *dislock) sendMessage(msg *msgp3.Message) {
	connID, _ := dl.nodeConn[msg.Receiver]
	lrlBytes, _ := json.Marshal(msg)
	if err := dl.tp.WriteData(connID, lrlBytes); err != nil {
		dl.logger.Printf("lock(%v) send message(%v) to node(%v) by connection(%v) error: %v.\n", dl.nodeID, msg.String(), msg.Receiver, connID, err.Error())
		dl.waiting = !dl.waiting
		// return err // TODO: handle write failure.
	}
	dl.writeCnt++
	dl.logger.Printf("lock(%v) send message(%v) to node(%v) by connection(%v) successfully.\n", dl.nodeID, msg.String(), msg.Receiver, connID)
}

func (dl *dislock) lamportDistributedMutualExclusion() {
	for {
		select {
		case msg := <-dl.chanRcvMsg:
			dl.p.SetTS(msg.TS) // update its local timeStamp when receiving any message.
			// although it matters in case of the message type being Request.
			if msg.MsgType == msgp3.Request { // request lock
				dl.mu.Lock()
				heap.Push(dl.msgHeap, msg)
				dl.logger.Printf("lock(%v) receive Request message(%v) from node(%v) by connection(%v).\n", dl.nodeID, msg.String(), msg.Sender, dl.nodeConn[msg.Sender])
				if dl.waiting && !dl.checkInReplySet(msg) && dl.requestTS < msg.TS {
					dl.logger.Printf("lock(%v) defer Request message(%v) from node(%v) by connection(%v).\n", dl.nodeID, msg.String(), msg.Sender, dl.nodeConn[msg.Sender])
					dl.deferProSet.PushBack(msg)
				} else {
					// this use global timestamp.
					lr := msgp3.NewReply(dl.p.GetTS()+1, dl.nodeID, msg.Sender, msg.MsgContent.(string)+"[reply]")
					dl.sendMessage(lr)
				}
				dl.mu.Unlock()
			} else if msg.MsgType == msgp3.Reply { // reply lock
				dl.mu.Lock()
				dl.logger.Printf("lock(%v) receive Reply message(%v) from node(%v) by connection(%v).\n", dl.nodeID, msg.String(), msg.Sender, dl.nodeConn[msg.Sender])
				dl.replyProSet.PushBack(msg)
				if e := dl.checkInDeferSet(msg); e != nil {
					m := e.Value.(msgp3.Message)
					dl.logger.Printf("lock(%v) remove defer Request message(%v) from node(%v) by connection(%v).\n", dl.nodeID, m.String(), m.Sender, dl.nodeConn[m.Sender])
					dl.deferProSet.Remove(e)
					// m := e.Value.(msgp3.Message)
					lr := msgp3.NewReply(dl.p.GetTS()+1, dl.nodeID, m.Sender, m.MsgContent.(string)+"[reply]")
					dl.sendMessage(lr)
				}
				dl.checkNotify()
				dl.mu.Unlock()
			} else if msg.MsgType == msgp3.Release {
				dl.mu.Lock()
				dl.logger.Printf("lock(%v) receive Release message(%v) from node(%v) by connection(%v).\n", dl.nodeID, msg.String(), msg.Sender, dl.nodeConn[msg.Sender])
				dl.logger.Printf("============,len(dl.msgHeap)=%v======[1]======\n", dl.msgHeap.Len())
				e := heap.Pop(dl.msgHeap).(msgp3.Message)
				dl.logger.Printf("=====[1]=======,pop message(%v)", e.String())
				dl.checkNotify()
				dl.mu.Unlock()
			}
		}
	}
}

func (dl *dislock) checkNotify() {
	if dl.replyProSet.Len() == dl.peerCnt-1 && dl.msgHeap.Len() > 0 {
		msg := heap.Pop(dl.msgHeap).(msgp3.Message)
		dl.logger.Printf("=====[1]=======,pop Front message(%v) to see whether is itselft.", msg.String())
		if msg.Sender == dl.nodeID && msg.TS == dl.requestTS {
			dl.logger.Printf("lock(%v) has been notified.", dl.nodeID)
			dl.waiting = false
			heap.Push(dl.msgHeap, msg) // MUST push it back.
			dl.chanAcquireLock <- true
		} else {
			heap.Push(dl.msgHeap, msg) // MUST push it back.
		}
	}
}
func (dl *dislock) checkInDeferSet(msg msgp3.Message) *list.Element {
	for e := dl.deferProSet.Front(); e != nil; e = e.Next() {
		m := e.Value.(msgp3.Message)
		if m.Sender == msg.Sender && m.Receiver == dl.nodeID {
			return e
		}
	}
	return nil
}

func (dl *dislock) checkInReplySet(msg msgp3.Message) bool {
	for e := dl.replyProSet.Front(); e != nil; e = e.Next() {
		m := e.Value.(msgp3.Message)
		if m.Sender == msg.Sender && m.Receiver == dl.nodeID {
			return true
		}
	}
	return false
}

// TODO: handle timeout.
func (dl *dislock) Acquire(requestTS msgp3.TimeStamp, msgContent interface{}) error {
	dl.mu.Lock()
	dl.replyProSet = dl.replyProSet.Init()
	dl.deferProSet = dl.deferProSet.Init()
	dl.requestTS = requestTS
	dl.waiting = true
	for nodeID, _ := range dl.nodeConn {
		if nodeID == dl.nodeID {
			continue
		}
		// send lock request messages to all peers.
		// NOTE: all request messages will use the same timestamp.
		lr := msgp3.NewRequest(requestTS, dl.nodeID, nodeID, msgContent)
		dl.sendMessage(lr)
	}
	// add message to request queue. we roughly add a fake message(to itself) to queue,
	// as what matters is the TS and Sender not Receiver.
	heap.Push(dl.msgHeap, *msgp3.NewRequest(requestTS, dl.nodeID, dl.nodeID, msgContent))
	dl.waiting = true
	dl.mu.Unlock()
	// wait for lock grant message
	dl.logger.Printf("lock(%v) wait all node reply messages.\n", dl.nodeID)
	<-dl.chanAcquireLock
	dl.logger.Printf("lock(%v) receive all node reply messages successfully.\n", dl.nodeID)
	return nil
}

func (dl *dislock) Release(releaseTS msgp3.TimeStamp, msgContent interface{}) error {
	dl.mu.Lock()
	for nodeID, _ := range dl.nodeConn {
		if nodeID == dl.nodeID {
			continue
		}
		// send lock Release messages to all peers.
		// NOTE: all Release messages will use the global timestamp.
		lr := msgp3.NewRelease(releaseTS, dl.nodeID, nodeID, msgContent)
		dl.sendMessage(lr)
	}
	dl.logger.Printf("============,len(dl.msgHeap)=%v=====[2]=======\n", dl.msgHeap.Len())
	e := heap.Pop(dl.msgHeap).(msgp3.Message)
	dl.logger.Printf("=====[1]=======,pop message(%v), and discarded it", e.String())
	dl.logger.Printf("lock(%v) send release message to all nodes successfully.\n", dl.nodeID)
	dl.mu.Unlock()
	return nil
}

func (dl *dislock) Stat() (int, int) {
	return dl.readCnt, dl.writeCnt
}

// @see process.Close
func (dl *dislock) Close() error {
	dl.logger.Printf("lock(%v) readCount=%v, writeCount=%v.\n", dl.nodeID, dl.readCnt, dl.writeCnt)
	if err := dl.tp.Close(); err != nil {
		return err
	}
	return nil
}
