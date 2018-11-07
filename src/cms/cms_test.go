package cms

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
	readCnt        int     // total read count
	writeCnt       int     // total write count
	latency        []int64 // elapsed time cost on trying to enter the critical section
	exchangeMsgCnt []int   //  number of messages exchanged when trying to enter the critical section
}

type CentLockMangStruct struct {
	clm *centLockMang
	// readCnt  int // total read count
	// writeCnt int // total write count
}

type runSystem struct {
	ps          []ProcessStruct
	clms        CentLockMangStruct
	pCnt        int
	managerID   int
	iterCnt     int // the count of entering the critical section and exiting.
	timeout     int
	processDone chan bool
	mu          sync.Mutex
}

func init() {
	logger = CreateLog("log/cms_test.log", "[cms_test]")
}

func (rs *runSystem) runProcess(wg *sync.WaitGroup, index int) {
	defer wg.Done()
	p := rs.ps[index].p
	p.dl.writeCnt = 0
	p.dl.readCnt = 0
	for i := 0; i < rs.iterCnt; i++ {
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
	rs.mu.Lock()
	rs.ps[index].writeCnt += p.dl.writeCnt
	rs.ps[index].readCnt += p.dl.readCnt
	rs.mu.Unlock()
	rs.processDone <- true
}

func (rs *runSystem) runManager(wg *sync.WaitGroup) {
	defer wg.Done()
	if err := rs.clms.clm.Start(); err != nil {
		return
	}
}

func testBasic1(t *testing.T, pCnt, managerID, iterCnt, timeout int) {
	port := 3000 + rand.Intn(50000)
	clm, err := NewCentLockMang(port, managerID)
	if err != nil {
		logger.Printf("Start centralized server manager(%v) error: %v.\n", managerID, err.Error())
		return
	}
	clms := CentLockMangStruct{clm: clm}
	var ps []ProcessStruct
	for i := 0; i < pCnt; i++ {
		p, err := NewProcess(port, i, managerID)
		if err != nil {
			logger.Printf("client create error: %v.\n", err.Error())
			return
		}
		ps = append(ps, ProcessStruct{p: p})
	}
	rs := &runSystem{
		ps:          ps,
		clms:        clms,
		pCnt:        pCnt,
		managerID:   managerID,
		iterCnt:     iterCnt,
		timeout:     timeout,
		processDone: make(chan bool, 2*pCnt),
	}
	var wg sync.WaitGroup
	wg.Add(pCnt)
	go rs.runManager(&wg)
	// phase 1
	for i := 0; i < pCnt; i++ {
		go rs.runProcess(&wg, i)
	}
	// two lines below is IMPORTANT, otherwise need to synchronize code of stats counting in dislock.go.
	wg.Wait()
	wg.Add(pCnt + 1)
	// phase 2
	for i := 0; i < pCnt; i++ {
		if i%2 == 0 {
			time.Sleep(time.Duration(rand.Intn((delayIntervalHigh-delayIntervalLow)+delayIntervalLow)) * time.Millisecond)
		}
		go rs.runProcess(&wg, i)
	}
	timeoutChan := time.After(time.Duration(rs.timeout) * time.Millisecond)
	go func() {
		processDoneCnt := 0
		for {
			select {
			case <-rs.processDone:
				processDoneCnt++
				if processDoneCnt == 2*rs.pCnt {
					time.Sleep(500 * time.Millisecond)
					rs.Close()
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
	// print stats info
	rs.printStats()
	// validate global variable, i.e. shared resource status
	rs.checkGlobalCnt()
}

func (rs *runSystem) Close() {
	for i := 0; i < rs.pCnt; i++ {
		if rs.ps[i].p != nil {
			rs.ps[i].p.Close()
		}
	}
	if rs.clms.clm != nil {
		rs.clms.clm.Close()
	}
	// logger.Printf("close all connections successfully.\n")
}

func (rs *runSystem) printStats() {
	totalReadCnt := 0
	totalWriteCnt := 0
	logger.Printf("==========================numProcess=%v, numMsg=%v, numPhase=2==========================\n", rs.pCnt, rs.iterCnt)
	for i := 0; i < rs.pCnt; i++ {
		logger.Printf("process(%v) readCount=%v, writeCount=%v.\n", i, rs.ps[i].readCnt, rs.ps[i].writeCnt)
		totalReadCnt += rs.ps[i].readCnt
		totalWriteCnt += rs.ps[i].writeCnt
		for j := 0; j < len(rs.ps[i].latency); j++ {
			logger.Printf("process(%v) cost time(%v) waiting for distributed lock at message(%v).\n", i, rs.ps[i].latency[j], j)
		}
	}
	logger.Printf("lockManager(%v) readCount=%v, writeCount=%v.\n", rs.managerID, rs.clms.clm.readCnt, rs.clms.clm.writeCnt)
	totalReadCnt += rs.clms.clm.readCnt
	totalWriteCnt += rs.clms.clm.writeCnt
	logger.Printf("totalReadCount=%v, totalWriteCount=%v.\n", totalReadCnt, totalWriteCnt)
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

func Test1(t *testing.T) {
	pCnt := 1
	managerID := 100
	iterCnt := 1
	timeout := 1000
	testBasic1(t, pCnt, managerID, iterCnt, timeout)
}

func Test2(t *testing.T) {
	pCnt := 1
	managerID := 100
	iterCnt := 5
	timeout := 2000
	testBasic1(t, pCnt, managerID, iterCnt, timeout)
}

func Test3(t *testing.T) {
	pCnt := 1
	managerID := 100
	iterCnt := 50
	timeout := 15000
	testBasic1(t, pCnt, managerID, iterCnt, timeout)
}

func Test4(t *testing.T) {
	pCnt := 2
	managerID := 100
	iterCnt := 1
	timeout := 2000
	testBasic1(t, pCnt, managerID, iterCnt, timeout)
}

func Test5(t *testing.T) {
	pCnt := 2
	managerID := 100
	iterCnt := 5
	timeout := 5000
	testBasic1(t, pCnt, managerID, iterCnt, timeout)
}

func Test6(t *testing.T) {
	pCnt := 2
	managerID := 100
	iterCnt := 50
	timeout := 20000
	testBasic1(t, pCnt, managerID, iterCnt, timeout)
}

func Test7(t *testing.T) {
	pCnt := 5
	managerID := 100
	iterCnt := 50
	timeout := 180000
	testBasic1(t, pCnt, managerID, iterCnt, timeout)
}

func Test8(t *testing.T) {
	pCnt := 10
	managerID := 100
	iterCnt := 100
	timeout := 360000
	testBasic1(t, pCnt, managerID, iterCnt, timeout)
}

func Test9(t *testing.T) {
	pCnt := 20
	managerID := 100
	iterCnt := 100
	timeout := 360000
	testBasic1(t, pCnt, managerID, iterCnt, timeout)
}
