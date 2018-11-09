// a centralized mutex server implementation
package message4

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

const (
	OK MessageType = iota + 1 // request mutual lock
)

type Message struct {
	MsgID      string
	MsgType    MessageType
	MsgContent interface{}
	Sender     int
	Receiver   int
}

// NewRequest returns a new distributed mutual lock message.
func NewOK(sender int, receiver int, msgContent interface{}) *Message {
	return &Message{
		MsgID:      RandStringBytes(msgIDCnt),
		MsgType:    OK,
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
	return fmt.Sprintf("[OK %s %d %d %v]", m.MsgID, m.Sender, m.Receiver, m.MsgContent)
}
