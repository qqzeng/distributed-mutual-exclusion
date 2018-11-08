// a centralized mutex server implementation
package message2

import (
	"fmt"
	"math/rand"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const msgIDCnt = 10

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

type MessageType int
type TimeStamp int

const (
	Request MessageType = iota + 1 // request mutual lock
	Reply                          // reply mutual lock
)

// Field must start with UPPERCASE letter
type Message struct {
	MsgID      string
	MsgType    MessageType
	TS         TimeStamp
	MsgContent interface{}
	Sender     int
	Receiver   int
}

// NewRequest returns a new distributed mutual lock message.
func NewRequest(ts TimeStamp, sender int, receiver int, msgContent interface{}) *Message {
	return &Message{
		MsgID:      RandStringBytes(msgIDCnt),
		MsgType:    Request,
		TS:         ts,
		Sender:     sender,
		Receiver:   receiver,
		MsgContent: msgContent,
	}
}

// ts is the corresponding ts of the its request message .
func NewReply(ts TimeStamp, sender int, receiver int, msgContent interface{}) *Message {
	return &Message{
		MsgID:      RandStringBytes(msgIDCnt),
		MsgType:    Reply,
		TS:         ts,
		Sender:     sender,
		Receiver:   receiver,
		MsgContent: msgContent,
	}
}

// String returns a string representation of this message. To pretty-print a
// message, you can pass it to a format string like so:
//     msg := NewRequest()
//     fmt.Printf("Request message: %s\n", msg)
func (m *Message) String() string {
	var name string
	switch m.MsgType {
	case Request:
		name = "Request"
	case Reply:
		name = "Reply"
	}
	return fmt.Sprintf("[%s %s %d %d %d %v]", name, m.MsgID, m.TS, m.Sender, m.Receiver, m.MsgContent)
}
