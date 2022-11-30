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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
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
func (c *Reconciler) ReconcileKind(ctx context.Context, r *v1beta1.CustomRun) kreconciler.Event {
	logger := logging.FromContext(ctx)
	logger.Infof("Reconciling %s/%s", r.Namespace, r.Name)

	if r.Spec.CustomRef == nil ||
		r.Spec.CustomRef.APIVersion != "wait.testing.tekton.dev/v1beta1" || r.Spec.CustomRef.Kind != "Wait" {
		// This is not a CustomRun we should have been notified about; do nothing.
		return nil
	}

	if r.Spec.CustomRef.Name != "" {
		logger.Errorf("Found unexpected ref name: %s", r.Spec.CustomRef.Name)
		r.Status.MarkCustomRunFailed("UnexpectedName", "Found unexpected ref name: %s", r.Spec.CustomRef.Name)
		return nil
	}

	expr := r.Spec.GetParam("duration")
	if expr == nil || expr.Value.StringVal == "" {
		logger.Error("The duration param was not passed")
		r.Status.MarkCustomRunFailed("MissingDuration", "The duration param was not passed")
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
		logger.Errorf("Found unexpected params: %v", found)
		r.Status.MarkCustomRunFailed("UnexpectedParams", "Found unexpected params: %v", found)
		return nil
	}

	duration, err := time.ParseDuration(expr.Value.StringVal)
	if err != nil {
		logger.Errorf("The duration param was invalid: %v", err)
		r.Status.MarkCustomRunFailed("InvalidDuration", "The duration param was invalid: %v", err)
		return nil
	}

	timeout := r.GetTimeout()
	if duration == timeout {
		logger.Error("Spec.Timeout shouldn't equal duration")
		r.Status.MarkCustomRunFailed("InvalidTimeOut", "Spec.Timeout shouldn't equal duration")
		return nil
	}

	if !r.HasStarted() {
		logger.Info("CustomRun hasn't started, start it")
		r.Status.InitializeConditions()
		r.Status.StartTime = &metav1.Time{Time: c.Clock.Now()}
		r.Status.MarkCustomRunRunning("Running", "Waiting for duration to elapse")
	}

	// Ignore completed waits.
	if r.IsDone() {
		logger.Info("CustomRun is finished, done reconciling")
		return nil
	}

	// Skip if the CustomRun is cancelled.
	if r.IsCancelled() {
		logger.Infof("The CustomRun %v has been cancelled", r.GetName())
		r.Status.CompletionTime = &metav1.Time{Time: c.Clock.Now()}
		var msg string = fmt.Sprint(r.Spec.StatusMessage)
		if msg == "" {
			msg = WaitTaskCancelledMsg
		}
		logger.Infof(msg)
		r.Status.MarkCustomRunFailed("Cancelled", msg)
		return nil
	}

	elapsed := c.Clock.Since(r.Status.StartTime.Time)

	// Custom Task is running and not timed out
	if r.Status.StartTime != nil && elapsed <= duration && elapsed <= timeout {
		logger.Infof("The CustomRun %s is running", r.GetName())
		waitTime := duration.Nanoseconds()
		if timeout.Nanoseconds() < waitTime {
			waitTime = timeout.Nanoseconds()
		}
		return controller.NewRequeueAfter(time.Duration(waitTime))
	}

	if r.Status.StartTime != nil && elapsed > duration && elapsed <= timeout {
		logger.Infof("The CustomRun %v finished at %v", r.GetName(), &metav1.Time{Time: c.Clock.Now()})
		r.Status.CompletionTime = &metav1.Time{Time: c.Clock.Now()}
		r.Status.MarkCustomRunSucceeded("DurationElapsed", "The wait duration has elapsed")
		return nil
	}

	// Custom Task Timed Out
	if r.Status.StartTime != nil && elapsed > timeout {
		logger.Infof("The CustomRun %v timed out", r.GetName())
		// Retry if the current RetriesStatus hasn't reached the retries limit
		if r.Spec.Retries > len(r.Status.RetriesStatus) {
			logger.Infof("CustomRun timed out, retrying... %#v", r.Status)
			retryCustomRun(r, apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: v1.ConditionFalse,
				Reason: "TimedOut",
			})
			return controller.NewRequeueImmediately()
		}

		r.Status.CompletionTime = &metav1.Time{Time: c.Clock.Now()}
		r.Status.MarkCustomRunFailed("TimedOut", WaitTaskFailedOnRunTimeoutMsg)

		return nil
	}

	// Don't emit events on nop-reconciliations, it causes scale problems.
	return nil
}

func retryCustomRun(r *v1beta1.CustomRun, c apis.Condition) {
	// Add retry history
	newStatus := r.Status.DeepCopy()
	newStatus.RetriesStatus = nil
	condSet := r.GetConditionSet()
	condSet.Manage(newStatus).SetCondition(c)
	r.Status.RetriesStatus = append(r.Status.RetriesStatus, *newStatus)

	// Clear status
	r.Status.StartTime = nil

	r.Status.MarkCustomRunRunning("Retrying", "")
}
