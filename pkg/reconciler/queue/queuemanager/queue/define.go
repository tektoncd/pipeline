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

package queue

import (
	"fmt"
	"strings"
	"sync"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/queue/v1alpha1"
	pipelineclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1beta1"
	queueclient "github.com/tektoncd/pipeline/pkg/client/queue/clientset/versioned"
	"github.com/tektoncd/pipeline/pkg/reconciler/queue/queuemanager/pkg/enhancedqueue"
	"github.com/tektoncd/pipeline/pkg/reconciler/queue/queuemanager/pkg/priorityqueue"
)

type defaultQueue struct {
	sync.RWMutex

	pq                      *v1alpha1.Queue
	eq                      *enhancedqueue.EnhancedQueue
	doneChanByKey           map[string]chan struct{}
	pipelineCaches          map[string]*v1beta1.PipelineRun
	rangeAtOnceCh           chan bool
	rangingProcessingQueue  bool
	started                 bool
	currentItemKeyAtRanging string
	pipelineRunLister       listers.PipelineRunLister
	pipelineClientSet       pipelineclient.Interface
	queueClientSet          queueclient.Interface
}

// New generate a new default queue
func New(pq *v1alpha1.Queue,
	pipelineRunLister listers.PipelineRunLister,
	pipelineClientSet pipelineclient.Interface,
	queueClientSet queueclient.Interface) *defaultQueue {
	return &defaultQueue{
		pq:                pq,
		eq:                enhancedqueue.NewEnhancedQueue(pq.Spec.PipelineConcurrency),
		doneChanByKey:     make(map[string]chan struct{}),
		pipelineCaches:    make(map[string]*v1beta1.PipelineRun),
		rangeAtOnceCh:     make(chan bool),
		pipelineRunLister: pipelineRunLister,
		pipelineClientSet: pipelineClientSet,
		queueClientSet:    queueClientSet,
	}
}

func (q *defaultQueue) ID() string {
	return fmt.Sprintf("%s/%s", q.pq.Namespace, q.pq.Name)
}

// ValidateCapacity Check the concurrency of queue
func (q *defaultQueue) ValidateCapacity() (bool, string) {
	if int64(q.eq.ProcessingQueue().Len()) > q.pq.Spec.PipelineConcurrency {
		var processItems []string
		q.eq.ProcessingQueue().Range(func(item priorityqueue.Item) (stopRange bool) {
			processItems = append(processItems, item.Key())
			return false
		})
		return false, fmt.Sprintf("Insufficient processing concurrency(%d), current processing count: %d, processing items: [%s]",
			q.eq.ProcessingWindow(),
			q.eq.ProcessingQueue().Len(),
			strings.Join(processItems, ", "))
	}
	return true, ""
}

// Update the processing window by new concurrency
func (q *defaultQueue) Update(pq *v1alpha1.Queue) {
	q.pq = pq
	q.eq.SetProcessingWindow(pq.Spec.PipelineConcurrency)
}

func (q *defaultQueue) getIsRangingProcessingQueue() bool {
	q.RLock()
	defer q.RUnlock()
	return q.rangingProcessingQueue
}

func (q *defaultQueue) setIsRangingProcessingQueueFlag() {
	q.Lock()
	defer q.Unlock()
	q.rangingProcessingQueue = true
}

func (q *defaultQueue) unsetIsRangingProcessingQueueFlag() {
	q.Lock()
	defer q.Unlock()
	q.rangingProcessingQueue = false
}

func (q *defaultQueue) setCurrentItemKeyAtRanging(itemKey string) {
	q.Lock()
	defer q.Unlock()
	q.currentItemKeyAtRanging = itemKey
}
