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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/queue/queuemanager/pkg/priorityqueue"
)

// makeItemKey
func makeItemKey(pr *v1beta1.PipelineRun) string {
	return pr.Name
}

// AddPipelineIntoQueue add pipeline run into queue, after the addition is complete,
// range the processing queue immediately
func (q *defaultQueue) AddPipelineIntoQueue(pr *v1beta1.PipelineRun, rangeQueue bool) {
	q.Lock()
	defer q.Unlock()
	q.addPipelineIntoQueueUnblock(pr, rangeQueue)
}

func (q *defaultQueue) addPipelineIntoQueueUnblock(pr *v1beta1.PipelineRun, rangeQueue bool) {
	if !pr.IsProcessing() {
		return
	}
	itemKey := makeItemKey(pr)

	createdTime := pr.CreationTimestamp.Time

	q.pipelineCaches[itemKey] = pr
	q.eq.ProcessingQueue().Add(priorityqueue.NewItem(itemKey, createdTime))
	if rangeQueue {
		go func() {
			q.rangeAtOnceCh <- true
		}()
	}
}
