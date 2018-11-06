package cms

import (
	"log"
	"os"
	"time"
)

type process struct {
	dl            *dislock
	pid           int // not really PID but identify itself , used in send message
	port          int
	lockManagerID int // centralized lock manager server pid.
	logger        *log.Logger
	work          func()
}

func NewProcess(port, pid, lockManagerID int) (*process, error) {
	p := &process{port: port, pid: pid, lockManagerID: lockManagerID}
	serverLogFile, _ := os.OpenFile("dislock.log", os.O_RDWR|os.O_CREATE, 0666)
	p.logger = log.New(serverLogFile, "[dislock] ", log.Lmicroseconds|log.Lshortfile)
	dl, err := NewDislock(port, pid)
	if err != nil {
		p.logger.Printf("process(%v) create error: %v.\n", p.pid, err.Error())
		return nil, err
	}
	p.dl = dl
	return p, nil
}

func (p *process) run() {
	// do lock task
	p.doLocalTask()
	var err error
	// begin to enter critical section, acquire lock first.
	err = p.dl.Acquire(p.lockManagerID, "") // if any process still in critical section, it will block.
	// request failure
	if err != nil {
		p.logger.Printf("process(%v) fail to acquire lock: %v.\n", p.pid, err.Error())
		return
	}
	// success, excute critical section code, operate shared resources
	p.work() // ignore any failure occurs in this stage temporaily.
	// exit critical section, release lock first.
	err = p.dl.Release(p.lockManagerID, "")
	if err != nil {
		p.logger.Printf("process(%v) fail to release lock: %v.\n", p.pid, err.Error())
		return
	}

}

// any operation involving a shared local resource can use a traditional lock
func (p *process) doLocalTask() {
	p.logger.Printf("process(%v) begin to do local task.\n", p.pid)
	time.Sleep(2000 * time.Millisecond)
	p.logger.Printf("process(%v) finish doing local task.\n", p.pid)
}
