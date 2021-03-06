package RicartAgrawala

import (
	msgp2 "RicartAgrawala/msg"
	netq2 "RicartAgrawala/netq"
	"container/list"
	"encoding/json"
	// "errors"
	// "fmt"
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
	shouldDefer     bool
	requestTS       msgp2.TimeStamp
	replyProSet     *list.List
	deferProSet     *list.List
	mu              sync.Mutex
	chanRcvMsg      chan msgp2.Message
	chanSendMsg     chan *msgp2.Message
	chanAcquireLock chan bool
	logger          *log.Logger

	// process handler
	p *process
	// sata info
	readCnt  int
	writeCnt int
}

func NewDislock(peerCnt, nodeID int, nodePort map[int]int, p *process) (*dislock, error) {
	dl := &dislock{
		nodeID:          nodeID,
		nodePort:        nodePort,
		peerCnt:         peerCnt,
		chanRcvMsg:      make(chan msgp2.Message, MSG_BUFFERED_SIZE),
		chanSendMsg:     make(chan *msgp2.Message, MSG_BUFFERED_SIZE),
		chanAcquireLock: make(chan bool, CHAN_SIZE),
		replyProSet:     list.New(),
		deferProSet:     list.New(),
		p:               p,
	}
	dl.logger = CreateLog("log/dislock_"+strconv.Itoa(nodeID)+".log", "[dislock] ")
	tp, err := netq2.NewTcpPeer(dl.peerCnt, dl.nodeID, dl.nodePort)
	if err != nil {
		dl.logger.Printf("dislock(%v) create error: %v.\n", dl.nodeID, err.Error())
		return nil, err
	}
	dl.tp = tp
	dl.logger.Printf("dislock(%v) created.\n", dl.nodeID)
	go dl.receiveMessage()
	go dl.ricartAgrawala()
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
		var lg msgp2.Message
		json.Unmarshal(lgBytes, &lg)
		// dl.logger.Printf("lock(%v) receive message(%v) from connection(%v).\n", dl.nodeID, lg.String(), rcvConnID)
		dl.chanRcvMsg <- lg
	}
}

func (dl *dislock) sendMessage(msg *msgp2.Message) {
	connID, _ := dl.nodeConn[msg.Receiver]
	lrlBytes, _ := json.Marshal(msg)
	if err := dl.tp.WriteData(connID, lrlBytes); err != nil {
		dl.logger.Printf("lock(%v) send message(%v) to node(%v) by connection(%v) error: %v.\n", dl.nodeID, msg.String(), msg.Receiver, connID, err.Error())
		dl.shouldDefer = !dl.shouldDefer
		// return err // TODO: handle write failure.
	}
	dl.writeCnt++
	dl.logger.Printf("lock(%v) send message(%v) to node(%v) by connection(%v) successfully.\n", dl.nodeID, msg.String(), msg.Receiver, connID)
}

func (dl *dislock) ricartAgrawala() {
	for {
		select {
		case msg := <-dl.chanRcvMsg:
			dl.p.SetTS(msg.TS)
			if msg.MsgType == msgp2.Request { // request lock
				// neglect message content.
				dl.mu.Lock()
				dl.logger.Printf("lock(%v) receive Request message(%v) from node(%v) by connection(%v).\n", dl.nodeID, msg.String(), msg.Sender, dl.nodeConn[msg.Sender])
				if dl.shouldDefer && dl.requestTS < msg.TS {
					dl.logger.Printf("lock(%v) defer Request message(%v) from node(%v) by connection(%v).\n", dl.nodeID, msg.String(), msg.Sender, dl.nodeConn[msg.Sender])
					dl.deferProSet.PushBack(msg)
				} else {
					lr := msgp2.NewReply(msg.TS, dl.nodeID, msg.Sender, msg.MsgContent.(string)+"[reply]")
					dl.sendMessage(lr)
				}
				dl.mu.Unlock()
			} else { // reply lock
				dl.mu.Lock()
				dl.logger.Printf("lock(%v) receive Reply message(%v) from node(%v) by connection(%v).\n", dl.nodeID, msg.String(), msg.Sender, dl.nodeConn[msg.Sender])
				dl.replyProSet.PushBack(msg)
				if dl.replyProSet.Len() == dl.peerCnt-1 {
					dl.chanAcquireLock <- true
				}
				dl.mu.Unlock()
			}
		}
	}
}

// TODO: handle timeout.
func (dl *dislock) Acquire(requestTS msgp2.TimeStamp, msgContent interface{}) error {
	dl.mu.Lock()
	dl.replyProSet = dl.replyProSet.Init()
	dl.deferProSet = dl.deferProSet.Init()
	dl.requestTS = requestTS
	dl.shouldDefer = true
	dl.mu.Unlock()
	for nodeID, _ := range dl.nodeConn {
		if nodeID == dl.nodeID {
			continue
		}
		// send lock request messages to all peers.
		// NOTE: all request messages will use the same timestamp.
		lr := msgp2.NewRequest(requestTS, dl.nodeID, nodeID, msgContent)
		dl.sendMessage(lr)
	}
	// wait for lock grant message
	dl.logger.Printf("lock(%v) wait all node reply messages.\n", dl.nodeID)
	<-dl.chanAcquireLock

	dl.logger.Printf("lock(%v) receive all node reply messages successfully.\n", dl.nodeID)
	return nil
}

func (dl *dislock) Release(msgContent interface{}) error {
	dl.mu.Lock()
	dl.shouldDefer = false
	for e := dl.deferProSet.Front(); e != nil; e = e.Next() {
		msg := e.Value.(msgp2.Message)
		lr := msgp2.NewReply(msg.TS, dl.nodeID, msg.Sender, msg.MsgContent.(string)+"[reply]")
		dl.sendMessage(lr)
	}
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
