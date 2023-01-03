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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	runv1beta1 "github.com/tektoncd/pipeline/pkg/apis/run/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetTaskRunStatusForPipelineTask takes a minimal embedded status child reference and returns the actual TaskRunStatus
// for the PipelineTask. It returns an error if the child reference's kind isn't TaskRun.
func GetTaskRunStatusForPipelineTask(ctx context.Context, client versioned.Interface, ns string, childRef v1beta1.ChildStatusReference) (*v1beta1.TaskRunStatus, error) {
	if childRef.Kind != "TaskRun" {
		return nil, fmt.Errorf("could not fetch status for PipelineTask %s: should have kind TaskRun, but is %s", childRef.PipelineTaskName, childRef.Kind)
	}

	tr, err := client.TektonV1beta1().TaskRuns(ns).Get(ctx, childRef.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	if tr == nil {
		return nil, nil
	}

	return &tr.Status, nil
}

// GetRunStatusForPipelineTask takes a minimal embedded status child reference and returns the actual CustomRunStatus for the
// PipelineTask. It returns an error if the child reference's kind isn't CustomRun.
func GetRunStatusForPipelineTask(ctx context.Context, client versioned.Interface, ns string, childRef v1beta1.ChildStatusReference) (*v1beta1.CustomRunStatus, error) {
	var runStatus *v1beta1.CustomRunStatus

	switch childRef.Kind {
	case "CustomRun":
		r, err := client.TektonV1beta1().CustomRuns(ns).Get(ctx, childRef.Name, metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return nil, err
		}
		if r == nil {
			return nil, nil
		}
		runStatus = &r.Status
	case "Run":
		r, err := client.TektonV1alpha1().Runs(ns).Get(ctx, childRef.Name, metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return nil, err
		}
		if r == nil {
			return nil, nil
		}
		asCustomRunStatus := runv1beta1.FromRunStatus(r.Status)
		runStatus = &asCustomRunStatus
	default:
		return nil, fmt.Errorf("could not fetch status for PipelineTask %s: should have kind Run or CustomRun, but is %s", childRef.PipelineTaskName, childRef.Kind)
	}

	return runStatus, nil
}

// GetFullPipelineTaskStatuses returns populated TaskRun and Run status maps for a PipelineRun from its ChildReferences.
// If the PipelineRun has no ChildReferences, its .Status.TaskRuns and .Status.Runs will be returned instead.
func GetFullPipelineTaskStatuses(ctx context.Context, client versioned.Interface, ns string, pr *v1beta1.PipelineRun) (map[string]*v1beta1.PipelineRunTaskRunStatus,
	map[string]*v1beta1.PipelineRunRunStatus, error) {
	// If the PipelineRun is nil, just return
	if pr == nil {
		return nil, nil, nil
	}

	// If there are no child references or either TaskRuns or Runs is non-zero, return the existing TaskRuns and Runs maps
	if len(pr.Status.ChildReferences) == 0 || len(pr.Status.TaskRuns) > 0 || len(pr.Status.Runs) > 0 {
		return pr.Status.TaskRuns, pr.Status.Runs, nil
	}

	trStatuses := make(map[string]*v1beta1.PipelineRunTaskRunStatus)
	runStatuses := make(map[string]*v1beta1.PipelineRunRunStatus)

	for _, cr := range pr.Status.ChildReferences {
		switch cr.Kind {
		case "TaskRun":
			tr, err := client.TektonV1beta1().TaskRuns(ns).Get(ctx, cr.Name, metav1.GetOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return nil, nil, err
			}

			trStatuses[cr.Name] = &v1beta1.PipelineRunTaskRunStatus{
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

			runStatuses[cr.Name] = &v1beta1.PipelineRunRunStatus{
				PipelineTaskName: cr.PipelineTaskName,
				WhenExpressions:  cr.WhenExpressions,
			}

			if r != nil {
				runStatuses[cr.Name].Status = &r.Status
			}
		case "Run":
			r, err := client.TektonV1alpha1().Runs(ns).Get(ctx, cr.Name, metav1.GetOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return nil, nil, err
			}

			runStatuses[cr.Name] = &v1beta1.PipelineRunRunStatus{
				PipelineTaskName: cr.PipelineTaskName,
				WhenExpressions:  cr.WhenExpressions,
			}

			if r != nil {
				asCustomRunStatus := runv1beta1.FromRunStatus(r.Status)
				runStatuses[cr.Name].Status = &asCustomRunStatus
			}
		default:
			// Don't do anything for unknown types.
		}
	}

	return trStatuses, runStatuses, nil
}
