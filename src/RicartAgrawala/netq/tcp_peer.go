package netq2

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
)

const (
	MSG_BUFFERED_SIZE  = 100
	CHAN_SIZE          = 1
	CONN_BUFFERED_SIZE = 1
)

type tcppeer struct {
	nodeNum       int           // number of clienttp.
	chanConnList  chan net.Conn // channel to notice a coming connection.
	chanStop      chan bool     // channel to indicate stop server.
	chanRespMap   map[int]*compConn
	chanReqMap    map[int]*compConn
	chanConnClose chan int
	ln            net.Listener
	readChannel   chan *ReadDataComp
	hostport      string
	chanOnClose   chan bool
	readError     chan int
	nodePort      map[int]int // nodeID => port
	nodes         []Node      // all peers info, Node{nodeID, hostport}
	nodeID        int
	nodeConn      map[int]int // nodeID => connID
	mu            sync.Mutex  // lock for normal case
	logger        *log.Logger
}

type compConn struct {
	connID         int
	conn           net.Conn
	chanDataBuffer chan []byte
}
type Node struct {
	hostport string // The host:port address of the storage server node.
	nodeID   int    // The ID identifying this storage server node.
}
type ReadDataComp struct {
	data   []byte
	connID int
}

func CreateLog(fileName, header string) *log.Logger {
	newpath := filepath.Join(".", "log")
	os.MkdirAll(newpath, os.ModePerm)
	serverLogFile, _ := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	return log.New(serverLogFile, header, log.Lmicroseconds|log.Lshortfile)
}

// New creates and returns (but does not start) a new server.
func NewTcpPeer(numNodes, nodeID int, nodePort map[int]int) (TcpPeer, error) {
	tp := &tcppeer{
		nodeNum:       numNodes,
		nodeID:        nodeID,
		nodePort:      nodePort,
		chanStop:      make(chan bool, CHAN_SIZE),
		readError:     make(chan int, CHAN_SIZE),
		chanConnList:  make(chan net.Conn, CONN_BUFFERED_SIZE),
		chanRespMap:   make(map[int]*compConn),
		chanReqMap:    make(map[int]*compConn),
		nodeConn:      make(map[int]int),
		chanConnClose: make(chan int, CONN_BUFFERED_SIZE),
		readChannel:   make(chan *ReadDataComp, MSG_BUFFERED_SIZE),
		hostport:      fmt.Sprintf(":%d", nodePort[nodeID]),
	}
	tp.logger = CreateLog("log/peer_"+strconv.Itoa(nodeID)+".log", "[peer]")
	for nodeID, port := range tp.nodePort {
		if nodeID == tp.nodeID {
			continue
		}
		tp.nodes = append(tp.nodes, Node{nodeID: nodeID, hostport: fmt.Sprintf(":%d", port)})
	}
	if err := tp.start(); err != nil {
		return nil, err
	}
	return tp, nil
}

func (tp *tcppeer) start() error {
	ln, err := net.Listen("tcp", tp.hostport)
	if err != nil {
		tp.logger.Printf("%v\n", err)
		return err
	}
	tp.ln = ln
	go tp.buildConnListen()
	go tp.handleStuff()
	return nil
}

func (tp *tcppeer) buildConnListen() error {
	for {
		// tp.logger.Println("Server waiting for a connection.")
		tp.logger.Printf("node(%v) begin to listen.\n", tp.nodeID)
		if tp.ln == nil {
			return errors.New(fmt.Sprintf("node(%v) listener has closed.\n", tp.nodeID))
		}
		conn, err := tp.ln.Accept()
		if err != nil {
			tp.logger.Printf("node(%v) error on accept: %v.\n", tp.nodeID, err.Error())
			return err
		}
		tp.logger.Printf("node(%v) listen a new connection.\n", tp.nodeID)
		tp.chanConnList <- conn
	}
}

// build out connections.
func (tp *tcppeer) BuildAllConn() (map[int]int, error) {
	tp.logger.Printf("node begin to DialTCP to all other peers.\n")
	for _, node := range tp.nodes {
		_, hostport := node.nodeID, node.hostport
		tp.logger.Printf("node(%v) begin to DialTCP to (%v, %v).\n", tp.nodeID, node.nodeID, node.hostport)
		addr, err := net.ResolveTCPAddr("tcp", hostport)
		if err != nil {
			return nil, err
		}
		conn, err := net.DialTCP("tcp", nil, addr)
		if err != nil {
			tp.logger.Printf("node(%v) error DialTCP to (%v, %v): %v.\n", tp.nodeID, node.nodeID, node.hostport, err.Error())
			return nil, err
		}
		connID := tp.nextServerConnID()
		cc := &compConn{
			connID:         connID,
			conn:           conn,
			chanDataBuffer: make(chan []byte, MSG_BUFFERED_SIZE),
		}
		tp.chanReqMap[connID] = cc
		tp.logger.Printf("node(%v) successfully DialTCP to (%v, %v).\n", tp.nodeID, node.nodeID, node.hostport)
		tp.nodeConn[node.nodeID] = connID
	}
	return tp.nodeConn, nil
}

func (tp *tcppeer) ReadData() (int, []byte, error) {
	for {
		select {
		case rdc := <-tp.readChannel:
			return rdc.connID, rdc.data, nil
		case connID := <-tp.readError:
			// tp.logger.Printf("Server error read.\n")
			return connID, nil, errors.New(fmt.Sprintf("server read from connection(%v) error.\n", connID))
		}
	}
}

func (tp *tcppeer) WriteData(connID int, data []byte) error {
	var cc *compConn
	var exist bool
	cc, exist = tp.chanRespMap[connID] // in connection
	if !exist {
		cc = tp.chanReqMap[connID] // out connection
	}
	_, err := cc.conn.Write(Packet(data))
	// tp.logger.Printf("server write write data to  connection %v: %v\n", connID, data)
	if err != nil {
		// TODO: handle error.
		return err
	}
	return nil
}

func (tp *tcppeer) Close() error {
	// tp.logger.Printf("server Close called..\n")
	tp.chanStop <- true
	return nil
}

func (tp *tcppeer) CloseConn(connID int) error {
	tp.chanConnClose <- connID
	return nil
}

func (tp *tcppeer) Count() int {
	return tp.nodeNum
}

// listen for in connection. read
func (tp *tcppeer) handleConn(cc *compConn) {
	// tp.logger.Println("Server Reading from connection..")
	tmpBuffer := make([]byte, 0)

	buffer := make([]byte, 1024)
	for {
		n, err := cc.conn.Read(buffer)
		if err != nil {
			// tp.logger.Printf("Server error read: %v.\n", err.Error())
			tp.readError <- cc.connID
			tp.chanConnClose <- cc.connID
			return
		}
		// TODO: read error handle
		tmpBuffer = Unpack(append(tmpBuffer, buffer[:n]...), cc.connID, tp.readChannel)
	}
}

func (tp *tcppeer) handleStuff() {
	for {
		select {
		case <-tp.chanStop:
			for _, cc := range tp.chanRespMap {
				tp.chanConnClose <- cc.connID
			}
			tp.ln.Close()
			for _, cc := range tp.chanReqMap {
				tp.chanConnClose <- cc.connID
			}
			tp.ln.Close()
			return
		case con := <-tp.chanConnList:
			if con != nil {
				connID := tp.nextServerConnID()
				cc := &compConn{
					connID:         connID,
					conn:           con,
					chanDataBuffer: make(chan []byte, MSG_BUFFERED_SIZE),
				}
				tp.chanRespMap[connID] = cc
				go tp.handleConn(cc)
			}
		case connID := <-tp.chanConnClose:
			var cc *compConn
			var exist bool
			cc, exist = tp.chanRespMap[connID]
			if exist && cc.conn != nil {
				tp.nodeNum--
				delete(tp.chanRespMap, connID)
				cc.conn.Close()
			}
			if !exist {
				cc, exist = tp.chanReqMap[connID]
				if exist && cc.conn != nil {
					tp.nodeNum--
					delete(tp.chanReqMap, connID)
					cc.conn.Close()
				}
			}
		}
	}
}

var nextServerConnID int32 = 0

func (tp *tcppeer) nextServerConnID() int {
	return int(atomic.AddInt32(&nextServerConnID, 1))
}
