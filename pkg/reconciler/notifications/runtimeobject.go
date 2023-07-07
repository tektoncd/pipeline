/*
Copyright 2023 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package notifications

import (
	"context"

	bc "github.com/allegro/bigcache/v3"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/events"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cache"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for Configuration resources.
type Reconciler interface {
	GetCloudEventsClient() cloudevent.CEClient
	GetCacheClient() *bc.BigCache
	SetCloudEventsClient(cloudevent.CEClient)
	SetCacheClient(*bc.BigCache)
}

// ReconcileRuntimeObject observes a v1beta1.RunObject and triggers notifications
func ReconcileRuntimeObject(ctx context.Context, c Reconciler, readOnlyRun v1beta1.RunObject) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	ctx = cloudevent.ToContext(ctx, c.GetCloudEventsClient())
	ctx = cache.ToContext(ctx, c.GetCacheClient())

	logger.Infof("reconciling %s", readOnlyRun.GetObjectMeta().GetName())

	condition := readOnlyRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded)
	logger.Debugf("customRun %s, condition: %s", readOnlyRun.GetObjectMeta().GetName(), condition)

	events.EmitCloudEvents(ctx, readOnlyRun)
	return nil
}
