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
	"context"

	"github.com/sirupsen/logrus"
	"github.com/tektoncd/pipeline/pkg/apis/queue/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/queue/queuemanager/pkg/priorityqueue"
)

// RangeProcessingQueue check all items in the queue. If the queue is full,
// pop the element out of the queue and cancel the running pipeline
func (q *defaultQueue) RangeProcessingQueue() {
	if q.getIsRangingProcessingQueue() {
		return
	}
	q.setIsRangingProcessingQueueFlag()
	defer q.unsetIsRangingProcessingQueueFlag()
	//defer func() {
	//	usage := q.Usage()
	//	if err := q.updateQueueStatus(usage); err != nil {
	//		logrus.Errorf("Failed to update Queue usage, usage: %v, err: %v", usage, err)
	//	}
	//}()

	q.eq.ProcessingQueue().Range(func(item priorityqueue.Item) (stopRange bool) {
		q.setCurrentItemKeyAtRanging(item.Key())
		validated, reason := q.ValidateCapacity()
		if validated {
			return false
		}
		logrus.Warnf("The queue is full, start cancel %s, Reason: %s", item.Key(), reason)
		pr := q.pipelineCaches[item.Key()]
		if pr == nil {
			return q.doPop(item)
		}
		if err := q.cancelPipelineRun(context.Background(), pr.Namespace, pr.Name, v1alpha1.Strategy(q.pq.Spec.Strategy)); err != nil {
			logrus.Errorf("Queue: %s failed to cancel pipelinerun: %s, err: %v", q.ID(), pr.Name, err)
			return true
		}
		q.PopOutPipeline(pr)
		return true
	})
}
