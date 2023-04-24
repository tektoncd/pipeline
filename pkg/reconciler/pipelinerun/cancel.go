/*
Copyright 2019 The Tekton Authors

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

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
	jsonpatch "gomodules.xyz/jsonpatch/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
)

var cancelTaskRunPatchBytes, cancelCustomRunPatchBytes []byte

func init() {
	var err error
	cancelTaskRunPatchBytes, err = json.Marshal([]jsonpatch.JsonPatchOperation{
		{
			Operation: "add",
			Path:      "/spec/status",
			Value:     v1.TaskRunSpecStatusCancelled,
		},
		{
			Operation: "add",
			Path:      "/spec/statusMessage",
			Value:     v1.TaskRunCancelledByPipelineMsg,
		}})
	if err != nil {
		log.Fatalf("failed to marshal TaskRun cancel patch bytes: %v", err)
	}
	cancelCustomRunPatchBytes, err = json.Marshal([]jsonpatch.JsonPatchOperation{
		{
			Operation: "add",
			Path:      "/spec/status",
			Value:     v1beta1.CustomRunSpecStatusCancelled,
		},
		{
			Operation: "add",
			Path:      "/spec/statusMessage",
			Value:     v1beta1.CustomRunCancelledByPipelineMsg,
		}})
	if err != nil {
		log.Fatalf("failed to marshal CustomRun cancel patch bytes: %v", err)
	}
}

func cancelCustomRun(ctx context.Context, runName string, namespace string, clientSet clientset.Interface) error {
	_, err := clientSet.TektonV1beta1().CustomRuns(namespace).Patch(ctx, runName, types.JSONPatchType, cancelCustomRunPatchBytes, metav1.PatchOptions{}, "")
	if errors.IsNotFound(err) {
		// The resource may have been deleted in the meanwhile, but we should
		// still be able to cancel the PipelineRun
		return nil
	}
	return err
}

func cancelTaskRun(ctx context.Context, taskRunName string, namespace string, clientSet clientset.Interface) error {
	_, err := clientSet.TektonV1().TaskRuns(namespace).Patch(ctx, taskRunName, types.JSONPatchType, cancelTaskRunPatchBytes, metav1.PatchOptions{}, "")
	if errors.IsNotFound(err) {
		// The resource may have been deleted in the meanwhile, but we should
		// still be able to cancel the PipelineRun
		return nil
	}
	return err
}

// cancelPipelineRun marks the PipelineRun as cancelled and any resolved TaskRun(s) too.
func cancelPipelineRun(ctx context.Context, logger *zap.SugaredLogger, pr *v1.PipelineRun, clientSet clientset.Interface) error {
	errs := cancelPipelineTaskRuns(ctx, logger, pr, clientSet)

	// If we successfully cancelled all the TaskRuns and Runs, we can consider the PipelineRun cancelled.
	if len(errs) == 0 {
		reason := ReasonCancelled

		pr.Status.SetCondition(&apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  reason,
			Message: fmt.Sprintf("PipelineRun %q was cancelled", pr.Name),
		})
		// update pr completed time
		pr.Status.CompletionTime = &metav1.Time{Time: time.Now()}
	} else {
		e := strings.Join(errs, "\n")
		// Indicate that we failed to cancel the PipelineRun
		pr.Status.SetCondition(&apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionUnknown,
			Reason:  ReasonCouldntCancel,
			Message: fmt.Sprintf("PipelineRun %q was cancelled but had errors trying to cancel TaskRuns and/or Runs: %s", pr.Name, e),
		})
		return fmt.Errorf("error(s) from cancelling TaskRun(s) from PipelineRun %s: %s", pr.Name, e)
	}
	return nil
}

// cancelPipelineTaskRuns patches `TaskRun` and `Run` with canceled status
func cancelPipelineTaskRuns(ctx context.Context, logger *zap.SugaredLogger, pr *v1.PipelineRun, clientSet clientset.Interface) []string {
	return cancelPipelineTaskRunsForTaskNames(ctx, logger, pr, clientSet, sets.NewString())
}

// cancelPipelineTaskRunsForTaskNames patches `TaskRun`s and `Run`s for the given task names, or all if no task names are given, with canceled status
func cancelPipelineTaskRunsForTaskNames(ctx context.Context, logger *zap.SugaredLogger, pr *v1.PipelineRun, clientSet clientset.Interface, taskNames sets.String) []string {
	errs := []string{}

	trNames, customRunNames, err := getChildObjectsFromPRStatusForTaskNames(ctx, pr.Status, taskNames)
	if err != nil {
		errs = append(errs, err.Error())
	}

	for _, taskRunName := range trNames {
		logger.Infof("cancelling TaskRun %s", taskRunName)

		if err := cancelTaskRun(ctx, taskRunName, pr.Namespace, clientSet); err != nil {
			errs = append(errs, fmt.Errorf("Failed to patch TaskRun `%s` with cancellation: %w", taskRunName, err).Error())
			continue
		}
	}

	for _, runName := range customRunNames {
		logger.Infof("cancelling CustomRun %s", runName)

		if err := cancelCustomRun(ctx, runName, pr.Namespace, clientSet); err != nil {
			errs = append(errs, fmt.Errorf("Failed to patch CustomRun `%s` with cancellation: %w", runName, err).Error())
			continue
		}
	}
	return errs
}

// getChildObjectsFromPRStatusForTaskNames returns taskruns and customruns in the PipelineRunStatus's ChildReferences,
// based on the given set of PipelineTask names. If that set is empty, all are returned.
func getChildObjectsFromPRStatusForTaskNames(ctx context.Context, prs v1.PipelineRunStatus, taskNames sets.String) ([]string, []string, error) {
	var trNames []string
	var customRunNames []string
	unknownChildKinds := make(map[string]string)

	for _, cr := range prs.ChildReferences {
		if taskNames.Len() == 0 || taskNames.Has(cr.PipelineTaskName) {
			switch cr.Kind {
			case taskRun:
				trNames = append(trNames, cr.Name)
			case customRun:
				customRunNames = append(customRunNames, cr.Name)
			default:
				unknownChildKinds[cr.Name] = cr.Kind
			}
		}
	}

	var err error
	if len(unknownChildKinds) > 0 {
		err = fmt.Errorf("found child objects of unknown kinds: %v", unknownChildKinds)
	}

	return trNames, customRunNames, err
}

// gracefullyCancelPipelineRun marks any non-final resolved TaskRun(s) as cancelled and runs finally.
func gracefullyCancelPipelineRun(ctx context.Context, logger *zap.SugaredLogger, pr *v1.PipelineRun, clientSet clientset.Interface) error {
	errs := cancelPipelineTaskRuns(ctx, logger, pr, clientSet)

	// If we successfully cancelled all the TaskRuns and Runs, we can proceed with the PipelineRun reconciliation to trigger finally.
	if len(errs) > 0 {
		e := strings.Join(errs, "\n")
		// Indicate that we failed to cancel the PipelineRun
		pr.Status.SetCondition(&apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionUnknown,
			Reason:  ReasonCouldntCancel,
			Message: fmt.Sprintf("PipelineRun %q was cancelled but had errors trying to cancel TaskRuns and/or Runs: %s", pr.Name, e),
		})
		return fmt.Errorf("error(s) from cancelling TaskRun(s) from PipelineRun %s: %s", pr.Name, e)
	}
	return nil
}
