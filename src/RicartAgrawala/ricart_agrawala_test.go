package RicartAgrawala

import (
	"fmt"
	"log"
	"math/rand"
	// "os"
	"sync"
	"testing"
	"time"
)

var (
	logger *log.Logger
)

const (
	delayIntervalLow  = 100
	delayIntervalHigh = 300
)

type ProcessStruct struct {
	p              *process
	nodeID         int
	nodeConn       map[int]int // not include itself
	readCnt        int         // total read count
	writeCnt       int         // total write count
	latency        []int64     // seconds, elapsed time cost on trying to enter the critical section
	exchangeMsgCnt []int       // number of messages exchanged when trying to enter the critical section

}

type runSystem struct {
	ps          []ProcessStruct
	peerCnt     int
	nodePort    map[int]int
	msgCnt      int // the count of entering the critical section and exiting.
	timeout     int
	processDone chan bool
	chanStat    chan bool
	mu          sync.Mutex
	phaseNum    int
}

func init() {
	logger = CreateLog("log/ricart_agrawala_test.log", "[ricart_agrawala_test]")
}

func (rs *runSystem) runProcess(wg *sync.WaitGroup, index int) {
	defer wg.Done()
	p := rs.ps[index].p
	// p.dl.writeCnt = 0
	// p.dl.readCnt = 0
	for i := 0; i < rs.msgCnt; i++ {
		err := p.Run(nil, fmt.Sprintf("message#%v", i))
		if err != nil {
			logger.Printf("process(%v) error at message(%v): %v.\n", index, i, err.Error())
			rs.processDone <- false
			return
		}
		rs.mu.Lock()
		rs.ps[index].latency = append(rs.ps[index].latency, p.latency)
		rs.mu.Unlock()
		// rs.ps[index].exchangeMsgCnt = append(rs.ps[index].exchangeMsgCnt, p.dl.writeCnt+p.dl.readCnt)
	}
	rs.processDone <- true
}

func testBasic1(t *testing.T, peerCnt, msgCnt, timeout, phaseNum int) {
	nodePort := make(map[int]int, peerCnt)
	var ps []ProcessStruct
	for i := 0; i < peerCnt; i++ {
		port := 3000 + rand.Intn(50000)
		nodeID := i // + 1000
		nodePort[nodeID] = port
	}
	for i := 0; i < peerCnt; i++ {
		nodeID := i // + 1000
		p, err := NewProcess(peerCnt, nodeID, nodePort)
		if err != nil { // not important
			logger.Printf("process(%v) create error: %v.\n", nodeID, err.Error())
			return
		}
		ps = append(ps, ProcessStruct{p: p, nodeID: nodeID})
	}
	rs := &runSystem{
		ps:          ps,
		peerCnt:     peerCnt,
		nodePort:    nodePort,
		msgCnt:      msgCnt,
		timeout:     timeout,
		processDone: make(chan bool, peerCnt),
		chanStat:    make(chan bool, 1),
		phaseNum:    phaseNum,
	}
	for i := 0; i < peerCnt; i++ {
		if err := rs.ps[i].p.BuildAllConn(); err != nil {
			logger.Fatalf("peer(%v) build rpc connection error: %v.\n", rs.ps[i].nodeID, err.Error())
		}
		logger.Printf("peer(%v) build rpc connection successfully.\n", rs.ps[i].nodeID)
	}
	var wg sync.WaitGroup
	wg.Add(peerCnt)
	// phase 1
	logger.Printf("Test for phase 1.\n")
	for i := 0; i < peerCnt; i++ {
		go rs.runProcess(&wg, i)
	}
	// phase 2
	if rs.phaseNum == 2 {
		// two lines below is IMPORTANT, otherwise need to synchronize code of stats counting in dislock.go.
		wg.Wait()
		wg.Add(peerCnt)
		for i := 0; i < peerCnt; i++ {
			if i%2 == 0 {
				time.Sleep(time.Duration(rand.Intn((delayIntervalHigh-delayIntervalLow)+delayIntervalLow)) * time.Millisecond)
			}
			go rs.runProcess(&wg, i)
		}
	}
	timeoutChan := time.After(time.Duration(rs.timeout) * time.Millisecond)
	go func() {
		processDoneCnt := 0
		for {
			select {
			case <-rs.processDone:
				processDoneCnt++
				if processDoneCnt == rs.peerCnt*rs.phaseNum {
					for i := 0; i < rs.peerCnt; i++ {
						rs.ps[i].readCnt, rs.ps[i].writeCnt = rs.ps[i].p.Stat()
					}
					logger.Printf("all peers tasks are done.\n")
					// time.Sleep(500 * time.Millisecond)
					rs.Close()
					rs.chanStat <- true
					return
				}
			case <-timeoutChan:
				rs.Close()
				logger.Fatalf("Test time out after %v secs.\n", rs.timeout)
				return
			}
		}
	}()

	wg.Wait()
	<-rs.chanStat
	// print stats info
	rs.printStats()
	// validate global variable, i.e. shared resource status
	rs.checkGlobalCnt()
}

func (rs *runSystem) Close() {
	logger.Printf("begin to close all connections.\n")
	for i := 0; i < rs.peerCnt; i++ {
		if rs.ps[i].p != nil {
			if err := rs.ps[i].p.Close(); err != nil {
				logger.Printf("Close node(%v) error.\n", i)
			}
		}
	}
}

func (rs *runSystem) printStats() {
	totalReadCnt := 0
	totalWriteCnt := 0
	totalDelay := int64(0)
	logger.Printf("==========================numProcess=%v, numMsg=%v, numPhase=%v==========================\n", rs.peerCnt, rs.msgCnt, rs.phaseNum)
	for i := 0; i < rs.peerCnt; i++ {
		totalPeerDelay := int64(0)
		logger.Printf("process(%v) readCount=%v, writeCount=%v.\n", i, rs.ps[i].readCnt, rs.ps[i].writeCnt)
		totalReadCnt += rs.ps[i].readCnt
		totalWriteCnt += rs.ps[i].writeCnt
		for j := 0; j < len(rs.ps[i].latency); j++ {
			// logger.Printf("process(%v) cost time(%v) waiting for distributed lock at message(%v).\n", i, rs.ps[i].latency[j], j)
			totalPeerDelay += rs.ps[i].latency[j]
		}
		totalDelay += totalPeerDelay
		logger.Printf("process(%v) mean delay=%v to acquire distributed lock for %v messages.\n", i, float32(totalPeerDelay)/float32(len(rs.ps[i].latency)), len(rs.ps[i].latency))
	}
	logger.Printf("totalReadCount=%v, totalWriteCount=%v.\n", totalReadCnt, totalWriteCnt)
	logger.Printf("mean delay=%v to acquire distributed lock for %v process and %v messages per process.\n", float32(totalDelay)/(float32(len(rs.ps[0].latency)*rs.peerCnt)), rs.peerCnt, len(rs.ps[0].latency))
}

// @see process.go
// the function will roughly check whether it's safe to share the globa count variable.
// in addition, you need to manually view the validateGlobalCount.log.
func (rs *runSystem) checkGlobalCnt() {
	logger.Printf("begin to check global count status.")
	for i := 1; i < len(globalCntArray); i++ {
		if globalCntArray[i] <= globalCntArray[i-1] {
			logger.Printf("Fail to synchronize share global count, at the value=%v.", globalCntArray[i])
			return
		}
	}
	logger.Printf("Synchronize share global count successfully.")
}

func TestRicartAgrawala1(t *testing.T) {
	phaseNum := 2
	peerCnt := 2
	msgCnt := 1
	timeout := 1000 * phaseNum
	testBasic1(t, peerCnt, msgCnt, timeout, phaseNum)
}

func TestRicartAgrawala2(t *testing.T) {
	phaseNum := 2
	peerCnt := 2
	msgCnt := 5
	timeout := 5000 * phaseNum
	testBasic1(t, peerCnt, msgCnt, timeout, phaseNum)
}

func TestRicartAgrawala3(t *testing.T) {
	phaseNum := 2
	peerCnt := 2
	msgCnt := 50
	timeout := 30000 * phaseNum
	testBasic1(t, peerCnt, msgCnt, timeout, phaseNum)
}

func TestRicartAgrawala4(t *testing.T) {
	phaseNum := 2
	peerCnt := 5
	msgCnt := 2
	timeout := 2000 * phaseNum
	testBasic1(t, peerCnt, msgCnt, timeout, phaseNum)
}

func TestRicartAgrawala5(t *testing.T) {
	phaseNum := 2
	peerCnt := 5
	msgCnt := 5
	timeout := 5000 * phaseNum
	testBasic1(t, peerCnt, msgCnt, timeout, phaseNum)
}

func TestRicartAgrawala6(t *testing.T) {
	phaseNum := 2
	peerCnt := 5
	msgCnt := 50
	timeout := 60000 * 2
	testBasic1(t, peerCnt, msgCnt, timeout, phaseNum)
}

func TestRicartAgrawala7(t *testing.T) {
	phaseNum := 2
	peerCnt := 10
	msgCnt := 50
	timeout := 120000 * phaseNum
	testBasic1(t, peerCnt, msgCnt, timeout, phaseNum)
}

func TestRicartAgrawala8(t *testing.T) {
	phaseNum := 2
	peerCnt := 10
	msgCnt := 100
	timeout := 180000 * phaseNum
	testBasic1(t, peerCnt, msgCnt, timeout, phaseNum)
}

func TestRicartAgrawala9(t *testing.T) {
	phaseNum := 2
	peerCnt := 20
	msgCnt := 100
	timeout := 360000 * phaseNum
	testBasic1(t, peerCnt, msgCnt, timeout, phaseNum)
}
