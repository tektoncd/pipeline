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
	"fmt"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	rrreconciler "github.com/tektoncd/pipeline/pkg/client/resolution/injection/reconciler/resolution/v1beta1/resolutionrequest"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	"k8s.io/utils/clock"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/reconciler"
)

// Reconciler is a knative reconciler for processing ResolutionRequest
// objects
type Reconciler struct {
	clock clock.PassiveClock
}

var _ rrreconciler.Interface = (*Reconciler)(nil)

// TODO(sbwsg): This should be exposed via ConfigMap using a config
// store similarly to Tekton Pipelines'.
const defaultMaximumResolutionDuration = 1 * time.Minute

// ReconcileKind processes updates to ResolutionRequests, sets status
// fields on it, and returns any errors experienced along the way.
func (r *Reconciler) ReconcileKind(ctx context.Context, rr *v1beta1.ResolutionRequest) reconciler.Event {
	if rr == nil {
		return nil
	}

	if rr.IsDone() {
		return nil
	}

	if rr.Status.GetCondition(apis.ConditionSucceeded) == nil {
		rr.Status.InitializeConditions()
	}

	switch {
	case rr.Status.Data != "":
		rr.Status.MarkSucceeded()
	case requestDuration(rr) > defaultMaximumResolutionDuration:
		rr.Status.MarkFailed(resolutioncommon.ReasonResolutionTimedOut, timeoutMessage())
	default:
		rr.Status.MarkInProgress(resolutioncommon.MessageWaitingForResolver)
		return controller.NewRequeueAfter(defaultMaximumResolutionDuration - requestDuration(rr))
	}

	return nil
}

// requestDuration returns the amount of time that has passed since a
// given ResolutionRequest was created.
func requestDuration(rr *v1beta1.ResolutionRequest) time.Duration {
	creationTime := rr.ObjectMeta.CreationTimestamp.DeepCopy().Time.UTC()
	return time.Now().UTC().Sub(creationTime)
}

func timeoutMessage() string {
	return fmt.Sprintf("resolution took longer than global timeout of %s", defaultMaximumResolutionDuration)
}
