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
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/queue/v1alpha1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
)

// Start running the queue program, listen to the channel in the goroutine and check the queue elements
func (q *defaultQueue) Start(stopChan chan struct{}) error {
	if q.started {
		return nil
	}
	labels, err := getLabelSelector(q.pq)
	if err != nil {
		return err
	}
	pipelineRuns, err := q.pipelineRunLister.PipelineRuns(q.pq.Namespace).List(labels)
	if err != nil {
		return err
	}
	for i := range pipelineRuns {
		q.AddPipelineIntoQueue(pipelineRuns[i], false)
	}
	ticket := time.NewTicker(time.Second * time.Duration(10))
	go func() {
		for {
			select {
			case <-stopChan:
				return
			case <-ticket.C:
				q.RangeProcessingQueue()
			case <-q.rangeAtOnceCh:
				q.RangeProcessingQueue()
			}
		}
	}()
	q.started = true
	q.rangeAtOnceCh <- true
	return nil
}

func getLabelSelector(queue *v1alpha1.Queue) (k8slabels.Selector, error) {
	labelSelector := map[string]string{
		v1beta1.BinQueueLabel: queue.Name,
	}
	out := k8slabels.SelectorFromSet(labelSelector)
	return out, nil
}
