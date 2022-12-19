package ttlcache

import (
	"container/heap"
)

func newPriorityQueue() *priorityQueue {
	queue := &priorityQueue{}
	heap.Init(queue)
	return queue
}

type priorityQueue struct {
	items []*item
}

func (pq *priorityQueue) isEmpty() bool {
	return len(pq.items) == 0
}

func (pq *priorityQueue) root() *item {
	if len(pq.items) == 0 {
		return nil
	}

	return pq.items[0]
}

func (pq *priorityQueue) update(item *item) {
	heap.Fix(pq, item.queueIndex)
}

func (pq *priorityQueue) push(item *item) {
	heap.Push(pq, item)
}

func (pq *priorityQueue) pop() *item {
	if pq.Len() == 0 {
		return nil
	}
	return heap.Pop(pq).(*item)
}

func (pq *priorityQueue) remove(item *item) {
	heap.Remove(pq, item.queueIndex)
}

func (pq priorityQueue) Len() int {
	length := len(pq.items)
	return length
}

// Less will consider items with time.Time default value (epoch start) as more than set items.
func (pq priorityQueue) Less(i, j int) bool {
	if pq.items[i].expireAt.IsZero() {
		return false
	}
	if pq.items[j].expireAt.IsZero() {
		return true
	}
	return pq.items[i].expireAt.Before(pq.items[j].expireAt)
}

func (pq priorityQueue) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
	pq.items[i].queueIndex = i
	pq.items[j].queueIndex = j
}

func (pq *priorityQueue) Push(x interface{}) {
	item := x.(*item)
	item.queueIndex = len(pq.items)
	pq.items = append(pq.items, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := pq.items
	n := len(old)
	item := old[n-1]
	item.queueIndex = -1
	// de-reference the element to be popped for Garbage Collector to de-allocate the memory
	old[n-1] = nil
	pq.items = old[0 : n-1]
	return item
}
