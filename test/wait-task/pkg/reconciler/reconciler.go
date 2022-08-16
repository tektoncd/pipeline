/*
Copyright 2021 The Tekton Authors

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

package reconciler

import (
	"context"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	kreconciler "knative.dev/pkg/reconciler"
)

type Reconciler struct{}

// ReconcileKind implements Interface.ReconcileKind.
func (c *Reconciler) ReconcileKind(ctx context.Context, r *v1alpha1.Run) kreconciler.Event {
	logger := logging.FromContext(ctx)
	logger.Infof("Reconciling %s/%s", r.Namespace, r.Name)

	// Ignore completed waits.
	if r.IsDone() {
		logger.Info("Run is finished, done reconciling")
		return nil
	}

	if r.Spec.Ref == nil ||
		r.Spec.Ref.APIVersion != "wait.testing.tekton.dev/v1alpha1" || r.Spec.Ref.Kind != "Wait" {
		// This is not a Run we should have been notified about; do nothing.
		return nil
	}
	if r.Spec.Ref.Name != "" {
		r.Status.MarkRunFailed("UnexpectedName", "Found unexpected ref name: %s", r.Spec.Ref.Name)
		return nil
	}

	expr := r.Spec.GetParam("duration")
	if expr == nil || expr.Value.StringVal == "" {
		r.Status.MarkRunFailed("MissingDuration", "The duration param was not passed")
		return nil
	}
	if len(r.Spec.Params) != 1 {
		var found []string
		for _, p := range r.Spec.Params {
			if p.Name == "duration" {
				continue
			}
			found = append(found, p.Name)
		}
		r.Status.MarkRunFailed("UnexpectedParams", "Found unexpected params: %v", found)
		return nil
	}

	dur, err := time.ParseDuration(expr.Value.StringVal)
	if err != nil {
		r.Status.MarkRunFailed("InvalidDuration", "The duration param was invalid: %v", err)
		return nil
	}

	if r.Status.StartTime == nil {
		now := metav1.Now()
		r.Status.StartTime = &now
		r.Status.MarkRunRunning("Running", "Waiting for duration to elapse")
	}

	done := r.Status.StartTime.Time.Add(dur)

	if time.Now().After(done) {
		now := metav1.Now()
		r.Status.CompletionTime = &now
		r.Status.MarkRunSucceeded("DurationElapsed", "The wait duration has elapsed")
	} else {
		// Enqueue another check when the timeout should be elapsed.
		return controller.NewRequeueAfter(time.Until(r.Status.StartTime.Time.Add(dur)))
	}

	// Don't emit events on nop-reconciliations, it causes scale problems.
	return nil
}
