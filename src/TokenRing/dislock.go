package TokenRing

import (
	netq2 "RicartAgrawala/netq"
	msgp4 "TokenRing/msg"
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
	waiting         bool
	havetoken       bool
	mu              sync.Mutex
	chanRcvMsg      chan msgp4.Message
	chanSendMsg     chan *msgp4.Message
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
		chanRcvMsg:      make(chan msgp4.Message, MSG_BUFFERED_SIZE),
		chanSendMsg:     make(chan *msgp4.Message, MSG_BUFFERED_SIZE),
		chanAcquireLock: make(chan bool, CHAN_SIZE),
		p:               p,
	}
	dl.logger = CreateLog("log/dislock_"+strconv.Itoa(nodeID)+".log", "[dislock] ")
	tp, err := netq2.NewTcpPeer(dl.peerCnt, dl.nodeID, dl.nodePort)
	if err != nil {
		dl.logger.Printf("dislock(%v) create error: %v.\n", dl.nodeID, err.Error())
		return nil, err
	}
	dl.tp = tp
	if nodeID == 0 { // the first process have the token.
		dl.havetoken = true
		dl.logger.Printf("dilock(%v) posess the token in starting up.\n", dl.nodeID)
	}
	dl.logger.Printf("dislock(%v) created.\n", dl.nodeID)
	go dl.receiveMessage()
	go dl.tokenRing()
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
		var lg msgp4.Message
		json.Unmarshal(lgBytes, &lg)
		// dl.logger.Printf("lock(%v) receive message(%v) from connection(%v).\n", dl.nodeID, lg.String(), rcvConnID)
		dl.chanRcvMsg <- lg
	}
}

func (dl *dislock) sendMessage(msg *msgp4.Message) {
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

func (dl *dislock) tokenRing() {
	for {
		select {
		case msg := <-dl.chanRcvMsg:
			if msg.MsgType == msgp4.OK { // request lock
				dl.mu.Lock()
				dl.logger.Printf("lock(%v) receive OK message(%v) from node(%v) by connection(%v).\n", dl.nodeID, msg.String(), msg.Sender, dl.nodeConn[msg.Sender])
				if dl.waiting {
					dl.logger.Printf("lock(%v) does need the lock.\n", dl.nodeID)
					dl.havetoken = true
					dl.chanAcquireLock <- true
				} else {
					dl.logger.Printf("lock(%v) does not need the lock.\n", dl.nodeID)
					lr := msgp4.NewOK(dl.nodeID, (dl.nodeID+1)%dl.peerCnt, msg.MsgContent.(string))
					dl.sendMessage(lr)
				}
				dl.mu.Unlock()
			}
		}
	}
}

// TODO: handle timeout.
func (dl *dislock) Acquire(msgContent interface{}) error {
	dl.mu.Lock()
	// if len(dl.chanAcquireLock) > 0 {
	// 	<-dl.chanAcquireLock
	// }
	dl.logger.Printf("lock(%v) try to get the lock.\n", dl.nodeID)
	if dl.havetoken == true {
		dl.logger.Printf("lock(%v) get the lock.\n", dl.nodeID)
		// dl.chanAcquireLock <- true
		dl.mu.Unlock()
	} else {
		dl.logger.Printf("lock(%v) wait the lock.\n", dl.nodeID)
		dl.waiting = true
		dl.mu.Unlock()
		<-dl.chanAcquireLock
		dl.logger.Printf("lock(%v) get the lock.\n", dl.nodeID)
	}
	return nil
}

func (dl *dislock) Release(msgContent interface{}) error {
	dl.mu.Lock()
	dl.logger.Printf("lock(%v) release the lock.\n", dl.nodeID)
	dl.havetoken = false
	dl.waiting = false
	lr := msgp4.NewOK(dl.nodeID, (dl.nodeID+1)%dl.peerCnt, msgContent)
	dl.sendMessage(lr)
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
