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
	"sync"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	pipelineclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1beta1"
	queueclient "github.com/tektoncd/pipeline/pkg/client/queue/clientset/versioned"
	listersv1alpha1 "github.com/tektoncd/pipeline/pkg/client/queue/listers/queue/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/queue/queuemanager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
)

type defaultManager struct {
	sync.RWMutex

	queueByID         map[string]types.Queue
	queueStopChanByID map[string]chan struct{}
	pipelineRunLister listers.PipelineRunLister
	queueLister       listersv1alpha1.QueueLister
	queueClientSet    queueclient.Interface
	pipelineClientSet pipelineclient.Interface
}

type Option func(mgr *defaultManager)

func WithPipelineRunLister(pipelineRunLister listers.PipelineRunLister) Option {
	return func(mgr *defaultManager) {
		mgr.pipelineRunLister = pipelineRunLister
	}
}

func WithQueueLister(queueLister listersv1alpha1.QueueLister) Option {
	return func(mgr *defaultManager) {
		mgr.queueLister = queueLister
	}
}

func WithQueueClientSet(queueClientSet queueclient.Interface) Option {
	return func(mgr *defaultManager) {
		mgr.queueClientSet = queueClientSet
	}
}

func WithPipelineClientSet(pipelineClientSet pipelineclient.Interface) Option {
	return func(mgr *defaultManager) {
		mgr.pipelineClientSet = pipelineClientSet
	}
}

// Init initialize the queue manager with options, start all queues
func Init(ctx context.Context, ops ...Option) {
	logger := logging.FromContext(ctx)
	var mgr defaultManager

	mgr.queueByID = make(map[string]types.Queue)
	mgr.queueStopChanByID = make(map[string]chan struct{})

	for _, op := range ops {
		op(&mgr)
	}

	go func() {
		select {
		case <-ctx.Done():
			begin := time.Now()
			logger.Infof("queueManager: begin stop")
			mgr.Stop()
			end := time.Now()
			logger.Infof("queueManager: end stop, cost: %s", end.Sub(begin).String())
		}
	}()
	once.Do(func() {
		queueManager = &mgr
	})

	cfg := config.FromContextOrDefaults(ctx)
	if cfg.FeatureFlags.EnableAPIFields == config.AlphaAPIFields {
		queues, err := queueManager.queueClientSet.TektonV1alpha1().Queues("").List(ctx, metav1.ListOptions{})
		if err != nil {
			logger.Panicf("Error list queues for manager: %v", err)
		}
		for i := range queues.Items {
			IdempotentAddQueue(&queues.Items[i])
		}
	}
}
