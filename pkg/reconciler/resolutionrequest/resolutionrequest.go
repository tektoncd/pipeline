/*
Copyright 2022 The Tekton Authors

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

package resolutionrequest

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	resolutionclientset "github.com/tektoncd/pipeline/pkg/client/resolution/clientset/versioned"
	rrreconciler "github.com/tektoncd/pipeline/pkg/client/resolution/injection/reconciler/resolution/v1beta1/resolutionrequest"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/reconciler"
)

// Reconciler is a knative reconciler for processing ResolutionRequest objects.
// It patches lifecycle-owned status fields itself and must be configured with
// SkipStatusUpdates so generated updates do not overwrite resolver-owned fields.
type Reconciler struct {
	// client applies lifecycle-only status patches.
	client resolutionclientset.Interface
	clock  clock.PassiveClock
}

var _ rrreconciler.Interface = (*Reconciler)(nil)

// ReconcileKind processes updates to ResolutionRequests, sets status
// fields on it, and returns any errors experienced along the way.
func (r *Reconciler) ReconcileKind(ctx context.Context, rr *v1beta1.ResolutionRequest) reconciler.Event {
	if rr == nil {
		return nil
	}

	original := rr.DeepCopy()
	reconciler.PreProcessReconcile(ctx, rr)
	event := r.reconcile(ctx, rr)
	reconciler.PostProcessReconcile(ctx, rr, original)

	if equality.Semantic.DeepEqual(lifecycleStatus(original), lifecycleStatus(rr)) {
		return event
	}
	if err := r.patchLifecycleStatus(ctx, rr); err != nil {
		if apierrors.IsConflict(err) {
			// RetryUpdateConflicts would replay lifecycle status derived from this
			// stale object. Requeue so lifecycle status is recomputed from fresh state.
			return controller.NewRequeueAfter(0)
		}
		return fmt.Errorf("failed to update resource lifecycle status: %w", err)
	}
	return event
}

func (r *Reconciler) reconcile(ctx context.Context, rr *v1beta1.ResolutionRequest) reconciler.Event {
	if rr.IsDone() {
		return nil
	}

	if rr.Status.GetCondition(apis.ConditionSucceeded) == nil {
		rr.Status.InitializeConditions()
	}

	maximumResolutionDuration := config.FromContextOrDefaults(ctx).Defaults.DefaultMaximumResolutionTimeout
	switch {
	case rr.IsResolved():
		rr.Status.MarkSucceeded()
	case requestDuration(rr) > maximumResolutionDuration:
		rr.Status.MarkFailed(resolutioncommon.ReasonResolutionTimedOut, timeoutMessage(maximumResolutionDuration))
	default:
		rr.Status.MarkInProgress(resolutioncommon.MessageWaitingForResolver)
		return controller.NewRequeueAfter(maximumResolutionDuration - requestDuration(rr))
	}

	return nil
}

// lifecycleStatusPatch intentionally excludes resolver-owned payload fields and annotations.
type lifecycleStatusPatch struct {
	Metadata map[string]string `json:"metadata"`
	Status   duckv1.Status     `json:"status"`
}

func lifecycleStatus(rr *v1beta1.ResolutionRequest) duckv1.Status {
	return duckv1.Status{
		ObservedGeneration: rr.Status.ObservedGeneration,
		Conditions:         rr.Status.Conditions,
	}
}

func (r *Reconciler) patchLifecycleStatus(ctx context.Context, rr *v1beta1.ResolutionRequest) error {
	patchBytes, err := json.Marshal(lifecycleStatusPatch{
		Metadata: map[string]string{"resourceVersion": rr.ResourceVersion},
		Status:   lifecycleStatus(rr),
	})
	if err != nil {
		return err
	}
	_, err = r.client.ResolutionV1beta1().ResolutionRequests(rr.Namespace).Patch(
		ctx, rr.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{}, "status")
	return err
}

// requestDuration returns the amount of time that has passed since a
// given ResolutionRequest was created.
func requestDuration(rr *v1beta1.ResolutionRequest) time.Duration {
	creationTime := rr.ObjectMeta.CreationTimestamp.DeepCopy().Time.UTC()
	return time.Now().UTC().Sub(creationTime)
}

func timeoutMessage(timeout time.Duration) string {
	return fmt.Sprintf("resolution took longer than global timeout of %s", timeout)
}
