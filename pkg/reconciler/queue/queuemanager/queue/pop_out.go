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

func (q *defaultQueue) doPop(item priorityqueue.Item) (stopRange bool) {
	// pop now
	poppedKey := q.eq.PopProcessingKey(item.Key())
	// queue cannot pop item anymore
	if poppedKey == "" {
		stopRange = true
		return
	}
	return
}

// PopOutPipeline pop out specify pipeline run
func (q *defaultQueue) PopOutPipeline(pr *v1beta1.PipelineRun) {
	q.Lock()
	defer q.Unlock()

	q.eq.PopProcessing(makeItemKey(pr))
	// delete from caches
	delete(q.pipelineCaches, makeItemKey(pr))
}
