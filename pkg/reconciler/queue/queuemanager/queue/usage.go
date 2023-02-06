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
	"github.com/tektoncd/pipeline/pkg/apis/queue/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/queue/queuemanager/pkg/priorityqueue"
)

// Usage return queue status, which contains the usage of the current queue
func (q *defaultQueue) Usage() v1alpha1.QueueStatus {
	q.RLock()
	defer q.RUnlock()

	processingPipelineRuns := make([]string, 0)
	q.eq.ProcessingQueue().Range(func(item priorityqueue.Item) (stopRange bool) {
		pipelineRunKey := item.Key()
		existP := q.pipelineCaches[pipelineRunKey]
		if existP == nil {
			return false
		}
		processingPipelineRuns = append(processingPipelineRuns, pipelineRunKey)
		return false
	})

	return v1alpha1.QueueStatus{
		QueueStatusFields: v1alpha1.QueueStatusFields{
			PipelineConcurrency: q.eq.ProcessingWindow(),
			RunningPipelines:    processingPipelineRuns,
		},
	}
}

//func (q *defaultQueue) updateQueueStatus(status v1alpha1.QueueStatus) error {
//	q.pq.Status = status
//	if _, err := q.queueClientSet.TektonV1alpha1().Queues(q.pq.Namespace).Update(context.Background(), q.pq, metav1.UpdateOptions{}); err != nil {
//		return err
//	}
//	return nil
//}
