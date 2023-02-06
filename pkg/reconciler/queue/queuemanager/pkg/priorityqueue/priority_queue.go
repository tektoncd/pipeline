/*
 *
 * Copyright 2019 The Tekton Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

package priorityqueue

import (
	"container/heap"
	"sort"
)

// PriorityQueue priority queue
type PriorityQueue struct {
	data *priorityQueue
}

type priorityQueue struct {
	items     []Item
	itemByKey map[string]Item
}

func NewPriorityQueue() *PriorityQueue {
	return &PriorityQueue{
		data: &priorityQueue{
			items:     make([]Item, 0),
			itemByKey: make(map[string]Item),
		},
	}
}

func (pq *PriorityQueue) Get(key string) Item {
	return pq.data.itemByKey[key]
}

func (pq *PriorityQueue) Peek() Item {
	if len(pq.data.items) == 0 {
		return nil
	}
	return pq.data.items[0]
}

func (pq *PriorityQueue) Pop() Item {
	if len(pq.data.items) == 0 {
		return nil
	}
	return heap.Pop(pq.data).(Item)
}

func (pq *PriorityQueue) Add(item Item) {
	if _, ok := pq.data.itemByKey[item.Key()]; !ok {
		heap.Push(pq.data, convertItem(item))
	}
	sort.Sort(pq.data)
}

func (pq *PriorityQueue) Remove(key string) Item {
	if existItem, ok := pq.data.itemByKey[key]; ok {
		delete(pq.data.itemByKey, key)
		return heap.Remove(pq.data, existItem.Index()).(Item)
	}
	return nil
}

// Len return queue length
func (pq *PriorityQueue) Len() int {
	return pq.data.Len()
}

// Range range items and apply func to item one by one.
func (pq *PriorityQueue) Range(f func(Item) (stopRange bool)) {
	for _, item := range pq.data.items {
		stopRange := f(item)
		if stopRange {
			break
		}
	}
}

// LeftHasHigherOrder judge order of two items.
// return true if left has higher order.
func (pq *PriorityQueue) LeftHasHigherOrder(left, right string) bool {
	leftItem, leftExist := pq.data.itemByKey[left]
	rightItem, rightExist := pq.data.itemByKey[right]
	if !leftExist && !rightExist {
		return false
	}
	if !leftExist {
		return false
	}
	if !rightExist {
		return true
	}
	// both exist, judge by index
	return leftItem.Index() < rightItem.Index()
}
