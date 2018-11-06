// a centralized mutex server implementation
// a single server that acts as a lock manager. It maintains queue Q containing lock requests that have not yet been granted.
package cms

import (
	"container/list"
	// "fmt"
	msgp "cms/msg"
	netq "cms/netq"
	"encoding/json"
	"log"
	"os"
	// "sync"
)

const (
	MSG_BUFFERED_SIZE = 100
)

type centLockMang struct {
	managerID    int        // regrad it as manager process id.
	processQueue *list.List // FIFO
	srv          netq.Server
	port         int
	chanRecvMsg  chan msgComp
	logger       *log.Logger
}

type msgComp struct {
	msg    msgp.Message
	connID int
}

func NewCentLockMang(port, managerID int) (*centLockMang, error) {
	clm := &centLockMang{
		managerID:   managerID,
		port:        port,
		chanRecvMsg: make(chan msgComp, MSG_BUFFERED_SIZE),
	}
	serverLogFile, _ := os.OpenFile("centLockMang.log", os.O_RDWR|os.O_CREATE, 0666)
	clm.logger = log.New(serverLogFile, "[centLockMang] ", log.Lmicroseconds|log.Lshortfile)
	srv, err := netq.NewServer(clm.port)
	if err != nil {
		clm.logger.Printf("centLockMang create error: %v.\n", err.Error())
		return nil, err
	}
	clm.srv = srv
	clm.logger.Printf("centLockMang create successfully.\n")
	return clm, nil
}

func (clm *centLockMang) Start() error {
	go clm.handleLockMsg()
	for {
		connID, readBytes, err := clm.srv.ReadData()
		if err != nil {
			clm.logger.Printf("centLockMang receive message error: %v.\n", err.Error())
			continue
			// return
		}
		var msg msgp.Message
		json.Unmarshal(readBytes, &msg)
		clm.logger.Printf("centLockMang receive message(%v) from process %v: %v.\n", msg.String(), msg.Sender, msg.String())
		clm.chanRecvMsg <- msgComp{connID: connID, msg: msg}
	}
}

func (clm *centLockMang) handleLockMsg() {
	for {
		select {
		case msgComp := <-clm.chanRecvMsg:
			message := msgComp.msg
			switch message.MsgType {
			case msgp.Request:
				if clm.processQueue.Len() == 0 {
					if err := clm.sendGrantMsg(message.Receiver, message.Sender, msgComp.connID, nil); err != nil {
						// return // TODO: handle error
						continue
					}
				} else {
					clm.processQueue.PushBack(message.Sender)
					clm.logger.Printf("centLockMang defer response to process(%v).\n", message.Sender)
				}
			case msgp.Release:
				if clm.processQueue.Len() > 0 {
					sender := clm.processQueue.Front().Value
					// clm.managerID
					if err := clm.sendGrantMsg(message.Receiver, sender.(int), msgComp.connID, nil); err != nil {
						// return // TODO: handle error
						continue
					}
				}
			case msgp.Grant:
				clm.logger.Printf("Error message(%v) type Grant.\n", message.String())
			}
		}

	}
}

func (clm *centLockMang) sendGrantMsg(sender, receiver, connID int, content interface{}) error {
	// write
	lg := msgp.NewGrant(sender, receiver, content.(string)+"#[grant]")
	lgBytes, _ := json.Marshal(lg)
	if err := clm.srv.WriteData(connID, lgBytes); err != nil {
		clm.logger.Printf("centLockMang send message(%v) to process(%v) error: %v.\n", lg.String(), lg.Receiver, err.Error())
		return err
	}
	clm.logger.Printf("centLockMang send message(%v) to process(%v) successfully.\n", lg.String(), lg.Receiver)
	return nil
}
