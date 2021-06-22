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

package resolution

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	resources "github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type TaskRunResolutionRequest struct {
	KubeClientSet     kubernetes.Interface
	PipelineClientSet clientset.Interface
	TaskRun           *v1beta1.TaskRun

	ResolvedTaskMeta *metav1.ObjectMeta
	ResolvedTaskSpec *v1beta1.TaskSpec
}

// TaskRunResolutionRequest.Resolve implements the default resolution behaviour
// for a Tekton Pipelines taskrun. It resolves the task associated with the taskrun
// from one of three places:
//
// - in-line in the taskrun's spec.taskSpec
// - from the cluster via the taskrun's spec.taskRef
// - (when relevant feature flag enabled) from a Tekton Bundle indicated by the taskrun's spec.taskRef.bundle field.
//
// If a task is resolved correctly from the taskrun then the ResolvedTaskMeta
// and ResolvedTaskSpec fields of the TaskRunResolutionRequest will be
// populated after this method returns.
//
// If an error occurs during any part in the resolution process it will be
// returned with both a human-readable Message and a machine-readable Reason
// embedded in a *resolution.Error.
func (req *TaskRunResolutionRequest) Resolve(ctx context.Context) error {
	if req.TaskRun.Status.TaskSpec != nil {
		return ErrorTaskRunAlreadyResolved
	}

	getTaskFunc, err := resources.GetTaskFuncFromTaskRun(ctx, req.KubeClientSet, req.PipelineClientSet, req.TaskRun)
	if err != nil {
		return NewError(ReasonTaskRunResolutionFailed, err)
	}

	taskMeta, taskSpec, err := resources.GetTaskData(ctx, req.TaskRun, getTaskFunc)
	if err != nil {
		return NewError(ReasonTaskRunResolutionFailed, err)
	}

	req.ResolvedTaskMeta = taskMeta
	req.ResolvedTaskSpec = taskSpec

	return nil
}
