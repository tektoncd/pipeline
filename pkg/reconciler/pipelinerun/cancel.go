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

	"github.com/tektoncd/pipeline/pkg/apis/config"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
	jsonpatch "gomodules.xyz/jsonpatch/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
)

var cancelTaskRunPatchBytes, cancelRunPatchBytes, cancelPipelineRunPatchBytes []byte

func init() {
	var err error
	cancelTaskRunPatchBytes, err = json.Marshal([]jsonpatch.JsonPatchOperation{{
		Operation: "add",
		Path:      "/spec/status",
		Value:     v1beta1.TaskRunSpecStatusCancelled,
	}})
	if err != nil {
		log.Fatalf("failed to marshal TaskRun cancel patch bytes: %v", err)
	}
	cancelRunPatchBytes, err = json.Marshal([]jsonpatch.JsonPatchOperation{{
		Operation: "add",
		Path:      "/spec/status",
		Value:     v1alpha1.RunSpecStatusCancelled,
	}})
	if err != nil {
		log.Fatalf("failed to marshal Run cancel patch bytes: %v", err)
	}
	cancelPipelineRunPatchBytes, err = json.Marshal([]jsonpatch.JsonPatchOperation{{
		Operation: "add",
		Path:      "/spec/status",
		Value:     v1beta1.PipelineRunSpecStatusCancelled,
	}})
	if err != nil {
		log.Fatalf("failed to marshal PipelineRun cancel patch bytes: %v", err)
	}
}

func cancelRun(ctx context.Context, runName string, namespace string, clientSet clientset.Interface) error {
	_, err := clientSet.TektonV1alpha1().Runs(namespace).Patch(ctx, runName, types.JSONPatchType, cancelRunPatchBytes, metav1.PatchOptions{}, "")
	return err
}

// cancelPipelineRun marks the PipelineRun as cancelled and any resolved TaskRun(s) too.
func cancelPipelineRun(ctx context.Context, logger *zap.SugaredLogger, pr *v1beta1.PipelineRun, clientSet clientset.Interface) error {
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
func cancelPipelineTaskRuns(ctx context.Context, logger *zap.SugaredLogger, pr *v1beta1.PipelineRun, clientSet clientset.Interface) []string {
	errs := []string{}

	trNames, runNames, prNames, err := getChildObjectsFromPRStatus(ctx, pr.Status)
	if err != nil {
		errs = append(errs, err.Error())
	}

	for _, taskRunName := range trNames {
		logger.Infof("cancelling TaskRun %s", taskRunName)

		if _, err := clientSet.TektonV1beta1().TaskRuns(pr.Namespace).Patch(ctx, taskRunName, types.JSONPatchType, cancelTaskRunPatchBytes, metav1.PatchOptions{}, ""); err != nil {
			errs = append(errs, fmt.Errorf("failed to patch TaskRun `%s` with cancellation: %s", taskRunName, err).Error())
			continue
		}
	}

	for _, runName := range runNames {
		logger.Infof("cancelling Run %s", runName)

		if err := cancelRun(ctx, runName, pr.Namespace, clientSet); err != nil {
			errs = append(errs, fmt.Errorf("failed to patch Run `%s` with cancellation: %s", runName, err).Error())
			continue
		}
	}

	for _, prName := range prNames {
		logger.Infof("cancelling PipelineRun %s", prName)

		if _, err := clientSet.TektonV1beta1().PipelineRuns(pr.Namespace).Patch(ctx, prName, types.JSONPatchType, cancelPipelineRunPatchBytes, metav1.PatchOptions{}, ""); err != nil {
			errs = append(errs, fmt.Errorf("failed to patch PipelineRun `%s` with cancellation: %s", prName, err).Error())
			continue
		}
	}

	return errs
}

// getChildObjectsFromPRStatus returns taskruns, runs, and child PipelineRuns in the PipelineRunStatus's ChildReferences or TaskRuns/Runs,
// based on the value of the embedded status flag.
func getChildObjectsFromPRStatus(ctx context.Context, prs v1beta1.PipelineRunStatus) ([]string, []string, []string, error) {
	cfg := config.FromContextOrDefaults(ctx)

	var trNames []string
	var runNames []string
	var prNames []string
	unknownChildKinds := make(map[string]string)

	if cfg.FeatureFlags.EmbeddedStatus != config.FullEmbeddedStatus {
		for _, cr := range prs.ChildReferences {
			switch cr.Kind {
			case "TaskRun":
				trNames = append(trNames, cr.Name)
			case "Run":
				runNames = append(runNames, cr.Name)
			case "PipelineRun":
				prNames = append(prNames, cr.Name)
			default:
				unknownChildKinds[cr.Name] = cr.Kind
			}
		}
	} else {
		for trName := range prs.TaskRuns {
			trNames = append(trNames, trName)
		}
		for runName := range prs.Runs {
			runNames = append(runNames, runName)
		}
	}

	var err error
	if len(unknownChildKinds) > 0 {
		err = fmt.Errorf("found child objects of unknown kinds: %v", unknownChildKinds)
	}

	return trNames, runNames, prNames, err
}

// gracefullyCancelPipelineRun marks any non-final resolved TaskRun(s) as cancelled and runs finally.
func gracefullyCancelPipelineRun(ctx context.Context, logger *zap.SugaredLogger, pr *v1beta1.PipelineRun, clientSet clientset.Interface) error {
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
