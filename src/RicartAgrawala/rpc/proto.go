// This file contains constants and arguments used to perform RPCs between peers.

package peerrpc

import (
	msgp2 "RicartAgrawala/msg"
)

// Status represents the status of a RPC's reply.
type Status int

const (
	OK   Status = iota + 1 // The RPC was a success.
	FAIL                   // The specified key does not exist.
)

type Node struct {
	HostPort string // The host:port address of the storage server node.
	NodeID   int    // The ID identifying this storage server node.
}

type SendMessageArgs struct {
	MessageInfo msgp2.Message
}

type SendMessageReply struct {
	MessageInfo msgp2.Message
	Status      Status
}
