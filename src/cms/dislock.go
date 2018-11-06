// a centralized mutex server implementation
// dislock implementation
package cms

import (
	msgp "cms/msg"
	netq "cms/netq"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	// "sync"
)

type dislock struct {
	cli    netq.Client
	lockID int // regard it as process id.
	port   int
	logger *log.Logger
}

func NewDislock(port, lockID int) (*dislock, error) {
	dl := &dislock{port: port, lockID: lockID}
	serverLogFile, _ := os.OpenFile("dislock.log", os.O_RDWR|os.O_CREATE, 0666)
	dl.logger = log.New(serverLogFile, "[dislock] ", log.Lmicroseconds|log.Lshortfile)
	cli, err := netq.NewClient(dl.port)
	if err != nil {
		dl.logger.Printf("dislock(%v) create error: %v.\n", dl.lockID, err.Error())
		return nil, err
	}
	dl.cli = cli
	return dl, nil
}

// TODO: handle timeout.
func (dl *dislock) Acquire(receiver int, msgContent interface{}) error {
	// send lock request message.
	lr := msgp.NewRequest(dl.lockID, receiver, msgContent)
	lrBytes, _ := json.Marshal(lr)
	if err := dl.cli.WriteData(lrBytes); err != nil {
		dl.logger.Printf("lock(%v) send request message(%v) error: %v.\n", dl.lockID, lr.String(), err.Error())
		return err
	}
	// wait for lock grant message
	dl.logger.Printf("lock(%v) wait grant message from server.\n", dl.lockID)
	lgBytes, err := dl.cli.ReadData()
	if err != nil {
		dl.logger.Printf("lock(%v) receive Grant message error: %v.\n", dl.lockID, err.Error())
		return err
	}
	var lg msgp.Message
	json.Unmarshal(lgBytes, &lg)
	if lg.MsgType == msgp.Grant {
		// neglect message content.
		dl.logger.Printf("lock(%v) receive Grant message(%v) from server.\n", dl.lockID, lg.String())
		return nil
	} else {
		errMsg := fmt.Sprintf("lock(%v) receive error message(%v) from server.\n", dl.lockID, lg.String())
		dl.logger.Printf(errMsg)
		return errors.New(errMsg)
	}
}

func (dl *dislock) Release(receiver int, msgContent interface{}) error {
	// send lock release message.
	lrl := msgp.NewRelease(dl.lockID, receiver, msgContent)
	lrlBytes, _ := json.Marshal(lrl)
	if err := dl.cli.WriteData(lrlBytes); err != nil {
		dl.logger.Printf("lock(%v) send release message(%v) error: %v.\n", dl.lockID, lrl.String(), err.Error())
		return err
	}
	dl.logger.Printf("lock(%v) send release message(%v) successfully.\n", dl.lockID, lrl.String())
	return nil
}
