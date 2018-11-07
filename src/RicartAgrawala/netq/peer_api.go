package netq2

import (
	peerrpc "RicartAgrawala/rpc"
)

// Peer defines the set of methods that can be invoked remotely via RPCs.
type Peer interface {
	SendMessage(*peerrpc.SendMessageArgs, *peerrpc.SendMessageReply) error
	BuildAllConnRPC() error
}
