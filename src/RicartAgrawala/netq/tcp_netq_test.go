package netq2

import (
	msgp2 "RicartAgrawala/msg"
	"encoding/json"
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

func init() {
	logger = CreateLog("log/tcp_netq_test.log", "[tcp_netq_test]")
}

type msgData struct {
	content string
}

type TcpPeerStruct struct {
	tp           TcpPeer
	nodeID       int
	peerReadCnt  int
	peerWriteCnt int
	nodeConn     map[int]int // not include itself
}

type runSystem struct {
	tps      []TcpPeerStruct
	nodePort map[int]int
	peerCnt  int
	msgCnt   int
	timeout  int
	peerDone chan bool
}

func (rs *runSystem) runPeer(wg *sync.WaitGroup, index int) {
	defer wg.Done()
	tp := rs.tps[index].tp
	var ts msgp2.TimeStamp
	tpID := rs.tps[index].nodeID
	for nodeID, _ := range rs.nodePort {
		if nodeID == tpID {
			continue
		}
		logger.Printf("peer(%v) begin to communicate with node(%v) by connection(%v).\n", tpID, nodeID, rs.tps[index].nodeConn[nodeID])
		for i := 0; i < rs.msgCnt; i++ {
			// write
			ts = msgp2.TimeStamp(i + 1000)
			smsg := msgp2.NewRequest(ts, tpID, nodeID, fmt.Sprintf("This is content of [%d,%d]", tpID, i))
			logger.Printf("peer(%v) write message(%v) to node(%v) by connection(%v).\n", tpID, smsg.String(), nodeID, rs.tps[index].nodeConn[nodeID])
			mdBytes, _ := json.Marshal(smsg)
			if err := tp.WriteData(rs.tps[index].nodeConn[nodeID], mdBytes); err != nil {
				logger.Printf("peer(%v) write error: %v.\n", tpID, err.Error())
				rs.peerDone <- false
				return
			}
			rs.tps[index].peerWriteCnt++
			// read
			recvConnID, readBytes, err := tp.ReadData()
			if err != nil {
				logger.Printf("peer(%v) read from connection(%v) error: %v.\n", tpID, recvConnID, err.Error())
				rs.peerDone <- false
				return
			}
			var rmsg msgp2.Message
			json.Unmarshal(readBytes, &rmsg)
			// logger.Printf("Peer read data from server %v.\n", readBytes)
			logger.Printf("peer(%v) read message(%v) from node(%v) by connnection(%v).\n", tpID, rmsg.String(), nodeID, recvConnID)
			rs.tps[index].peerReadCnt++
		}
	}
	// tp.Close()
	rs.peerDone <- true
	return
}

func testBasic1(t *testing.T, peerCnt, msgCnt, timeout int) {
	var tps []TcpPeerStruct
	nodePort := make(map[int]int, peerCnt)
	for i := 0; i < peerCnt; i++ {
		port := 3000 + rand.Intn(50000)
		nodeID := i // + 1000
		nodePort[nodeID] = port
	}
	for i := 0; i < peerCnt; i++ {
		nodeID := i // + 1000
		tp, err := NewTcpPeer(peerCnt, nodeID, nodePort)
		if err != nil { // not important
			logger.Printf("peer(%v) create error: %v.\n", nodeID, err.Error())
			return
		}
		tps = append(tps, TcpPeerStruct{tp: tp, nodeID: nodeID})
	}
	rs := &runSystem{
		tps:      tps,
		peerCnt:  peerCnt,
		nodePort: nodePort,
		msgCnt:   msgCnt,
		timeout:  timeout,
		peerDone: make(chan bool, len(tps)),
	}
	for i := 0; i < peerCnt; i++ {
		if nodeConnMap, err := rs.tps[i].tp.BuildAllConn(); err != nil {
			nodeID := i // + 1000
			logger.Printf("peer(%v) build rpc connection error: %v.\n", nodeID, err.Error())
		} else {
			rs.tps[i].nodeConn = nodeConnMap
		}
	}
	var wg sync.WaitGroup
	wg.Add(peerCnt)
	logger.Printf("Test for phase 1.\n")
	for i := 0; i < peerCnt; i++ {
		go rs.runPeer(&wg, i)
	}

	timeoutChan := time.After(time.Duration(rs.timeout) * time.Millisecond)
	go func() {
		peerDoneCnt := 0
		for {
			select {
			case <-rs.peerDone:
				peerDoneCnt++
				if peerDoneCnt == len(rs.tps) {
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
}

func (rs *runSystem) Close() {
	for i := 0; i < rs.peerCnt; i++ {
		if rs.tps[i].tp != nil {
			rs.tps[i].tp.Close()
		}
	}
	logger.Printf("close all connections successfully.\n")
}

func (rs *runSystem) printStats() {
	logger.Printf("==========================numProcess=%v, numMsg=%v, numPhase=2==========================\n", rs.peerCnt, rs.msgCnt)
	totalReadCnt := 0
	totalWriteCnt := 0
	for i := 0; i < rs.peerCnt; i++ {
		logger.Printf("peerent(%v) readCount=%v, writeCount=%v.\n", i, rs.tps[i].peerReadCnt, rs.tps[i].peerWriteCnt)
		totalReadCnt += rs.tps[i].peerReadCnt
		totalWriteCnt += rs.tps[i].peerWriteCnt
	}
	logger.Printf("totalReadCount=%v, totalWriteCount=%v.\n", totalReadCnt, totalWriteCnt)
}

func Test1(t *testing.T) {
	peerCnt := 2
	msgCnt := 1
	timeout := 2000
	testBasic1(t, peerCnt, msgCnt, timeout)
}

func Test2(t *testing.T) {
	peerCnt := 2
	msgCnt := 10
	timeout := 2000
	testBasic1(t, peerCnt, msgCnt, timeout)
}

func Test3(t *testing.T) {
	peerCnt := 2
	msgCnt := 100
	timeout := 2000
	testBasic1(t, peerCnt, msgCnt, timeout)
}

func Test4(t *testing.T) {
	peerCnt := 5
	msgCnt := 100
	timeout := 2000
	testBasic1(t, peerCnt, msgCnt, timeout)
}

func Test5(t *testing.T) {
	peerCnt := 10
	msgCnt := 100
	timeout := 2000
	testBasic1(t, peerCnt, msgCnt, timeout)
}

func Test6(t *testing.T) {
	peerCnt := 20
	msgCnt := 100
	timeout := 2000
	testBasic1(t, peerCnt, msgCnt, timeout)
}
