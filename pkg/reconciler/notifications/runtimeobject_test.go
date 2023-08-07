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

package notifications_test

import (
	"context"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/events"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cache"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"github.com/tektoncd/pipeline/pkg/reconciler/notifications"
	ntesting "github.com/tektoncd/pipeline/pkg/reconciler/notifications/testing"
	"github.com/tektoncd/pipeline/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
)

// TestReconcileRuntimeObject runs reconcile with a cloud event sink configured
// and ensures that the event logic is correctly invoked for all supported types
func TestReconcileRuntimeObject(t *testing.T) {
	cms := []*corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetEventsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				"sink": "http://synk:8080",
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				"send-cloudevents-for-runs": "true",
			},
		},
	}

	for _, tc := range []struct {
		name      string
		runObject v1beta1.RunObject
	}{{
		name:      "v1 TaskRun",
		runObject: &v1.TaskRun{},
	}, {
		name:      "v1 PipelineRun",
		runObject: &v1.PipelineRun{},
	}, {
		name:      "v1beta1 TaskRun",
		runObject: &v1beta1.TaskRun{},
	}, {
		name:      "v1beta1 PipelineRun",
		runObject: &v1beta1.PipelineRun{},
	}, {
		name:      "v1beta1 CustomRun",
		runObject: &v1beta1.CustomRun{},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mock EmitCloudEvents
			calls := []ntesting.TestEmitCloudEventsParams{}
			events.EmitCloudEvents = func(ctx context.Context, object runtime.Object) {
				calls = append(calls, ntesting.TestEmitCloudEventsParams{
					Ctx:    ctx,
					Object: object,
				})
			}

			d := test.Data{
				ConfigMaps: cms,
			}
			testAssets, cancel := ntesting.InitializeTestAssets(t, &d)
			defer cancel()
			clients := testAssets.Clients
			reconciler := &ntesting.FakeReconciler{}
			notifications.ReconcilerFromContext(testAssets.Ctx, reconciler)

			if err := notifications.ReconcileRuntimeObject(testAssets.Ctx, reconciler, tc.runObject); err != nil {
				t.Errorf("didn't expect an error, but got one: %v", err)
			}

			if len(calls) != 1 {
				t.Errorf("expected one call to EmitCloudEvents, got: %d", len(calls))
			}

			// Check the context
			ctx := calls[0].Ctx
			if ceClient := cloudevent.Get(ctx); ceClient == nil {
				t.Error("expected the cloudevents client in the context, but got none")
			}
			if cacheClient := cache.Get(ctx); cacheClient == nil {
				t.Error("expected the cache client in the context, but got none")
			}

			for _, a := range clients.Kube.Actions() {
				aVerb := a.GetVerb()
				if aVerb != "get" && aVerb != "list" && aVerb != "watch" {
					t.Errorf("Expected only read actions to be logged in the kubeclient, got %s", aVerb)
				}
			}

			// Check that the object is the same passed to reconcile
			runObject := calls[0].Object
			if runObject != tc.runObject {
				t.Error("expected EmitCloudEvents to receive exactly the same object from the reconcile")
			}
		})
	}
}
