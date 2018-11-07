// This file provides a type-safe wrapper that should be used to register the
// storage server to receive RPCs from a TribServer's libstore. DO NOT MODIFY!
package peerrpc

type RemotePeer interface {
	SendMessage(*SendMessageArgs, *SendMessageReply) error
}

type Peer struct {
	// Embed all methods into the struct. See the Effective Go section about
	// embedding for more details: golang.org/doc/effective_go.html#embedding
	RemotePeer
}

// Wrap wraps s in a type-safe wrapper struct to ensure that only the desired
// Peer methods are exported to receive RPCs.
func Wrap(p RemotePeer) RemotePeer {
	return &Peer{p}
}
