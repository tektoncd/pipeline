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
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/apis"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddOutputImageDigestExporter(t *testing.T) {
	currentDir, _ := os.Getwd()
	for _, c := range []struct {
		desc      string
		task      *v1alpha1.Task
		taskRun   *v1alpha1.TaskRun
		wantSteps []corev1.Container
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
						Name: "source-image",
						Type: "image",
					}},
				},
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name:           "source-image",
						Type:           "image",
						OutputImageDir: currentDir,
					}},
				},
				Steps: []corev1.Container{{
					Name: "step1",
				},
				},
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
						Name: "source-image",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-image-1",
						},
					}},
				},
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-image",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-image-1",
						},
					}},
				},
			},
		},
		wantSteps: []corev1.Container{
			{
				Name: "step1",
			},
			{
				Name:                     "image-digest-exporter-step1-9l9zj",
				Image:                    "override-with-imagedigest-exporter-image:latest",
				Command:                  []string{"/ko-app/imagedigestexporter"},
				Args:                     []string{"-images", fmt.Sprintf("[{\"name\":\"source-image-1\",\"type\":\"image\",\"url\":\"gcr.io/some-image-1\",\"digest\":\"\",\"OutputImageDir\":\"%s\"}]", currentDir),
												   "-terminationMessagePath", "/builder/home/image-outputs/termination-log"},
				TerminationMessagePath:   TerminationMessagePath,
				TerminationMessagePolicy: "FallbackToLogsOnError",
			}},
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
						Name: "source-image",
						Type: "image",
					}},
				},
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name:           "source-image",
						Type:           "image",
						OutputImageDir: currentDir,
					}},
				},
				Steps: []corev1.Container{{
					Name: "step1",
				}, {
					Name: "step2",
				}},
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
						Name: "source-image",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-image-1",
						},
					}},
				},
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-image",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-image-1",
						},
					}},
				},
			},
		},
		wantSteps: []corev1.Container{
			{
				Name: "step1",
			},
			{
				Name:                     "image-digest-exporter-step1-9l9zj",
				Image:                    "override-with-imagedigest-exporter-image:latest",
				Command:                  []string{"/ko-app/imagedigestexporter"},
				Args:                     []string{"-images", fmt.Sprintf("[{\"name\":\"source-image-1\",\"type\":\"image\",\"url\":\"gcr.io/some-image-1\",\"digest\":\"\",\"OutputImageDir\":\"%s\"}]", currentDir),
													"-terminationMessagePath", "/builder/home/image-outputs/termination-log"},
				TerminationMessagePath:   TerminationMessagePath,
				TerminationMessagePolicy: "FallbackToLogsOnError",
			}, {
				Name: "step2",
			}, {
				Name:                     "image-digest-exporter-step2-mz4c7",
				Image:                    "override-with-imagedigest-exporter-image:latest",
				Command:                  []string{"/ko-app/imagedigestexporter"},
				Args:                     []string{"-images", fmt.Sprintf("[{\"name\":\"source-image-1\",\"type\":\"image\",\"url\":\"gcr.io/some-image-1\",\"digest\":\"\",\"OutputImageDir\":\"%s\"}]", currentDir),
													"-terminationMessagePath", "/builder/home/image-outputs/termination-log"},
				TerminationMessagePath:   TerminationMessagePath,
				TerminationMessagePolicy: "FallbackToLogsOnError",
			},
		},
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
						Params: []v1alpha1.Param{{
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
			err := AddOutputImageDigestExporter(c.taskRun, &c.task.Spec, gr)
			if err != nil {
				t.Fatalf("Failed to declare output resources for test %q: error %v", c.desc, err)
			}

			if d := cmp.Diff(c.task.Spec.Steps, c.wantSteps); d != "" {
				t.Fatalf("post build steps mismatch: %s", d)
			}
		})
	}
}

func TestUpdateTaskRunStatus_withValidJson(t *testing.T) {
	for _, c := range []struct {
		desc    string
		podLog  []byte
		taskRun *v1alpha1.TaskRun
		want    []v1alpha1.PipelineResourceResult
	}{{
		desc:   "image resource updated",
		podLog: []byte("[{\"name\":\"source-image\",\"digest\":\"sha256:1234\"}]"),
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				Inputs: v1alpha1.TaskRunInputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-image",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-image-1",
						},
					}},
				},
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-image",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-image-1",
						},
					}},
				},
			},
		},
		want: []v1alpha1.PipelineResourceResult{{
			Name:   "source-image",
			Digest: "sha256:1234",
		}},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			names.TestingSeed()
			c.taskRun.Status.SetCondition(&apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			})
			err := UpdateTaskRunStatusWithResourceResult(c.taskRun, c.podLog)
			if err != nil {
				t.Errorf("UpdateTaskRunStatusWithResourceResult failed with error: %s", err)
			}
			if d := cmp.Diff(c.taskRun.Status.ResourcesResult, c.want); d != "" {
				t.Errorf("post build steps mismatch: %s", d)
			}
		})
	}
}

func TestUpdateTaskRunStatus_withInvalidJson(t *testing.T) {
	for _, c := range []struct {
		desc    string
		podLog  []byte
		taskRun *v1alpha1.TaskRun
		want    []v1alpha1.PipelineResourceResult
	}{{
		desc:   "image resource exporter with malformed json output",
		podLog: []byte("extralogscamehere[{\"name\":\"source-image\",\"digest\":\"sha256:1234\"}]"),
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				Inputs: v1alpha1.TaskRunInputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-image",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-image-1",
						},
					}},
				},
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-image",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-image-1",
						},
					}},
				},
			},
		},
		want: nil,
	}, {
		desc:   "task with no image resource ",
		podLog: []byte(""),
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
			},
		},
		want: nil,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			names.TestingSeed()
			c.taskRun.Status.SetCondition(&apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			})
			err := UpdateTaskRunStatusWithResourceResult(c.taskRun, c.podLog)
			if err == nil {
				t.Error("UpdateTaskRunStatusWithResourceResult expected to fail with error")
			}
			if d := cmp.Diff(c.taskRun.Status.ResourcesResult, c.want); d != "" {
				t.Errorf("post build steps mismatch: %s", d)
			}
		})
	}
}

func TestTaskRunHasOutputImageResource(t *testing.T) {
	for _, c := range []struct {
		desc       string
		task       *v1alpha1.Task
		taskRun    *v1alpha1.TaskRun
		wantResult bool
	}{{
		desc: "image resource as output",
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Inputs: &v1alpha1.Inputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-image",
						Type: "image",
					}},
				},
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-image",
						Type: "image",
					}},
				},
				Steps: []corev1.Container{{
					Name: "step1",
				}},
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
						Name: "source-image",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-image-1",
						},
					}},
				},
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-image",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-image-1",
						},
					}},
				},
			},
		},
		wantResult: true,
	}, {
		desc: "task with no image resource",
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Inputs: &v1alpha1.Inputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source",
						Type: "git",
					}},
				},
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source",
						Type: "git",
					}},
				},
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
						Name: "source",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-git-1",
						},
					}},
				},
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-git-1",
						},
					}},
				},
			},
		},
		wantResult: false,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			gr := func(name string) (*v1alpha1.PipelineResource, error) {
				var r *v1alpha1.PipelineResource
				if name == "source-image-1" {
					r = &v1alpha1.PipelineResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "source-image-1",
							Namespace: "marshmallow",
						},
						Spec: v1alpha1.PipelineResourceSpec{
							Type: "image",
							Params: []v1alpha1.Param{{
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
					}
				} else if name == "source-git-1" {
					r = &v1alpha1.PipelineResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "source-git-1",
							Namespace: "marshmallow",
						},
						Spec: v1alpha1.PipelineResourceSpec{
							Type: "git",
							Params: []v1alpha1.Param{{
								Name:  "url",
								Value: "github.com/repo",
							},
							},
						},
					}
				}
				return r, nil
			}
			result := TaskRunHasOutputImageResource(gr, c.taskRun)

			if d := cmp.Diff(result, c.wantResult); d != "" {
				t.Fatalf("post build steps mismatch: %s", d)
			}
		})
	}
}
