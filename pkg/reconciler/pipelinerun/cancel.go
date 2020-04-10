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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
)

// cancelPipelineRun marks the PipelineRun as cancelled and any resolved TaskRun(s) too.
func cancelPipelineRun(logger *zap.SugaredLogger, pr *v1alpha1.PipelineRun, pipelineState []*resources.ResolvedPipelineRunTask, clientSet clientset.Interface) error {
	pr.Status.SetCondition(&apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionFalse,
		Reason:  "PipelineRunCancelled",
		Message: fmt.Sprintf("PipelineRun %q was cancelled", pr.Name),
	})
	// update pr completed time
	pr.Status.CompletionTime = &metav1.Time{Time: time.Now()}
	errs := []string{}
	for _, rprt := range pipelineState {
		if rprt.TaskRun == nil {
			// No taskrun yet, pass
			continue
		}

		logger.Infof("cancelling TaskRun %s", rprt.TaskRunName)

		// Use Patch to update the TaskRuns since the TaskRun controller may be operating on the
		// TaskRuns at the same time and trying to update the entire object may cause a race
		b, err := getCancelPatch()
		if err != nil {
			errs = append(errs, fmt.Errorf("couldn't make patch to update TaskRun cancellation: %v", err).Error())
			continue
		}
		if _, err := clientSet.TektonV1alpha1().TaskRuns(pr.Namespace).Patch(rprt.TaskRunName, types.JSONPatchType, b, ""); err != nil {
			errs = append(errs, fmt.Errorf("Failed to patch TaskRun `%s` with cancellation: %s", rprt.TaskRunName, err).Error())
			continue
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("error(s) from cancelling TaskRun(s) from PipelineRun %s: %s", pr.Name, strings.Join(errs, "\n"))
	}
	return nil
}

func getCancelPatch() ([]byte, error) {
	patches := []jsonpatch.JsonPatchOperation{{
		Operation: "add",
		Path:      "/spec/status",
		Value:     v1alpha1.TaskRunSpecStatusCancelled,
	}}
	patchBytes, err := json.Marshal(patches)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal patch bytes in order to cancel: %v", err)
	}
	return patchBytes, nil
}
