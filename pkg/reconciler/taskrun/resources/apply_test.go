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

package resources_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"github.com/tektoncd/pipeline/test/builder"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	images = pipeline.Images{
		EntrypointImage:          "override-with-entrypoint:latest",
		NopImage:                 "tianon/true",
		GitImage:                 "override-with-git:latest",
		CredsImage:               "override-with-creds:latest",
		KubeconfigWriterImage:    "override-with-kubeconfig-writer-image:latest",
		ShellImage:               "busybox",
		GsutilImage:              "google/cloud-sdk",
		BuildGCSFetcherImage:     "gcr.io/cloud-builders/gcs-fetcher:latest",
		PRImage:                  "override-with-pr:latest",
		ImageDigestExporterImage: "override-with-imagedigest-exporter-image:latest",
	}

	simpleTaskSpec = &v1alpha1.TaskSpec{
		Inputs: &v1alpha1.Inputs{
			Resources: []v1alpha1.TaskResource{{
				ResourceDeclaration: v1alpha1.ResourceDeclaration{
					Name: "workspace",
				},
			}},
		},
		Outputs: &v1alpha1.Outputs{
			Resources: []v1alpha1.TaskResource{{
				ResourceDeclaration: v1alpha1.ResourceDeclaration{
					Name:       "imageToUse-ab",
					TargetPath: "/foo/builtImage",
				},
			}, {
				ResourceDeclaration: v1alpha1.ResourceDeclaration{
					Name:       "imageToUse-re",
					TargetPath: "foo/builtImage",
				},
			}},
		},
		Sidecars: []corev1.Container{{
			Name:  "foo",
			Image: "$(inputs.params.myimage)",
			Env: []corev1.EnvVar{{
				Name:  "foo",
				Value: "$(inputs.params.FOO)",
			}},
		}},
		StepTemplate: &corev1.Container{
			Env: []corev1.EnvVar{{
				Name:  "template-var",
				Value: "$(inputs.params.FOO)",
			}},
		},
		Steps: []v1alpha1.Step{{Container: corev1.Container{
			Name:  "foo",
			Image: "$(inputs.params.myimage)",
		}}, {Container: corev1.Container{
			Name:       "baz",
			Image:      "bat",
			WorkingDir: "$(inputs.resources.workspace.path)",
			Args:       []string{"$(inputs.resources.workspace.url)"},
		}}, {Container: corev1.Container{
			Name:  "qux",
			Image: "$(inputs.params.something)",
			Args:  []string{"$(outputs.resources.imageToUse.url)"},
		}}, {Container: corev1.Container{
			Name:  "foo",
			Image: "$(inputs.params.myimage)",
		}}, {Container: corev1.Container{
			Name:       "baz",
			Image:      "$(inputs.params.somethingelse)",
			WorkingDir: "$(inputs.resources.workspace.path)",
			Args:       []string{"$(inputs.resources.workspace.url)"},
		}}, {Container: corev1.Container{
			Name:  "qux",
			Image: "quux",
			Args:  []string{"$(outputs.resources.imageToUse.url)"},
		}}, {Container: corev1.Container{
			Name:  "foo",
			Image: "busybox:$(inputs.params.FOO)",
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "$(inputs.params.FOO)",
				MountPath: "path/to/$(inputs.params.FOO)",
				SubPath:   "sub/$(inputs.params.FOO)/path",
			}},
		}}, {Container: corev1.Container{
			Name:  "foo",
			Image: "busybox:$(inputs.params.FOO)",
			Env: []corev1.EnvVar{{
				Name:  "foo",
				Value: "value-$(inputs.params.FOO)",
			}, {
				Name: "bar",
				ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "config-$(inputs.params.FOO)"},
						Key:                  "config-key-$(inputs.params.FOO)",
					},
				},
			}, {
				Name: "baz",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "secret-$(inputs.params.FOO)"},
						Key:                  "secret-key-$(inputs.params.FOO)",
					},
				},
			}},
			EnvFrom: []corev1.EnvFromSource{{
				Prefix: "prefix-0-$(inputs.params.FOO)",
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "config-$(inputs.params.FOO)"},
				},
			}, {
				Prefix: "prefix-1-$(inputs.params.FOO)",
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "secret-$(inputs.params.FOO)"},
				},
			}},
		}}, {Container: corev1.Container{
			Name:  "outputs-resources-path-ab",
			Image: "$(outputs.resources.imageToUse-ab.path)",
		}}, {Container: corev1.Container{
			Name:  "outputs-resources-path-re",
			Image: "$(outputs.resources.imageToUse-re.path)",
		}}},
		Volumes: []corev1.Volume{{
			Name: "$(inputs.params.FOO)",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "$(inputs.params.FOO)",
					},
				},
			},
		}, {
			Name: "some-secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "$(inputs.params.FOO)",
				},
			},
		}, {
			Name: "some-pvc",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "$(inputs.params.FOO)",
				},
			},
		}},
	}

	gcsTaskSpec = &v1alpha1.TaskSpec{
		Outputs: &v1alpha1.Outputs{
			Resources: []v1alpha1.TaskResource{{
				ResourceDeclaration: v1alpha1.ResourceDeclaration{
					Name: "bucket",
				},
			}},
		},
		Steps: []v1alpha1.Step{{Container: corev1.Container{
			Name:  "foobar",
			Image: "someImage",
			Args:  []string{"$(outputs.resources.bucket.path)"},
		}}},
	}

	arrayParamTaskSpec = &v1alpha1.TaskSpec{
		Steps: []v1alpha1.Step{{Container: corev1.Container{
			Name:  "simple-image",
			Image: "some-image",
		}}, {Container: corev1.Container{
			Name:    "image-with-c-specified",
			Image:   "some-other-image",
			Command: []string{"echo"},
			Args:    []string{"first", "second", "$(inputs.params.array-param)", "last"},
		}}},
	}

	arrayAndStringParamTaskSpec = &v1alpha1.TaskSpec{
		Steps: []v1alpha1.Step{{Container: corev1.Container{
			Name:  "simple-image",
			Image: "some-image",
		}}, {Container: corev1.Container{
			Name:    "image-with-c-specified",
			Image:   "some-other-image",
			Command: []string{"echo"},
			Args:    []string{"$(inputs.params.normal-param)", "second", "$(inputs.params.array-param)", "last"},
		}}},
	}

	multipleArrayParamsTaskSpec = &v1alpha1.TaskSpec{
		Steps: []v1alpha1.Step{{Container: corev1.Container{
			Name:  "simple-image",
			Image: "some-image",
		}}, {Container: corev1.Container{
			Name:    "image-with-c-specified",
			Image:   "some-other-image",
			Command: []string{"cmd", "$(inputs.params.another-array-param)"},
			Args:    []string{"first", "second", "$(inputs.params.array-param)", "last"},
		}}},
	}

	multipleArrayAndStringsParamsTaskSpec = &v1alpha1.TaskSpec{
		Steps: []v1alpha1.Step{{Container: corev1.Container{
			Name:  "simple-image",
			Image: "image-$(inputs.params.string-param2)",
		}}, {Container: corev1.Container{
			Name:    "image-with-c-specified",
			Image:   "some-other-image",
			Command: []string{"cmd", "$(inputs.params.array-param1)"},
			Args:    []string{"$(inputs.params.array-param2)", "second", "$(inputs.params.array-param1)", "$(inputs.params.string-param1)", "last"},
		}}},
	}

	arrayTaskRun0Elements = &v1alpha1.TaskRun{
		Spec: v1alpha1.TaskRunSpec{
			Inputs: v1alpha1.TaskRunInputs{
				Params: []v1alpha1.Param{{
					Name: "array-param",
					Value: v1alpha1.ArrayOrString{
						Type:     v1alpha1.ParamTypeArray,
						ArrayVal: []string{},
					}},
				},
			},
		},
	}

	arrayTaskRun1Elements = &v1alpha1.TaskRun{
		Spec: v1alpha1.TaskRunSpec{
			Inputs: v1alpha1.TaskRunInputs{
				Params: []v1alpha1.Param{{
					Name:  "array-param",
					Value: *builder.ArrayOrString("foo"),
				}},
			},
		},
	}

	arrayTaskRun3Elements = &v1alpha1.TaskRun{
		Spec: v1alpha1.TaskRunSpec{
			Inputs: v1alpha1.TaskRunInputs{
				Params: []v1alpha1.Param{{
					Name:  "array-param",
					Value: *builder.ArrayOrString("foo", "bar", "third"),
				}},
			},
		},
	}

	arrayTaskRunMultipleArrays = &v1alpha1.TaskRun{
		Spec: v1alpha1.TaskRunSpec{
			Inputs: v1alpha1.TaskRunInputs{
				Params: []v1alpha1.Param{{
					Name:  "array-param",
					Value: *builder.ArrayOrString("foo", "bar", "third"),
				}, {
					Name:  "another-array-param",
					Value: *builder.ArrayOrString("part1", "part2"),
				}},
			},
		},
	}

	arrayTaskRunWith1StringParam = &v1alpha1.TaskRun{
		Spec: v1alpha1.TaskRunSpec{
			Inputs: v1alpha1.TaskRunInputs{
				Params: []v1alpha1.Param{{
					Name:  "array-param",
					Value: *builder.ArrayOrString("middlefirst", "middlesecond"),
				}, {
					Name:  "normal-param",
					Value: *builder.ArrayOrString("foo"),
				}},
			},
		},
	}

	arrayTaskRunMultipleArraysAndStrings = &v1alpha1.TaskRun{
		Spec: v1alpha1.TaskRunSpec{
			Inputs: v1alpha1.TaskRunInputs{
				Params: []v1alpha1.Param{{
					Name:  "array-param1",
					Value: *builder.ArrayOrString("1-param1", "2-param1", "3-param1", "4-param1"),
				}, {
					Name:  "array-param2",
					Value: *builder.ArrayOrString("1-param2", "2-param2", "2-param3"),
				}, {
					Name:  "string-param1",
					Value: *builder.ArrayOrString("foo"),
				}, {
					Name:  "string-param2",
					Value: *builder.ArrayOrString("bar"),
				}},
			},
		},
	}

	inputs = map[string]v1alpha1.PipelineResourceInterface{
		"workspace": gitResource,
	}

	outputs = map[string]v1alpha1.PipelineResourceInterface{
		"imageToUse": imageResource,
		"bucket":     gcsResource,
	}

	gitResource, _ = v1alpha1.ResourceFromType(&v1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "git-resource",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: v1alpha1.PipelineResourceTypeGit,
			Params: []v1alpha1.ResourceParam{{
				Name:  "URL",
				Value: "https://git-repo",
			}},
		},
	}, images)

	imageResource, _ = v1alpha1.ResourceFromType(&v1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image-resource",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: v1alpha1.PipelineResourceTypeImage,
			Params: []v1alpha1.ResourceParam{{
				Name:  "URL",
				Value: "gcr.io/hans/sandwiches",
			}},
		},
	}, images)

	gcsResource, _ = v1alpha1.ResourceFromType(&v1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gcs-resource",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: v1alpha1.PipelineResourceTypeStorage,
			Params: []v1alpha1.ResourceParam{{
				Name:  "type",
				Value: "gcs",
			}, {
				Name:  "location",
				Value: "theCloud?",
			}},
		},
	}, images)
)

func applyMutation(ts *v1alpha1.TaskSpec, f func(*v1alpha1.TaskSpec)) *v1alpha1.TaskSpec {
	ts = ts.DeepCopy()
	f(ts)
	return ts
}

func TestApplyArrayParameters(t *testing.T) {
	type args struct {
		ts *v1alpha1.TaskSpec
		tr *v1alpha1.TaskRun
		dp []v1alpha1.ParamSpec
	}
	tests := []struct {
		name string
		args args
		want *v1alpha1.TaskSpec
	}{{
		name: "array parameter with 0 elements",
		args: args{
			ts: arrayParamTaskSpec,
			tr: arrayTaskRun0Elements,
		},
		want: applyMutation(arrayParamTaskSpec, func(spec *v1alpha1.TaskSpec) {
			spec.Steps[1].Args = []string{"first", "second", "last"}
		}),
	}, {
		name: "array parameter with 1 element",
		args: args{
			ts: arrayParamTaskSpec,
			tr: arrayTaskRun1Elements,
		},
		want: applyMutation(arrayParamTaskSpec, func(spec *v1alpha1.TaskSpec) {
			spec.Steps[1].Args = []string{"first", "second", "foo", "last"}
		}),
	}, {
		name: "array parameter with 3 elements",
		args: args{
			ts: arrayParamTaskSpec,
			tr: arrayTaskRun3Elements,
		},
		want: applyMutation(arrayParamTaskSpec, func(spec *v1alpha1.TaskSpec) {
			spec.Steps[1].Args = []string{"first", "second", "foo", "bar", "third", "last"}
		}),
	}, {
		name: "multiple arrays",
		args: args{
			ts: multipleArrayParamsTaskSpec,
			tr: arrayTaskRunMultipleArrays,
		},
		want: applyMutation(multipleArrayParamsTaskSpec, func(spec *v1alpha1.TaskSpec) {
			spec.Steps[1].Command = []string{"cmd", "part1", "part2"}
			spec.Steps[1].Args = []string{"first", "second", "foo", "bar", "third", "last"}
		}),
	}, {
		name: "array and normal string parameter",
		args: args{
			ts: arrayAndStringParamTaskSpec,
			tr: arrayTaskRunWith1StringParam,
		},
		want: applyMutation(arrayAndStringParamTaskSpec, func(spec *v1alpha1.TaskSpec) {
			spec.Steps[1].Args = []string{"foo", "second", "middlefirst", "middlesecond", "last"}
		}),
	}, {
		name: "several arrays and strings",
		args: args{
			ts: multipleArrayAndStringsParamsTaskSpec,
			tr: arrayTaskRunMultipleArraysAndStrings,
		},
		want: applyMutation(multipleArrayAndStringsParamsTaskSpec, func(spec *v1alpha1.TaskSpec) {
			spec.Steps[0].Image = "image-bar"
			spec.Steps[1].Command = []string{"cmd", "1-param1", "2-param1", "3-param1", "4-param1"}
			spec.Steps[1].Args = []string{"1-param2", "2-param2", "2-param3", "second", "1-param1", "2-param1", "3-param1", "4-param1", "foo", "last"}
		}),
	}, {
		name: "default array parameter",
		args: args{
			ts: arrayParamTaskSpec,
			tr: &v1alpha1.TaskRun{},
			dp: []v1alpha1.ParamSpec{
				{
					Name:    "array-param",
					Default: builder.ArrayOrString("defaulted", "value!"),
				},
			},
		},
		want: applyMutation(arrayParamTaskSpec, func(spec *v1alpha1.TaskSpec) {
			spec.Steps[1].Args = []string{"first", "second", "defaulted", "value!", "last"}
		}),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resources.ApplyParameters(tt.args.ts, tt.args.tr, tt.args.dp...)
			if d := cmp.Diff(got, tt.want); d != "" {
				t.Errorf("ApplyParameters() got diff %s", d)
			}
		})
	}
}

func TestApplyParameters(t *testing.T) {
	tr := &v1alpha1.TaskRun{
		Spec: v1alpha1.TaskRunSpec{
			Inputs: v1alpha1.TaskRunInputs{
				Params: []v1alpha1.Param{{
					Name:  "myimage",
					Value: *builder.ArrayOrString("bar"),
				}, {
					Name:  "FOO",
					Value: *builder.ArrayOrString("world"),
				}},
			},
		},
	}
	dp := []v1alpha1.ParamSpec{{
		Name:    "something",
		Default: builder.ArrayOrString("mydefault"),
	}, {
		Name:    "somethingelse",
		Default: builder.ArrayOrString(""),
	}}
	want := applyMutation(simpleTaskSpec, func(spec *v1alpha1.TaskSpec) {
		spec.StepTemplate.Env[0].Value = "world"

		spec.Steps[0].Image = "bar"
		spec.Steps[2].Image = "mydefault"
		spec.Steps[3].Image = "bar"
		spec.Steps[4].Image = ""

		spec.Steps[6].VolumeMounts[0].Name = "world"
		spec.Steps[6].VolumeMounts[0].SubPath = "sub/world/path"
		spec.Steps[6].VolumeMounts[0].MountPath = "path/to/world"
		spec.Steps[6].Image = "busybox:world"

		spec.Steps[7].Env[0].Value = "value-world"
		spec.Steps[7].Env[1].ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name = "config-world"
		spec.Steps[7].Env[1].ValueFrom.ConfigMapKeyRef.Key = "config-key-world"
		spec.Steps[7].Env[2].ValueFrom.SecretKeyRef.LocalObjectReference.Name = "secret-world"
		spec.Steps[7].Env[2].ValueFrom.SecretKeyRef.Key = "secret-key-world"
		spec.Steps[7].EnvFrom[0].Prefix = "prefix-0-world"
		spec.Steps[7].EnvFrom[0].ConfigMapRef.LocalObjectReference.Name = "config-world"
		spec.Steps[7].EnvFrom[1].Prefix = "prefix-1-world"
		spec.Steps[7].EnvFrom[1].SecretRef.LocalObjectReference.Name = "secret-world"
		spec.Steps[7].Image = "busybox:world"
		spec.Steps[8].Image = "$(outputs.resources.imageToUse-ab.path)"
		spec.Steps[9].Image = "$(outputs.resources.imageToUse-re.path)"

		spec.Volumes[0].Name = "world"
		spec.Volumes[0].VolumeSource.ConfigMap.LocalObjectReference.Name = "world"
		spec.Volumes[1].VolumeSource.Secret.SecretName = "world"
		spec.Volumes[2].VolumeSource.PersistentVolumeClaim.ClaimName = "world"

		spec.Sidecars[0].Image = "bar"
		spec.Sidecars[0].Env[0].Value = "world"
	})
	got := resources.ApplyParameters(simpleTaskSpec, tr, dp...)
	if d := cmp.Diff(got, want); d != "" {
		t.Errorf("ApplyParameters() got diff %s", d)
	}
}

func TestApplyResources(t *testing.T) {
	type args struct {
		ts   *v1alpha1.TaskSpec
		r    map[string]v1alpha1.PipelineResourceInterface
		rStr string
	}
	tests := []struct {
		name string
		args args
		want *v1alpha1.TaskSpec
	}{{
		name: "no replacements specified",
		args: args{
			ts:   simpleTaskSpec,
			r:    make(map[string]v1alpha1.PipelineResourceInterface),
			rStr: "inputs",
		},
		want: applyMutation(simpleTaskSpec, func(spec *v1alpha1.TaskSpec) {
			spec.Steps[1].WorkingDir = "/workspace/workspace"
			spec.Steps[4].WorkingDir = "/workspace/workspace"
			spec.Steps[8].Image = "/foo/builtImage"
			spec.Steps[9].Image = "/workspace/foo/builtImage"
		}),
	}, {
		name: "input resource specified",
		args: args{
			ts:   simpleTaskSpec,
			r:    inputs,
			rStr: "inputs",
		},
		want: applyMutation(simpleTaskSpec, func(spec *v1alpha1.TaskSpec) {
			spec.Steps[1].WorkingDir = "/workspace/workspace"
			spec.Steps[1].Args = []string{"https://git-repo"}
			spec.Steps[4].WorkingDir = "/workspace/workspace"
			spec.Steps[4].Args = []string{"https://git-repo"}
			spec.Steps[8].Image = "/foo/builtImage"
			spec.Steps[9].Image = "/workspace/foo/builtImage"
		}),
	}, {
		name: "output resource specified",
		args: args{
			ts:   simpleTaskSpec,
			r:    outputs,
			rStr: "outputs",
		},
		want: applyMutation(simpleTaskSpec, func(spec *v1alpha1.TaskSpec) {
			spec.Steps[1].WorkingDir = "/workspace/workspace"
			spec.Steps[2].Args = []string{"gcr.io/hans/sandwiches"}
			spec.Steps[4].WorkingDir = "/workspace/workspace"
			spec.Steps[5].Args = []string{"gcr.io/hans/sandwiches"}
			spec.Steps[8].Image = "/foo/builtImage"
			spec.Steps[9].Image = "/workspace/foo/builtImage"
		}),
	}, {
		name: "output resource specified with path replacement",
		args: args{
			ts:   gcsTaskSpec,
			r:    outputs,
			rStr: "outputs",
		},
		want: applyMutation(gcsTaskSpec, func(spec *v1alpha1.TaskSpec) {
			spec.Steps[0].Args = []string{"/workspace/output/bucket"}
		}),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resources.ApplyResources(tt.args.ts, tt.args.r, tt.args.rStr)
			if d := cmp.Diff(got, tt.want); d != "" {
				t.Errorf("ApplyResources() -want, +got: %v", d)
			}
		})
	}
}

func TestApplyWorkspaces(t *testing.T) {
	names.TestingSeed()
	ts := &v1alpha1.TaskSpec{
		StepTemplate: &corev1.Container{
			Env: []corev1.EnvVar{{
				Name:  "template-var",
				Value: "$(workspaces.myws.volume)",
			}},
		},
		Steps: []v1alpha1.Step{{Container: corev1.Container{
			Name:       "$(workspaces.myws.volume)",
			Image:      "$(workspaces.otherws.volume)",
			WorkingDir: "$(workspaces.otherws.volume)",
			Args:       []string{"$(workspaces.myws.path)"},
		}}, {Container: corev1.Container{
			Name:  "foo",
			Image: "bar",
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "$(workspaces.myws.volume)",
				MountPath: "path/to/$(workspaces.otherws.path)",
				SubPath:   "$(workspaces.myws.volume)",
			}},
		}}, {Container: corev1.Container{
			Name:  "foo",
			Image: "bar",
			Env: []corev1.EnvVar{{
				Name:  "foo",
				Value: "$(workspaces.myws.volume)",
			}, {
				Name: "baz",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "$(workspaces.myws.volume)"},
						Key:                  "$(workspaces.myws.volume)",
					},
				},
			}},
			EnvFrom: []corev1.EnvFromSource{{
				Prefix: "$(workspaces.myws.volume)",
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "$(workspaces.myws.volume)"},
				},
			}},
		}}},
		Volumes: []corev1.Volume{{
			Name: "$(workspaces.myws.volume)",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "$(workspaces.myws.volume)",
					},
				},
			}}, {
			Name: "some-secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "$(workspaces.myws.volume)",
				},
			}}, {
			Name: "some-pvc",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "$(workspaces.myws.volume)",
				},
			},
		},
		},
	}
	want := applyMutation(ts, func(spec *v1alpha1.TaskSpec) {
		spec.StepTemplate.Env[0].Value = "ws-9l9zj"

		spec.Steps[0].Name = "ws-9l9zj"
		spec.Steps[0].Image = "ws-mz4c7"
		spec.Steps[0].WorkingDir = "ws-mz4c7"
		spec.Steps[0].Args = []string{"/workspace/myws"}

		spec.Steps[1].VolumeMounts[0].Name = "ws-9l9zj"
		spec.Steps[1].VolumeMounts[0].MountPath = "path/to//foo"
		spec.Steps[1].VolumeMounts[0].SubPath = "ws-9l9zj"

		spec.Steps[2].Env[0].Value = "ws-9l9zj"
		spec.Steps[2].Env[1].ValueFrom.SecretKeyRef.LocalObjectReference.Name = "ws-9l9zj"
		spec.Steps[2].Env[1].ValueFrom.SecretKeyRef.Key = "ws-9l9zj"
		spec.Steps[2].EnvFrom[0].Prefix = "ws-9l9zj"
		spec.Steps[2].EnvFrom[0].ConfigMapRef.LocalObjectReference.Name = "ws-9l9zj"

		spec.Volumes[0].Name = "ws-9l9zj"
		spec.Volumes[0].VolumeSource.ConfigMap.LocalObjectReference.Name = "ws-9l9zj"
		spec.Volumes[1].VolumeSource.Secret.SecretName = "ws-9l9zj"
		spec.Volumes[2].VolumeSource.PersistentVolumeClaim.ClaimName = "ws-9l9zj"
	})
	w := []v1alpha1.WorkspaceDeclaration{{
		Name: "myws",
	}, {
		Name:      "otherws",
		MountPath: "/foo",
	}}
	wb := []v1alpha1.WorkspaceBinding{{
		Name: "myws",
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
			ClaimName: "foo",
		},
	}, {
		Name:     "otherws",
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	}}
	got := resources.ApplyWorkspaces(ts, w, wb)
	if d := cmp.Diff(got, want); d != "" {
		t.Errorf("ApplyParameters() got diff %s", d)
	}
}
