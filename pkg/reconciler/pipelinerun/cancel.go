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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	jsonpatch "gomodules.xyz/jsonpatch/v2"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
)

// cancelPipelineRun marks the PipelineRun as cancelled and any resolved TaskRun(s) too.
func cancelPipelineRun(logger *zap.SugaredLogger, pr *v1beta1.PipelineRun, clientSet clientset.Interface) error {
	errs := []string{}

	// Use Patch to update the TaskRuns since the TaskRun controller may be operating on the
	// TaskRuns at the same time and trying to update the entire object may cause a race
	b, err := getCancelPatch()
	if err != nil {
		return fmt.Errorf("couldn't make patch to update TaskRun cancellation: %v", err)
	}

	// Loop over the TaskRuns in the PipelineRun status.
	// If a TaskRun is not in the status yet we should not cancel it anyways.
	for taskRunName := range pr.Status.TaskRuns {
		logger.Infof("cancelling TaskRun %s", taskRunName)

		if _, err := clientSet.TektonV1beta1().TaskRuns(pr.Namespace).Patch(taskRunName, types.JSONPatchType, b, ""); err != nil {
			errs = append(errs, fmt.Errorf("Failed to patch TaskRun `%s` with cancellation: %s", taskRunName, err).Error())
			continue
		}
	}
	// If we successfully cancelled all the TaskRuns, we can consider the PipelineRun cancelled.
	if len(errs) == 0 {
		pr.Status.SetCondition(&apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  ReasonCancelled,
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
			Message: fmt.Sprintf("PipelineRun %q was cancelled but had errors trying to cancel TaskRuns: %s", pr.Name, e),
		})
		return fmt.Errorf("error(s) from cancelling TaskRun(s) from PipelineRun %s: %s", pr.Name, e)
	}
	return nil
}

func getCancelPatch() ([]byte, error) {
	patches := []jsonpatch.JsonPatchOperation{{
		Operation: "add",
		Path:      "/spec/status",
		Value:     v1beta1.TaskRunSpecStatusCancelled,
	}}
	patchBytes, err := json.Marshal(patches)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal patch bytes in order to cancel: %v", err)
	}
	return patchBytes, nil
}
