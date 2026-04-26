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

package pipelinerun

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	pipelineErrors "github.com/tektoncd/pipeline/pkg/apis/pipeline/errors"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"gomodules.xyz/jsonpatch/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
)

var timeoutTaskRunPatchBytes, timeoutCustomRunPatchBytes []byte

func init() {
	var err error
	timeoutTaskRunPatchBytes, err = json.Marshal([]jsonpatch.JsonPatchOperation{
		{
			Operation: "add",
			Path:      "/spec/status",
			Value:     v1.TaskRunSpecStatusCancelled,
		},
		{
			Operation: "add",
			Path:      "/spec/statusMessage",
			Value:     v1.TaskRunCancelledByPipelineTimeoutMsg,
		}})
	if err != nil {
		log.Fatalf("failed to marshal TaskRun timeout patch bytes: %v", err)
	}
	timeoutCustomRunPatchBytes, err = json.Marshal([]jsonpatch.JsonPatchOperation{
		{
			Operation: "add",
			Path:      "/spec/status",
			Value:     v1beta1.CustomRunSpecStatusCancelled,
		},
		{
			Operation: "add",
			Path:      "/spec/statusMessage",
			Value:     v1beta1.CustomRunCancelledByPipelineTimeoutMsg,
		}})
	if err != nil {
		log.Fatalf("failed to marshal CustomRun timeout patch bytes: %v", err)
	}
}

// timeoutPipelineRun marks the PipelineRun as timed out and any resolved TaskRun(s) too.
func timeoutPipelineRun(ctx context.Context, logger *zap.SugaredLogger, pr *v1.PipelineRun, clientSet clientset.Interface) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().Tracer(TracerName).Start(ctx, "timeoutPipelineRun",
		trace.WithAttributes(
			attribute.String("pipelineRun.name", pr.Name),
			attribute.String("pipelineRun.namespace", pr.Namespace),
		))
	defer span.End()

	errs := timeoutPipelineTasks(ctx, logger, pr, clientSet)

	// If we successfully timed out all the TaskRuns and Runs, we can consider the PipelineRun timed out.
	if len(errs) == 0 {
		pr.SetTimeoutCondition(ctx)
		// update pr completed time
		pr.Status.CompletionTime = &metav1.Time{Time: time.Now()}
	} else {
		e := strings.Join(errs, "\n")
		// Indicate that we failed to time out the PipelineRun
		pr.Status.SetCondition(&apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionUnknown,
			Reason:  v1.PipelineRunReasonCouldntTimeOut.String(),
			Message: fmt.Sprintf("PipelineRun %q was timed out but had errors trying to time out TaskRuns and/or Runs: %s", pr.Name, e),
		})
		combinedErr := fmt.Errorf("error(s) from timing out TaskRun(s) from PipelineRun %s: %s", pr.Name, e)
		span.SetStatus(codes.Error, combinedErr.Error())
		span.RecordError(combinedErr)
		return combinedErr
	}
	return nil
}

func timeoutCustomRun(ctx context.Context, customRunName string, namespace string, clientSet clientset.Interface) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().Tracer(TracerName).Start(ctx, "timeoutCustomRun",
		trace.WithAttributes(
			attribute.String("customRun.name", customRunName),
			attribute.String("customRun.namespace", namespace),
		))
	defer span.End()

	_, err := clientSet.TektonV1beta1().CustomRuns(namespace).Patch(ctx, customRunName, types.JSONPatchType, timeoutCustomRunPatchBytes, metav1.PatchOptions{}, "")
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}
	return err
}

func timeoutTaskRun(ctx context.Context, taskRunName string, namespace string, clientSet clientset.Interface) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().Tracer(TracerName).Start(ctx, "timeoutTaskRun",
		trace.WithAttributes(
			attribute.String("taskRun.name", taskRunName),
			attribute.String("taskRun.namespace", namespace),
		))
	defer span.End()

	_, err := clientSet.TektonV1().TaskRuns(namespace).Patch(ctx, taskRunName, types.JSONPatchType, timeoutTaskRunPatchBytes, metav1.PatchOptions{}, "")
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}
	return err
}

// timeoutPipelineTaskRuns patches `TaskRun` and `Run` with canceled status and an appropriate message
func timeoutPipelineTasks(ctx context.Context, logger *zap.SugaredLogger, pr *v1.PipelineRun, clientSet clientset.Interface) []string {
	return timeoutPipelineTasksForTaskNames(ctx, logger, pr, clientSet, sets.NewString())
}

// timeoutPipelineTasksForTaskNames patches `TaskRun`s and `Run`s for the given task names, or all if no task names are given, with canceled status and appropriate message
func timeoutPipelineTasksForTaskNames(ctx context.Context, logger *zap.SugaredLogger, pr *v1.PipelineRun, clientSet clientset.Interface, taskNames sets.String) []string {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().Tracer(TracerName).Start(ctx, "timeoutPipelineTasksForTaskNames",
		trace.WithAttributes(
			attribute.String("pipelineRun.name", pr.Name),
			attribute.String("pipelineRun.namespace", pr.Namespace),
		))
	defer span.End()

	errs := []string{}

	trNames, customRunNames, err := getChildObjectsFromPRStatusForTaskNames(ctx, pr.Status, taskNames)
	if err != nil {
		errs = append(errs, err.Error())
	}

	for _, taskRunName := range trNames {
		logger.Infof("patching TaskRun %s for timeout", taskRunName)

		if err := timeoutTaskRun(ctx, taskRunName, pr.Namespace, clientSet); err != nil {
			if pipelineErrors.IsImmutableTaskRunSpecError(err) {
				// The TaskRun may have completed and the spec field is immutable, we should ignore this error.
				continue
			}
			errs = append(errs, fmt.Errorf("failed to patch TaskRun `%s` with timeout: %w", taskRunName, err).Error())
			continue
		}
	}

	for _, custonRunName := range customRunNames {
		logger.Infof("patching CustomRun %s for timeout", custonRunName)

		if err := timeoutCustomRun(ctx, custonRunName, pr.Namespace, clientSet); err != nil {
			errs = append(errs, fmt.Errorf("failed to patch CustomRun `%s` with timeout: %w", custonRunName, err).Error())
			continue
		}
	}

	if len(errs) > 0 {
		combined := strings.Join(errs, "; ")
		span.SetStatus(codes.Error, combined)
		span.RecordError(fmt.Errorf("%s", combined))
	}
	return errs
}
