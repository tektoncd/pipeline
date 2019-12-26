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

package resources

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddOutputImageDigestExporter(t *testing.T) {
	for _, c := range []struct {
		desc      string
		task      *v1alpha1.Task
		taskRun   *v1alpha1.TaskRun
		wantSteps []v1alpha1.Step
	}{{
		desc: "image resource declared as both input and output",
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Inputs: &v1alpha1.Inputs{
					Resources: []v1alpha1.TaskResource{{
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "source-image",
							Type: "image",
						}}},
				},
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "source-image",
							Type: "image",
						},
					}},
				},
				Steps: []v1alpha1.Step{{Container: corev1.Container{
					Name: "step1",
				}}},
			},
		},
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				Inputs: v1alpha1.TaskRunInputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-image",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "source-image-1",
							},
						},
					}},
				},
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-image",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "source-image-1",
							},
						},
					}},
				},
			},
		},
		wantSteps: []v1alpha1.Step{{Container: corev1.Container{
			Name: "step1",
		}}, {Container: corev1.Container{
			Name:                     "image-digest-exporter-9l9zj",
			Image:                    "override-with-imagedigest-exporter-image:latest",
			Command:                  []string{"/ko-app/imagedigestexporter"},
			Args:                     []string{"-images", "[{\"name\":\"source-image-1\",\"type\":\"image\",\"url\":\"gcr.io/some-image-1\",\"digest\":\"\",\"OutputImageDir\":\"/workspace/output/source-image\"}]"},
			TerminationMessagePolicy: "FallbackToLogsOnError",
		}}},
	}, {
		desc: "image resource in task with multiple steps",
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Inputs: &v1alpha1.Inputs{
					Resources: []v1alpha1.TaskResource{{
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "source-image",
							Type: "image",
						}}},
				},
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						ResourceDeclaration: v1alpha1.ResourceDeclaration{
							Name: "source-image",
							Type: "image",
						},
					}},
				},
				Steps: []v1alpha1.Step{{Container: corev1.Container{
					Name: "step1",
				}}, {Container: corev1.Container{
					Name: "step2",
				}}},
			},
		},
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				Inputs: v1alpha1.TaskRunInputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-image",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "source-image-1",
							},
						},
					}},
				},
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
							Name: "source-image",
							ResourceRef: &v1alpha1.PipelineResourceRef{
								Name: "source-image-1",
							},
						},
					}},
				},
			},
		},
		wantSteps: []v1alpha1.Step{{Container: corev1.Container{
			Name: "step1",
		}}, {Container: corev1.Container{
			Name: "step2",
		}}, {Container: corev1.Container{
			Name:    "image-digest-exporter-9l9zj",
			Image:   "override-with-imagedigest-exporter-image:latest",
			Command: []string{"/ko-app/imagedigestexporter"},
			Args:    []string{"-images", "[{\"name\":\"source-image-1\",\"type\":\"image\",\"url\":\"gcr.io/some-image-1\",\"digest\":\"\",\"OutputImageDir\":\"/workspace/output/source-image\"}]"},

			TerminationMessagePolicy: "FallbackToLogsOnError",
		}}},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			names.TestingSeed()
			gr := func(n string) (*v1alpha1.PipelineResource, error) {
				return &v1alpha1.PipelineResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "source-image-1",
						Namespace: "marshmallow",
					},
					Spec: v1alpha1.PipelineResourceSpec{
						Type: "image",
						Params: []v1alpha1.ResourceParam{{
							Name:  "url",
							Value: "gcr.io/some-image-1",
						}, {
							Name:  "digest",
							Value: "",
						}, {
							Name:  "OutputImageDir",
							Value: "/workspace/source-image-1/index.json",
						}},
					},
				}, nil
			}
			err := AddOutputImageDigestExporter("override-with-imagedigest-exporter-image:latest", c.taskRun, &c.task.Spec, gr)
			if err != nil {
				t.Fatalf("Failed to declare output resources for test %q: error %v", c.desc, err)
			}

			if d := cmp.Diff(c.task.Spec.Steps, c.wantSteps); d != "" {
				t.Fatalf("post build steps mismatch: %s", d)
			}
		})
	}
}
