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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CopyTaskMetaToTaskRun implements the default Tekton Pipelines behaviour
// for copying meta information to a TaskRun from the Task it references.
//
// The labels and annotations from the task will all be copied to the taskrun
// verbatim. This matches the expectations/behaviour of the current TaskRun
// reconciler.
func CopyTaskMetaToTaskRun(taskMeta *metav1.ObjectMeta, tr *v1beta1.TaskRun) {
	needsLabels := len(taskMeta.Labels) > 0 || tr.Spec.TaskRef != nil
	if tr.ObjectMeta.Labels == nil && needsLabels {
		tr.ObjectMeta.Labels = map[string]string{}
	}
	for key, value := range taskMeta.Labels {
		tr.ObjectMeta.Labels[key] = value
	}

	if tr.ObjectMeta.Annotations == nil {
		tr.ObjectMeta.Annotations = make(map[string]string, len(taskMeta.Annotations))
	}
	for key, value := range taskMeta.Annotations {
		tr.ObjectMeta.Annotations[key] = value
	}

	if tr.Spec.TaskRef != nil {
		if tr.Spec.TaskRef.Kind == "ClusterTask" {
			tr.ObjectMeta.Labels[pipeline.ClusterTaskLabelKey] = taskMeta.Name
		} else {
			tr.ObjectMeta.Labels[pipeline.TaskLabelKey] = taskMeta.Name
		}
	}
}

// CopyPipelineMetaToPipelineRun implements the default Tekton Pipelines
// behaviour for copying meta information to a PipelineRun from the Pipeline it
// references.
//
// The labels and annotations from the pipeline will all be copied to the
// pipielinerun verbatim. This matches the expectations/behaviour of the
// current PipelineRun reconciler.
func CopyPipelineMetaToPipelineRun(pipelineMeta *metav1.ObjectMeta, pr *v1beta1.PipelineRun) {
	if pr.ObjectMeta.Labels == nil {
		pr.ObjectMeta.Labels = make(map[string]string, len(pipelineMeta.Labels)+1)
	}
	for key, value := range pipelineMeta.Labels {
		pr.ObjectMeta.Labels[key] = value
	}
	pr.ObjectMeta.Labels[pipeline.PipelineLabelKey] = pipelineMeta.Name

	if pr.ObjectMeta.Annotations == nil {
		pr.ObjectMeta.Annotations = make(map[string]string, len(pipelineMeta.Annotations))
	}
	for key, value := range pipelineMeta.Annotations {
		pr.ObjectMeta.Annotations[key] = value
	}
}
