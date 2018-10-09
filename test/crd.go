/*
Copyright 2018 The Knative Authors

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

// crd contains defintions of resource instances which are useful across integration tests

package test

import (
	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
)

const (
	hwTaskName           = "helloworld"
	hwTaskRunName        = "helloworld-run"
	hwPipelineName       = "helloworld-pipeline"
	hwPipelineRunName    = "helloworld-pipelinerun"
	hwPipelineParamsName = "helloworld-pipelineparams"

	hwContainerName = "helloworld-busybox"
	taskOutput      = "do you want to build a snowman"
)

func getHelloWorldTask(namespace string) *v1alpha1.Task {
	return &v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      hwTaskName,
		},
		Spec: v1alpha1.TaskSpec{
			BuildSpec: &buildv1alpha1.BuildSpec{
				Steps: []corev1.Container{
					corev1.Container{
						Name:  hwContainerName,
						Image: "busybox",
						Args: []string{
							"echo", taskOutput,
						},
					},
				},
			},
		},
	}
}

func getHelloWorldTaskRun(namespace string) *v1alpha1.TaskRun {
	return &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      hwTaskRunName,
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: v1alpha1.TaskRef{
				Name: hwTaskName,
			},
			Trigger: v1alpha1.TaskTrigger{
				TriggerRef: v1alpha1.TaskTriggerRef{
					Type: v1alpha1.TaskTriggerTypeManual,
				},
			},
		},
	}
}

func getHelloWorldPipeline(namespace string) *v1alpha1.Pipeline {
	return &v1alpha1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      hwPipelineName,
		},
		Spec: v1alpha1.PipelineSpec{
			Tasks: []v1alpha1.PipelineTask{
				v1alpha1.PipelineTask{
					Name: "helloworld-task-1",
					TaskRef: v1alpha1.TaskRef{
						Name: hwTaskName,
					},
				},
				v1alpha1.PipelineTask{
					Name: "helloworld-task-2",
					TaskRef: v1alpha1.TaskRef{
						Name: hwTaskName,
					},
				},
				v1alpha1.PipelineTask{
					Name: "helloworld-task-3",
					TaskRef: v1alpha1.TaskRef{
						Name: hwTaskName,
					},
				},
			},
		},
	}
}
func getHelloWorldPipelineParams(namespace string) *v1alpha1.PipelineParams {
	return &v1alpha1.PipelineParams{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      hwPipelineParamsName,
		},
		Spec: v1alpha1.PipelineParamsSpec{},
	}
}

func getHelloWorldPipelineRun(namespace string) *v1alpha1.PipelineRun {
	return &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      hwPipelineRunName,
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{
				Name: hwPipelineName,
			},
			PipelineParamsRef: v1alpha1.PipelineParamsRef{
				Name: hwPipelineParamsName,
			},
			PipelineTriggerRef: v1alpha1.PipelineTriggerRef{
				Type: v1alpha1.PipelineTriggerTypeManual,
			},
		},
	}
}
