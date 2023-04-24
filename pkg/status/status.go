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

package status

import (
	"context"
	"fmt"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetTaskRunStatusForPipelineTask takes a child reference and returns the actual TaskRunStatus
// for the PipelineTask. It returns an error if the child reference's kind isn't TaskRun.
func GetTaskRunStatusForPipelineTask(ctx context.Context, client versioned.Interface, ns string, childRef v1.ChildStatusReference) (*v1.TaskRunStatus, error) {
	if childRef.Kind != "TaskRun" {
		return nil, fmt.Errorf("could not fetch status for PipelineTask %s: should have kind TaskRun, but is %s", childRef.PipelineTaskName, childRef.Kind)
	}

	tr, err := client.TektonV1().TaskRuns(ns).Get(ctx, childRef.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	if tr == nil {
		return nil, nil //nolint:nilnil // would be more ergonomic to return a sentinel error
	}

	return &tr.Status, nil
}

// GetCustomRunStatusForPipelineTask takes a child reference and returns the actual CustomRunStatus for the
// PipelineTask. It returns an error if the child reference's kind isn't CustomRun.
func GetCustomRunStatusForPipelineTask(ctx context.Context, client versioned.Interface, ns string, childRef v1.ChildStatusReference) (*v1beta1.CustomRunStatus, error) {
	var runStatus *v1beta1.CustomRunStatus

	switch childRef.Kind {
	case "CustomRun":
		r, err := client.TektonV1beta1().CustomRuns(ns).Get(ctx, childRef.Name, metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return nil, err
		}
		if r == nil {
			return nil, nil //nolint:nilnil // would be more ergonomic to return a sentinel error
		}
		runStatus = &r.Status
	default:
		return nil, fmt.Errorf("could not fetch status for PipelineTask %s: should have kind CustomRun, but is %s", childRef.PipelineTaskName, childRef.Kind)
	}

	return runStatus, nil
}

// GetPipelineTaskStatuses returns populated TaskRun and Run status maps for a PipelineRun from its ChildReferences.
// If the PipelineRun has no ChildReferences, nothing will be populated.
func GetPipelineTaskStatuses(ctx context.Context, client versioned.Interface, ns string, pr *v1.PipelineRun) (map[string]*v1.PipelineRunTaskRunStatus,
	map[string]*v1.PipelineRunRunStatus, error) {
	// If the PipelineRun is nil, just return
	if pr == nil {
		return nil, nil, nil
	}

	// If there are no child references or either TaskRuns or Runs is non-zero, return the existing TaskRuns and Runs maps
	if len(pr.Status.ChildReferences) == 0 {
		return nil, nil, nil
	}

	trStatuses := make(map[string]*v1.PipelineRunTaskRunStatus)
	runStatuses := make(map[string]*v1.PipelineRunRunStatus)

	for _, cr := range pr.Status.ChildReferences {
		switch cr.Kind {
		case "TaskRun":
			tr, err := client.TektonV1().TaskRuns(ns).Get(ctx, cr.Name, metav1.GetOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return nil, nil, err
			}

			trStatuses[cr.Name] = &v1.PipelineRunTaskRunStatus{
				PipelineTaskName: cr.PipelineTaskName,
				WhenExpressions:  cr.WhenExpressions,
			}

			if tr != nil {
				trStatuses[cr.Name].Status = &tr.Status
			}
		case "CustomRun":
			r, err := client.TektonV1beta1().CustomRuns(ns).Get(ctx, cr.Name, metav1.GetOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return nil, nil, err
			}

			runStatuses[cr.Name] = &v1.PipelineRunRunStatus{
				PipelineTaskName: cr.PipelineTaskName,
				WhenExpressions:  cr.WhenExpressions,
			}

			if r != nil {
				runStatuses[cr.Name].Status = &r.Status
			}
		default:
			// Don't do anything for unknown types.
		}
	}

	return trStatuses, runStatuses, nil
}
