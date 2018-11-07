package netq2

import (
	peerrpc "RicartAgrawala/rpc"
	"errors"
	"fmt"
	"log"
	"net"
	http "net/http"
	rpc "net/rpc"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// Peer defines the set of methods that can be invoked remotely via RPCs.
type peer struct {
	listener net.Listener // listener for listening rpc requests.
	nodeCnt  int          // number of slaves
	nodeID   int
	nodes    []peerrpc.Node // all peers info, Node{nodeID, hostport}
	peers    map[int]*rpc.Client
	nodePort map[int]int // nodeID => port
	mu       sync.Mutex  // lock for normal case
	logger   *log.Logger
}

func CreateLog(fileName, header string) *log.Logger {
	newpath := filepath.Join(".", "log")
	os.MkdirAll(newpath, os.ModePerm)
	serverLogFile, _ := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	return log.New(serverLogFile, header, log.Lmicroseconds|log.Lshortfile)
}

// ports: nodeID => port
func NewPeer(numNodes, nodeID int, ports map[int]int) (Peer, error) {
	p := &peer{
		peers:    make(map[int]*rpc.Client),
		nodeCnt:  numNodes,
		nodeID:   nodeID,
		nodePort: ports,
	}
	p.logger = CreateLog("log/peer_"+strconv.Itoa(nodeID)+".log", "[peer]")
	if err := p.buildRPCListen(ports[nodeID]); err != nil {
		return nil, err
	}
	node := peerrpc.Node{
		NodeID:   nodeID,
		HostPort: fmt.Sprintf("localhost:%d", ports[nodeID]),
	}
	p.nodes = append(p.nodes, node)
	// if err := p.buildConnRPC(ports); err != nil {
	// 	return nil, err
	// }
	// go p.BuildConnRPC(ports)
	return p, nil
}

func (p *peer) buildRPCListen(port int) error {
	// master register itself to listen connections from other nodes.
	p.logger.Printf("node(%v) begin to register listen.\n", p.nodeID)
	var err error
	var l net.Listener
	l, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		p.logger.Printf("Node(%v) failed to listen: %v.\n", p.nodeID, err)
		return err
	}
	err = rpc.RegisterName("Peer", peerrpc.Wrap(p))
	if err != nil {
		return err
	}
	p.listener = l
	rpc.HandleHTTP()
	go http.Serve(p.listener, nil)
	p.logger.Printf("node(%v) end register listen.\n", p.nodeID)
	return nil
}

func (p *peer) BuildAllConnRPC() error {
	time.Sleep(time.Duration(1000) * time.Millisecond)
	p.logger.Printf("node begin to DialHTTP to all other peers.\n")
	for nodeID, port := range p.nodePort {
		if nodeID == p.nodeID {
			continue
		}
		node := peerrpc.Node{
			NodeID:   nodeID,
			HostPort: fmt.Sprintf(":%d", port),
		}
		p.nodes = append(p.nodes, node)
		p.logger.Printf("node(%v) begin to DialHTTP to (%v, %v).\n", p.nodeID, node.NodeID, node.HostPort)
		cli, err := rpc.DialHTTP("tcp", node.HostPort)
		if err != nil {
			p.logger.Printf("node(%v) error DialHTTP to (%v, %v): %v.\n", p.nodeID, node.NodeID, node.HostPort, err.Error())
			return err
		}
		p.peers[node.NodeID] = cli
		p.logger.Printf("node(%v) successfully DialHTTP to (%v, %v).\n", p.nodeID, node.NodeID, node.HostPort)
	}
	p.logger.Printf("node end DialHTTP to all other peers.\n")
	return nil
}

func (p *peer) SendMessage(args *peerrpc.SendMessageArgs, reply *peerrpc.SendMessageReply) error {

	return errors.New("Not implemented.")
}
