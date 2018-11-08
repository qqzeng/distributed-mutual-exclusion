package LamportDistributedMutualExclusion

// a Message heap built using the heap interface.
// package main

import (
	msgp3 "LamportDistributedMutualExclusion/msg"
	// "container/heap"
	// "fmt"
)

// An MessageHeap is a min-heap of ints.
type MessageHeap []msgp3.Message

func (h MessageHeap) Len() int           { return len(h) }
func (h MessageHeap) Less(i, j int) bool { return h[i].TS < h[j].TS }
func (h MessageHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *MessageHeap) Push(x interface{}) {
	// Push and Pop use pointer receivers because they modify the slice's length,
	// not just its contents.
	*h = append(*h, x.(msgp3.Message))
}

func (h *MessageHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// test inserts several ints into an MessageHeap, checks the minimum,
// and removes them in order of priority.
// func main() {
// 	h := &MessageHeap{}
// 	heap.Init(h)
// 	heap.Push(h, *msgp3.NewRequest(msgp3.TimeStamp(1), 1, 2, ""))
// 	// heap.Push(h, msgp3.NewRequest(msgp3.TimeStamp(5), 2, 1, ""))
// 	// heap.Push(h, msgp3.NewRequest(msgp3.TimeStamp(2), 1, 3, ""))
// 	// heap.Push(h, msgp3.NewRequest(msgp3.TimeStamp(10), 3, 2, ""))
// 	fmt.Printf("minimum: %v\n", (*h)[0].String())
// 	for h.Len() > 0 {
// 		// fmt.Printf("the front element is %v.\n", h.Front().(*msgp3.Message).String())
// 		fmt.Printf("%v\n", heap.Pop(h))
// 	}
// }
