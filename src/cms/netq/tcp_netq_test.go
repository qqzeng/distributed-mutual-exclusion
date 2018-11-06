package netq

import (
	msgp "cms/msg"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"testing"
	"time"
)

var (
	logger *log.Logger
)

func init() {
	serverLogFile, _ := os.OpenFile("log_tcp_netq.log", os.O_RDWR|os.O_CREATE, 0666)
	logger = log.New(serverLogFile, "[tcp_netq_test] ", log.Lmicroseconds|log.Lshortfile)
}

type msgData struct {
	content string
}

func TestBasic1(t *testing.T) {
	s, err := NewServer(6066)
	if err != nil {
		logger.Printf("server create error: %v.\n", err.Error())
		return
	}
	c, err := NewClient(6066)
	if err != nil {
		logger.Printf("client create error: %v.\n", err.Error())
		return
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		// write
		for i := 0; i < 10; i++ {
			s := msgp.NewRequest(1, 2, fmt.Sprintf("This is content of %d", i))
			mdBytes, _ := json.Marshal(s)
			if err := c.WriteData(mdBytes); err != nil {
				logger.Printf("client write error: %v.\n", err.Error())
				return
			}
			// read
			readBytes, err := c.ReadData()
			if err != nil {
				logger.Printf("client read error: %v.\n", err.Error())
				continue
			}
			var r msgp.Message
			json.Unmarshal(readBytes, &r)
			// logger.Printf("Client read data from server %v.\n", readBytes)
			logger.Printf("client read message from server: %v.\n", r.String())
		}
		c.Close()
		return
	}()
	timeout := 1000
	timeoutChan := time.After(time.Duration(timeout) * time.Millisecond)
	go func() {
		select {
		case <-timeoutChan:
			s.Close()
			return
		}
	}()
	go func() {
		defer wg.Done()
		// read
		for {
			connID, readBytes, err := s.ReadData()
			if err != nil {
				logger.Printf("server read error: %v.\n", err.Error())
				return
			}
			var r msgp.Message
			json.Unmarshal(readBytes, &r)
			logger.Printf("server read message from connection %v: %v.\n", connID, r.String())
			// write
			g := msgp.NewGrant(r.Receiver, r.Sender, r.MsgContent.(string)+"_grant")
			mdBytes, _ := json.Marshal(g)
			if err := s.WriteData(connID, mdBytes); err != nil {
				logger.Printf("client write error: %v.\n", err.Error())
				return
			}
			logger.Printf("server write message to connection %v: %v.\n", connID, g.String())
		}
	}()

	wg.Wait()
}

func TestJsonMarshal(t *testing.T) {
	s := msgp.NewRequest(1, 2, fmt.Sprintf("This is content of %d", 10))
	bb, err := json.Marshal(s)
	if err != nil {
		logger.Println("error:", err)
	}
	os.Stdout.Write(bb)

	var r msgp.Message
	json.Unmarshal(bb, &r)
	logger.Println(r)
}
