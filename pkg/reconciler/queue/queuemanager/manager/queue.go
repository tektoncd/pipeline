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

package manager

import (
	"context"
	"fmt"
	"sync"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/queue/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/queue/queuemanager/queue"
	"github.com/tektoncd/pipeline/pkg/reconciler/queue/queuemanager/types"
)

var (
	once = sync.Once{}
)

var (
	queueManager *defaultManager
)

func makeQueueID(queueName, queueNamespace string) string {
	return fmt.Sprintf("%s/%s", queueNamespace, queueName)
}

// IdempotentAddQueue Add the queue to the queue manager and start the queue daemon,
// which is an idempotent operation
func IdempotentAddQueue(pq *v1alpha1.Queue) types.Queue {
	queueManager.Lock()
	defer queueManager.Unlock()

	newQueue := queue.New(pq, queueManager.pipelineRunLister, queueManager.pipelineClientSet, queueManager.queueClientSet)

	_, ok := queueManager.queueByID[newQueue.ID()]
	if ok {
		queueManager.queueByID[newQueue.ID()].Update(pq)
		return queueManager.queueByID[newQueue.ID()]
	}

	queueManager.queueByID[newQueue.ID()] = newQueue
	qStopCh := make(chan struct{})
	queueManager.queueStopChanByID[newQueue.ID()] = qStopCh
	go func() {
		newQueue.Start(qStopCh)
	}()

	return newQueue
}

// PutPipelineIntoQueue check whether the pipeline run is bound to the queue,
// if so, add it to the corresponding queue
func PutPipelineIntoQueue(ctx context.Context, pr *v1beta1.PipelineRun) {
	if !pr.IsProcessing() {
		return
	}
	bindQueue := queueManager.ensureQueueDetail(pr)
	if bindQueue == nil {
		return
	}
	bindQueue.AddPipelineIntoQueue(pr, true)
}

// PopOutPipelineFromQueue check whether the pipeline run is bound to the queue,
// if so, pop it from queue
func PopOutPipelineFromQueue(pr *v1beta1.PipelineRun) {
	bindQueue := queueManager.ensureQueueDetail(pr)
	if bindQueue == nil {
		return
	}
	bindQueue.PopOutPipeline(pr)
}

func (mgr *defaultManager) ensureQueueDetail(pr *v1beta1.PipelineRun) types.Queue {
	bindQueueName := pr.GetBindQueueName()
	if bindQueueName == "" {
		return nil
	}
	bindQueueID := makeQueueID(bindQueueName, pr.Namespace)
	if bindQueue, ok := queueManager.queueByID[bindQueueID]; ok {
		return bindQueue
	}

	bindQueue, err := queueManager.queueLister.Queues(pr.Namespace).Get(bindQueueName)
	if err != nil {
		return nil
	}
	pq := IdempotentAddQueue(bindQueue)
	return pq
}
