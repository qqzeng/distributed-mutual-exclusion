package netq2

import (
	// msgp2 "RicartAgrawala/msg"
	// "encoding/json"
	// "fmt"
	"log"
	"math/rand"
	"os"
	// "sync"
	"testing"
	// "time"
)

var (
	logger *log.Logger
)

func init() {
	serverLogFile, _ := os.OpenFile("log_rpc_netq.log", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	logger = log.New(serverLogFile, "[rpc_netq_test]", log.Lmicroseconds|log.Lshortfile)
}

type msgData struct {
	content string
}

type PeerStruct struct {
	peer         Peer
	peerReadCnt  int
	peerWriteCnt int
}

type runSystem struct {
	peers    []PeerStruct
	nodePort map[int]int
	peerCnt  int
	msgCnt   int
	timeout  int
	peerDone chan bool
}

// func (rs *runSystem) runPeer(wg *sync.WaitGroup, index int, managerID int) {
// 	defer wg.Done()
// 	p := rs.peers[index].peer
// 	// write
// 	for i := 0; i < rs.msgCnt; i++ {
// 		s := msgp.NewRequest(index, managerID, fmt.Sprintf("This is content of [%d,%d]", index, i))
// 		mdBytes, _ := json.Marshal(s)
// 		if err := p.WriteData(mdBytes); err != nil {
// 			logger.Printf("peerent write error: %v.\n", err.Error())
// 			rs.peerDone <- false
// 			return
// 		}
// 		rs.peers[index].peerWriteCnt++
// 		// read
// 		readBytes, err := p.ReadData()
// 		if err != nil {
// 			logger.Printf("peerent read error: %v.\n", err.Error())
// 			rs.peerDone <- false
// 			return
// 		}
// 		var r msgp.Message
// 		json.Unmarshal(readBytes, &r)
// 		// logger.Printf("Peer read data from server %v.\n", readBytes)
// 		logger.Printf("peerent read message from server: %v.\n", r.String())
// 		rs.peers[index].peerReadCnt++
// 	}
// 	// p.Close()
// 	rs.peerDone <- true
// 	return
// }

func testBasic1(t *testing.T, peerCnt, msgCnt, timeout int) {
	var peers []PeerStruct
	nodePort := make(map[int]int)
	for i := 0; i < peerCnt; i++ {
		port := 3000 + rand.Intn(50000)
		nodeID := i // + 1000
		nodePort[nodeID] = port
	}
	for i := 0; i < peerCnt; i++ {
		nodeID := i // + 1000
		p, err := NewPeer(peerCnt, nodeID, nodePort)
		if err != nil { // not important
			logger.Printf("peer(%v) create error: %v.\n", nodeID, err.Error())
			return
		}
		peers = append(peers, PeerStruct{peer: p})
	}
	rs := &runSystem{
		peers:    peers,
		peerCnt:  peerCnt,
		msgCnt:   msgCnt,
		timeout:  timeout,
		peerDone: make(chan bool, len(peers)),
	}
	for i := 0; i < peerCnt; i++ {
		if err := rs.peers[i].peer.BuildAllConnRPC(); err != nil {
			nodeID := i // + 1000
			logger.Printf("peer(%v) build rpc connection error: %v.\n", nodeID, err.Error())
		}
	}
	// var wg sync.WaitGroup
	// wg.Add(peerCnt + 1)
	// for i := 0; i < peerCnt; i++ {
	// 	go rs.runPeer(&wg, i, managerID)
	// }

	// timeoutChan := time.After(time.Duration(rs.timeout) * time.Millisecond)
	// go func() {
	// 	peerDoneCnt := 0
	// 	for {
	// 		select {
	// 		case <-rs.peerDone:
	// 			peerDoneCnt++
	// 			if peerDoneCnt == len(rs.peers) {
	// 				time.Sleep(500 * time.Millisecond)
	// 				// rs.Close()
	// 				return
	// 			}
	// 		case <-timeoutChan:
	// 			// rs.Close()
	// 			logger.Fatalf("Test time out after %v secs.\n", rs.timeout)
	// 			return
	// 		}
	// 	}
	// }()

	// wg.Wait()

	// print stats info
	// rs.printStats()
}

// func (rs *runSystem) Close() {
// 	for i := 0; i < rs.peerCnt; i++ {
// 		if rs.peers[i].peer != nil {
// 			rs.peers[i].peer.Close()
// 		}
// 	}
// 	if rs.srv.srv != nil {
// 		rs.srv.srv.Close()
// 	}
// 	logger.Printf("close all connections successfully.\n")
// }

// func (rs *runSystem) printStats() {
// 	totalReadCnt := 0
// 	totalWriteCnt := 0
// 	for i := 0; i < rs.peerCnt; i++ {
// 		logger.Printf("peerent(%v) readCount=%v, writeCount=%v.\n", i, rs.peers[i].peerReadCnt, rs.peers[i].peerWriteCnt)
// 		totalReadCnt += rs.peers[i].peerReadCnt
// 		totalWriteCnt += rs.peers[i].peerWriteCnt
// 	}
// 	logger.Printf("server(%v) readCount=%v, writeCount=%v.\n", rs.managerID, rs.srv.srvReadCnt, rs.srv.srvWriteCnt)
// 	totalReadCnt += rs.srv.srvReadCnt
// 	totalWriteCnt += rs.srv.srvWriteCnt
// 	logger.Printf("totalReadCount=%v, totalWriteCount=%v.\n", totalReadCnt, totalWriteCnt)
// }

func Test1(t *testing.T) {
	peerCnt := 10
	msgCnt := 500
	timeout := 1000
	testBasic1(t, peerCnt, msgCnt, timeout)
}
