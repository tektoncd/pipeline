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

// implement pkg container heap.Interface
func (pq priorityQueue) Len() int { return len(pq.items) }

func (pq priorityQueue) Less(i, j int) bool {
	return pq.items[i].CreationTime().Before(pq.items[j].CreationTime())
}

func (pq priorityQueue) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
	pq.items[i].SetIndex(i)
	pq.items[j].SetIndex(j)
}

func (pq *priorityQueue) Push(x interface{}) {
	n := len(pq.items)
	item := x.(Item)
	item.SetIndex(n)
	pq.items = append(pq.items, item)
	pq.itemByKey[item.Key()] = item
}

func (pq *priorityQueue) Pop() interface{} {
	old := pq.items
	n := len(old)
	item := old[n-1]
	item.SetIndex(-1)
	pq.items = old[0 : n-1]
	delete(pq.itemByKey, item.Key())
	return item
}
