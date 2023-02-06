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

package enhancedqueue

import (
	"sync"

	"github.com/tektoncd/pipeline/pkg/reconciler/queue/queuemanager/pkg/priorityqueue"
)

type EnhancedQueue struct {
	sync.RWMutex

	processing       *priorityqueue.PriorityQueue
	processingWindow int64
}

func NewEnhancedQueue(window int64) *EnhancedQueue {
	return &EnhancedQueue{
		processing:       priorityqueue.NewPriorityQueue(),
		processingWindow: window,
	}
}

func (eq *EnhancedQueue) ProcessingQueue() *priorityqueue.PriorityQueue {
	eq.Lock()
	defer eq.Unlock()

	return eq.processing
}

func (eq *EnhancedQueue) inProcessing(key string) bool {
	return eq.processing.Get(key) != nil
}

func (eq *EnhancedQueue) InProcessing(key string) bool {
	eq.RLock()
	defer eq.RUnlock()

	return eq.inProcessing(key)
}

func (eq *EnhancedQueue) InQueue(key string) bool {
	eq.RLock()
	defer eq.RUnlock()

	return eq.inProcessing(key)
}

func (eq *EnhancedQueue) PopProcessing(key string, dryRun ...bool) string {
	eq.Lock()
	defer eq.Unlock()

	if !eq.inProcessing(key) {
		return ""
	}

	if len(dryRun) > 0 && dryRun[0] {
		return key
	}

	eq.processing.Remove(key)
	return key
}

func (eq *EnhancedQueue) ProcessingWindow() int64 {
	eq.RLock()
	defer eq.RUnlock()

	return eq.processingWindow
}

func (eq *EnhancedQueue) SetProcessingWindow(newWindow int64) {
	eq.Lock()
	defer eq.Unlock()

	eq.processingWindow = newWindow
}

// PopProcessingKey pop processing queue specified key.
func (eq *EnhancedQueue) PopProcessingKey(key string) string {
	eq.Lock()
	defer eq.Unlock()

	return eq.popProcessingKeyWithoutLock(key)
}

// popProcessingKeyWithoutLock pop specified key from processing queue, lock outside.
func (eq *EnhancedQueue) popProcessingKeyWithoutLock(popKey string) string {
	poppedItem := eq.processing.Remove(popKey)
	if poppedItem == nil {
		return ""
	}
	return poppedItem.Key()
}
