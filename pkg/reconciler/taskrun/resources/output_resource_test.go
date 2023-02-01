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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/resource"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakek8s "k8s.io/client-go/kubernetes/fake"
)

var (
	outputTestResources map[string]v1beta1.PipelineResourceInterface
	images              = pipeline.Images{
		EntrypointImage:          "override-with-entrypoint:latest",
		NopImage:                 "override-with-nop:latest",
		GitImage:                 "override-with-git:latest",
		ShellImage:               "busybox",
		GsutilImage:              "gcr.io/google.com/cloudsdktool/cloud-sdk",
		ImageDigestExporterImage: "override-with-imagedigest-exporter-image:latest",
	}
)

func outputTestResourceSetup() {
	rs := []*resourcev1alpha1.PipelineResource{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-image",
			Namespace: "marshmallow",
		},
		Spec: resourcev1alpha1.PipelineResourceSpec{
			Type: "image",
		},
	}}

	outputTestResources = make(map[string]v1beta1.PipelineResourceInterface)
	for _, r := range rs {
		ri, _ := resource.FromType(r.Name, r, images)
		outputTestResources[r.Name] = ri
	}
}

func TestValidOutputResources(t *testing.T) {
	for _, c := range []struct {
		name        string
		desc        string
		task        *v1beta1.Task
		taskRun     *v1beta1.TaskRun
		wantVolumes []corev1.Volume
	}{{
		name: "image resource in output with pipelinerun with owner",
		desc: "image resource declared as output with pipelinerun owner reference should not generate any steps",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-image",
							},
						},
						Paths: []string{"pipeline-task-name"},
					}},
				},
			},
		},
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "image",
						}}},
				},
			},
		},
		wantVolumes: nil,
	}, {
		name: "image resource as output",
		desc: "image resource defined only in output",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-only-output-step",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-image",
							},
						},
					}},
				},
			},
		},
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "image",
						}}},
				},
			},
		},
	}, {
		desc: "multiple image output resource with no steps",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-image",
							},
						},
					}, {
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "source-workspace-1",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "source-image",
							},
						},
					}},
				},
			},
		},
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "image",
						}}, {
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace-1",
							Type: "image",
						}}},
				},
			},
		},
	}} {
		t.Run(c.name, func(t *testing.T) {
			names.TestingSeed()
			outputTestResourceSetup()
			fakekubeclient := fakek8s.NewSimpleClientset()
			got, err := AddOutputResources(context.Background(), fakekubeclient, images, c.task.Name, &c.task.Spec, c.taskRun, resolveOutputResources(c.taskRun))
			if err != nil {
				t.Fatalf("Failed to declare output resources for test name %q ; test description %q: error %v", c.name, c.desc, err)
			}

			if got != nil {
				if d := cmp.Diff(c.wantVolumes, got.Volumes); d != "" {
					t.Fatalf("post build steps volumes mismatch %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestInvalidOutputResources(t *testing.T) {
	for _, c := range []struct {
		desc    string
		task    *v1beta1.Task
		taskRun *v1beta1.TaskRun
		wantErr bool
	}{{
		desc: "no outputs defined",
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{},
		},
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-only-output-step",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
		},
		wantErr: false,
	}, {
		desc: "no outputs defined in taskrun but defined in task",
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "source-workspace",
							Type: "image",
						}}},
				},
			},
		},
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-only-output-step",
				Namespace: "foo",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
		},
		wantErr: true,
	}, {
		desc: "optional outputs declared",
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{{ResourceDeclaration: v1beta1.ResourceDeclaration{
						Name:     "source-workspace",
						Type:     "git",
						Optional: true,
					}}},
				},
			},
		},
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-optional-output",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
		},
		wantErr: false,
	}, {
		desc: "required outputs declared",
		task: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{{ResourceDeclaration: v1beta1.ResourceDeclaration{
						Name:     "source-workspace",
						Type:     "git",
						Optional: false,
					}}},
				},
			},
		},
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-required-output",
				Namespace: "marshmallow",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "pipelinerun",
				}},
			},
		},
		wantErr: true,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			outputTestResourceSetup()
			fakekubeclient := fakek8s.NewSimpleClientset()
			_, err := AddOutputResources(context.Background(), fakekubeclient, images, c.task.Name, &c.task.Spec, c.taskRun, resolveOutputResources(c.taskRun))
			if (err != nil) != c.wantErr {
				t.Fatalf("Test AddOutputResourceSteps %v : error%v", c.desc, err)
			}
		})
	}
}

func resolveOutputResources(taskRun *v1beta1.TaskRun) map[string]v1beta1.PipelineResourceInterface {
	resolved := make(map[string]v1beta1.PipelineResourceInterface)
	if taskRun.Spec.Resources == nil {
		return resolved
	}
	for _, r := range taskRun.Spec.Resources.Outputs {
		var i v1beta1.PipelineResourceInterface
		if name := r.ResourceRef.Name; name != "" {
			i = outputTestResources[name]
			resolved[r.Name] = i
		} else if r.ResourceSpec != nil {
			i, _ = resource.FromType(r.Name, &resourcev1alpha1.PipelineResource{
				ObjectMeta: metav1.ObjectMeta{
					Name: r.Name,
				},
				Spec: *r.ResourceSpec,
			}, images)
			resolved[r.Name] = i
		}
	}
	return resolved
}
