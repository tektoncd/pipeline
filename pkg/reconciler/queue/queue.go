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

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/queue/v1alpha1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1beta1"
	listersv1alpha1 "github.com/tektoncd/pipeline/pkg/client/queue/listers/queue/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/queue/queuemanager/manager"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler is a knative reconciler for processing queue
// objects
type Reconciler struct {
	QueueLister       listersv1alpha1.QueueLister
	PipelineClientSet clientset.Interface
	PipelineRunLister listers.PipelineRunLister
}

func (r *Reconciler) ReconcileKind(ctx context.Context, queue *v1alpha1.Queue) pkgreconciler.Event {
	if queue == nil {
		return nil
	}
	cfg := config.FromContextOrDefaults(ctx)
	if cfg.FeatureFlags.EnableAPIFields == config.AlphaAPIFields {
		manager.IdempotentAddQueue(queue)
	}
	return nil
}
