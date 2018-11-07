package cms

import (
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	localTaskIntervalLow     = 100
	localTaskIntervalHigh    = 300
	criticalTaskIntervalLow  = 100
	criticalTaskIntervalHigh = 200
)

type process struct {
	dl            *dislock
	pid           int // not really PID but identify itself , used in send message
	port          int
	lockManagerID int // centralized lock manager server pid.
	logger        *log.Logger
	work          func()

	// stat info
	latency int64 //elapsed time between making a request and being able to enter the critical section
}

var validateLogger *log.Logger = CreateLog("log/validateGlobalCount.log", "")

// just for facilitating our testing cases.
var globalCnt int
var globalCntArray []int

func CreateLog(fileName, header string) *log.Logger {
	newpath := filepath.Join(".", "log")
	os.MkdirAll(newpath, os.ModePerm)
	serverLogFile, _ := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	return log.New(serverLogFile, header, log.Lmicroseconds|log.Lshortfile)
}

func NewProcess(port, pid, lockManagerID int) (*process, error) {
	p := &process{port: port, pid: pid, lockManagerID: lockManagerID}
	p.logger = CreateLog("log/process_"+strconv.Itoa(pid)+".log", "[process]")
	dl, err := NewDislock(port, pid)
	if err != nil {
		p.logger.Printf("process(%v) create error: %v.\n", p.pid, err.Error())
		return nil, err
	}
	p.dl = dl
	return p, nil
}

func (p *process) Run(w func(), msgContent string) error {
	if w != nil {
		p.work = w
	} else {
		p.work = p.defaultWork
	}
	// do lock task
	p.doLocalTask()
	var err error
	startTime := time.Now().Unix()
	// begin to enter critical section, acquire lock first.
	err = p.dl.Acquire(p.lockManagerID, msgContent) // if any process still in critical section, it will block.
	// request failure
	if err != nil {
		p.logger.Printf("process(%v) fail to acquire lock: %v.\n", p.pid, err.Error())
		return err
	}
	endTime := time.Now().Unix()
	p.latency = endTime - startTime
	p.logger.Printf("process(%v) entered the critical section at %v.\n", p.pid, time.Unix(time.Now().Unix(), 0).Format("2006-01-02 15:04:05"))
	// success, excute critical section code, operate shared resources
	p.work() // ignore any failure occurs in this stage temporaily.
	// exit critical section, release lock first.
	err = p.dl.Release(p.lockManagerID, msgContent+"[Release]")
	if err != nil {
		p.logger.Printf("process(%v) fail to release lock: %v.\n", p.pid, err.Error())
		return err
	}
	p.logger.Printf("process(%v) exited the critical section.\n", p.pid)
	return nil
}

// the method is not good in usage logical, because the lock will automatically close when process called Release.
// so it just mainly facilitate our testing cases.
func (p *process) Close() error {
	if err := p.dl.Close(); err != nil {
		return err
	}
	return nil
}

// any operation involving a shared local resource can use a traditional lock
func (p *process) doLocalTask() {
	p.logger.Printf("process(%v) begin to do local task.\n", p.pid)
	// sleep for a time between localTaskIntervalLow and localTaskIntervalHigh
	time.Sleep(time.Duration(rand.Intn((localTaskIntervalHigh-localTaskIntervalLow)+localTaskIntervalLow)) * time.Millisecond)
	p.logger.Printf("process(%v) finish doing local task.\n", p.pid)
}

func (p *process) defaultWork() {
	// sleep for a time between criticalTaskIntervalLow and criticalTaskIntervalHigh
	// p.logger.Printf("process(%v) begin to handle globalCnt=%v.\n", p.pid, globalCnt)
	validateLogger.Printf("process(%v) begin to handle globalCnt=%v.\n", p.pid, globalCnt)
	timeoutChan := time.After(time.Duration(rand.Intn((criticalTaskIntervalHigh-criticalTaskIntervalLow)+criticalTaskIntervalLow)) * time.Millisecond)
	for {
		select {
		case <-timeoutChan:
			// p.logger.Printf("process(%v) end handle globalCnt=%v.\n", p.pid, globalCnt)
			validateLogger.Printf("process(%v) end handle globalCnt=%v.\n", p.pid, globalCnt)
			globalCntArray = append(globalCntArray, globalCnt)
			return
		default:
			globalCnt++
			time.Sleep(100 * time.Millisecond)
		}
	}
}
