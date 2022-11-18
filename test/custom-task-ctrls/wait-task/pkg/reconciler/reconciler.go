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
	"fmt"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	kreconciler "knative.dev/pkg/reconciler"
)

const (
	WaitTaskCancelledMsg          string = "Wait Task cancelled."
	WaitTaskFailedOnRunTimeoutMsg string = "Wait Task failed as it times out."
)

// Reconciler implements controller.Reconciler for Configuration resources.
type Reconciler struct {
	Clock clock.PassiveClock
}

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

	// Skip if the Run is cancelled.
	if r.IsCancelled() {
		logger.Infof("The Custom Task Run %v has been cancelled", r.GetName())
		r.Status.CompletionTime = &metav1.Time{Time: c.Clock.Now()}
		var msg string = fmt.Sprint(r.Spec.StatusMessage)
		if msg == "" {
			msg = WaitTaskCancelledMsg
		}
		r.Status.MarkRunFailed("Cancelled", msg)
		return nil
	}

	if !r.HasStarted() {
		logger.Info("Run hasn't started, start it")
		r.Status.InitializeConditions()
		r.Status.StartTime = &metav1.Time{Time: c.Clock.Now()}
		r.Status.MarkRunRunning("Running", "Waiting for duration to elapse")
		controller.NewRequeueImmediately()
	}

	duration, err := time.ParseDuration(expr.Value.StringVal)
	if err != nil {
		r.Status.MarkRunFailed("InvalidDuration", "The duration param was invalid: %v", err)
		return nil
	}
	timeout := r.GetTimeout()
	if duration == timeout {
		r.Status.MarkRunFailed("InvalidTimeOut", "Spec.Timeout shouldn't equal duration")
		return nil
	}
	elapsed := c.Clock.Since(r.Status.StartTime.Time)

	// Custom Task is running and not timed out
	if r.Status.StartTime != nil && elapsed <= duration && elapsed <= timeout {
		logger.Infof("The Custom Task Run %s is running", r.GetName())
		waitTime := duration.Nanoseconds()
		if timeout.Nanoseconds() < waitTime {
			waitTime = timeout.Nanoseconds()
		}
		return controller.NewRequeueAfter(time.Duration(waitTime))
	}

	if r.Status.StartTime != nil && elapsed > duration && elapsed <= timeout {
		logger.Infof("The Custom Task Run %v finished", r.GetName())
		r.Status.CompletionTime = &metav1.Time{Time: c.Clock.Now()}
		r.Status.MarkRunSucceeded("DurationElapsed", "The wait duration has elapsed")
		return nil
	}

	// Custom Task timed out
	if r.Status.StartTime != nil && elapsed > timeout {
		logger.Infof("The Custom Task Run %v timed out", r.GetName())

		// Retry if the current RetriesStatus hasn't reached the retries limit
		if r.Spec.Retries > len(r.Status.RetriesStatus) {
			logger.Infof("Run timed out, retrying... %#v", r.Status)

			addRetryHistory(r, apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: v1.ConditionFalse,
				Reason: "TimedOut",
			})
			retryRun(r)
			return controller.NewRequeueImmediately()
		}

		r.Status.CompletionTime = &metav1.Time{Time: c.Clock.Now()}
		r.Status.MarkRunFailed("TimedOut", WaitTaskFailedOnRunTimeoutMsg)

		return nil
	}

	// Don't emit events on nop-reconciliations, it causes scale problems.
	return nil
}

func addRetryHistory(r *v1alpha1.Run, c apis.Condition) {
	newStatus := *r.Status.DeepCopy()
	newStatus.RetriesStatus = nil

	condSet := r.GetConditionSet()
	condSet.Manage(&newStatus).SetCondition(c)

	r.Status.RetriesStatus = append(r.Status.RetriesStatus, newStatus)
}

func retryRun(run *v1alpha1.Run) {
	// Clear status
	run.Status.StartTime = nil
	run.Status.CompletionTime = nil
}
