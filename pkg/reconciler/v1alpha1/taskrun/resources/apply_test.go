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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/taskrun/resources"
	"github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var simpleTaskSpec = &v1alpha1.TaskSpec{
	Inputs: &v1alpha1.Inputs{
		Resources: []v1alpha1.TaskResource{
			{
				Name: "workspace",
			},
		},
	},
	Outputs: &v1alpha1.Outputs{
		Resources: []v1alpha1.TaskResource{
			{
				Name: "imageToUse",
			},
		},
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
		Image: "quux",
		Args:  []string{"$(outputs.resources.imageToUse.url)"},
	}}, {Container: corev1.Container{
		Name: "foo",
		// TODO(#1170): Remove support for ${} syntax
		Image: "${inputs.params.myimage}",
	}}, {Container: corev1.Container{
		Name:  "baz",
		Image: "bat",
		// TODO(#1170): Remove support for ${} syntax
		WorkingDir: "${inputs.resources.workspace.path}",
		Args:       []string{"${inputs.resources.workspace.url}"},
	}}, {Container: corev1.Container{
		Name:  "qux",
		Image: "quux",
		// TODO(#1170): Remove support for ${} syntax
		Args: []string{"${outputs.resources.imageToUse.url}"},
	}}},
}

var envTaskSpec = &v1alpha1.TaskSpec{
	Steps: []v1alpha1.Step{{Container: corev1.Container{
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
			// TODO(#1170): Remove support for ${} syntax
			Name: "baz",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "secret-${inputs.params.FOO}"},
					Key:                  "secret-key-${inputs.params.FOO}",
				},
			},
		}},
		EnvFrom: []corev1.EnvFromSource{{
			Prefix: "prefix-0-$(inputs.params.FOO)",
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: "config-$(inputs.params.FOO)"},
			},
		}, {
			// TODO(#1170): Remove support for ${} syntax
			Prefix: "prefix-1-${inputs.params.FOO}",
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: "secret-${inputs.params.FOO}"},
			},
		}},
	}}},
}

var stepTemplateTaskSpec = &v1alpha1.TaskSpec{
	StepTemplate: &corev1.Container{
		Env: []corev1.EnvVar{{
			Name:  "template-var",
			Value: "$(inputs.params.FOO)",
		}},
	},
	Steps: []v1alpha1.Step{{Container: corev1.Container{
		Name:  "simple-image",
		Image: "$(inputs.params.myimage)",
	}}, {Container: corev1.Container{
		Name:  "image-with-env-specified",
		Image: "some-other-image",
		Env: []corev1.EnvVar{{
			Name:  "template-var",
			Value: "overridden-value",
		}},
	}}},
}

var volumeMountTaskSpec = &v1alpha1.TaskSpec{
	Steps: []v1alpha1.Step{{Container: corev1.Container{
		Name:  "foo",
		Image: "busybox:$(inputs.params.FOO)",
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "$(inputs.params.FOO)",
			MountPath: "path/to/$(inputs.params.FOO)",
			SubPath:   "sub/$(inputs.params.FOO)/path",
		}},
	}}},
}

var gcsTaskSpec = &v1alpha1.TaskSpec{
	Outputs: &v1alpha1.Outputs{
		Resources: []v1alpha1.TaskResource{{
			Name: "bucket",
		}},
	},
	Steps: []v1alpha1.Step{{Container: corev1.Container{
		Name:  "foobar",
		Image: "someImage",
		Args:  []string{"$(outputs.resources.bucket.path)"},
	}}},
}

var arrayParamTaskSpec = &v1alpha1.TaskSpec{
	Steps: []v1alpha1.Step{{Container: corev1.Container{
		Name:  "simple-image",
		Image: "some-image",
	}}, {Container: corev1.Container{
		Name:    "image-with-args-specified",
		Image:   "some-other-image",
		Command: []string{"echo"},
		Args:    []string{"first", "second", "$(inputs.params.array-param)", "last"},
	}}},
}

var arrayAndStringParamTaskSpec = &v1alpha1.TaskSpec{
	Steps: []v1alpha1.Step{{Container: corev1.Container{
		Name:  "simple-image",
		Image: "some-image",
	}}, {Container: corev1.Container{
		Name:    "image-with-args-specified",
		Image:   "some-other-image",
		Command: []string{"echo"},
		Args:    []string{"$(inputs.params.normal-param)", "second", "$(inputs.params.array-param)", "last"},
	}}},
}

var multipleArrayParamsTaskSpec = &v1alpha1.TaskSpec{
	Steps: []v1alpha1.Step{{Container: corev1.Container{
		Name:  "simple-image",
		Image: "some-image",
	}}, {Container: corev1.Container{
		Name:    "image-with-args-specified",
		Image:   "some-other-image",
		Command: []string{"cmd", "$(inputs.params.another-array-param)"},
		Args:    []string{"first", "second", "$(inputs.params.array-param)", "last"},
	}}},
}

var multipleArrayAndStringsParamsTaskSpec = &v1alpha1.TaskSpec{
	Steps: []v1alpha1.Step{{Container: corev1.Container{
		Name:  "simple-image",
		Image: "image-$(inputs.params.string-param2)",
	}}, {Container: corev1.Container{
		Name:    "image-with-args-specified",
		Image:   "some-other-image",
		Command: []string{"cmd", "$(inputs.params.array-param1)"},
		Args:    []string{"$(inputs.params.array-param2)", "second", "$(inputs.params.array-param1)", "$(inputs.params.string-param1)", "last"},
	}}},
}

var paramTaskRun = &v1alpha1.TaskRun{
	Spec: v1alpha1.TaskRunSpec{
		Inputs: v1alpha1.TaskRunInputs{
			Params: []v1alpha1.Param{{
				Name:  "myimage",
				Value: *builder.ArrayOrString("bar"),
			}},
		},
	},
}

var arrayTaskRun0Elements = &v1alpha1.TaskRun{
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

var arrayTaskRun1Elements = &v1alpha1.TaskRun{
	Spec: v1alpha1.TaskRunSpec{
		Inputs: v1alpha1.TaskRunInputs{
			Params: []v1alpha1.Param{{
				Name:  "array-param",
				Value: *builder.ArrayOrString("foo"),
			}},
		},
	},
}

var arrayTaskRun3Elements = &v1alpha1.TaskRun{
	Spec: v1alpha1.TaskRunSpec{
		Inputs: v1alpha1.TaskRunInputs{
			Params: []v1alpha1.Param{{
				Name:  "array-param",
				Value: *builder.ArrayOrString("foo", "bar", "third"),
			}},
		},
	},
}

var arrayTaskRunMultipleArrays = &v1alpha1.TaskRun{
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

var arrayTaskRunWith1StringParam = &v1alpha1.TaskRun{
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

var arrayTaskRunMultipleArraysAndStrings = &v1alpha1.TaskRun{
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

var inputs = map[string]v1alpha1.PipelineResourceInterface{
	"workspace": gitResource,
}

var outputs = map[string]v1alpha1.PipelineResourceInterface{
	"imageToUse": imageResource,
	"bucket":     gcsResource,
}

var gitResource, _ = v1alpha1.ResourceFromType(&v1alpha1.PipelineResource{
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
})

var imageResource, _ = v1alpha1.ResourceFromType(&v1alpha1.PipelineResource{
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
})

var gcsResource, _ = v1alpha1.ResourceFromType(&v1alpha1.PipelineResource{
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
})

func applyMutation(ts *v1alpha1.TaskSpec, f func(*v1alpha1.TaskSpec)) *v1alpha1.TaskSpec {
	ts = ts.DeepCopy()
	f(ts)
	return ts
}

func TestApplyParameters(t *testing.T) {
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
		name: "single parameter",
		args: args{
			ts: simpleTaskSpec,
			tr: paramTaskRun,
		},
		want: applyMutation(simpleTaskSpec, func(spec *v1alpha1.TaskSpec) {
			spec.Steps[0].Image = "bar"
			spec.Steps[3].Image = "bar"
		}),
	}, {
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
	}, {
		name: "volume mount parameter",
		args: args{
			ts: volumeMountTaskSpec,
			tr: &v1alpha1.TaskRun{
				Spec: v1alpha1.TaskRunSpec{
					Inputs: v1alpha1.TaskRunInputs{
						Params: []v1alpha1.Param{{
							Name:  "FOO",
							Value: *builder.ArrayOrString("world"),
						}},
					},
				},
			},
		},
		want: applyMutation(volumeMountTaskSpec, func(spec *v1alpha1.TaskSpec) {
			spec.Steps[0].VolumeMounts[0].Name = "world"
			spec.Steps[0].VolumeMounts[0].SubPath = "sub/world/path"
			spec.Steps[0].VolumeMounts[0].MountPath = "path/to/world"
			spec.Steps[0].Image = "busybox:world"
		}),
	}, {
		name: "envs parameter",
		args: args{
			ts: envTaskSpec,
			tr: &v1alpha1.TaskRun{
				Spec: v1alpha1.TaskRunSpec{
					Inputs: v1alpha1.TaskRunInputs{
						Params: []v1alpha1.Param{{
							Name:  "FOO",
							Value: *builder.ArrayOrString("world"),
						}},
					},
				},
			},
		},
		want: applyMutation(envTaskSpec, func(spec *v1alpha1.TaskSpec) {
			spec.Steps[0].Env[0].Value = "value-world"
			spec.Steps[0].Env[1].ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name = "config-world"
			spec.Steps[0].Env[1].ValueFrom.ConfigMapKeyRef.Key = "config-key-world"
			spec.Steps[0].Env[2].ValueFrom.SecretKeyRef.LocalObjectReference.Name = "secret-world"
			spec.Steps[0].Env[2].ValueFrom.SecretKeyRef.Key = "secret-key-world"
			spec.Steps[0].EnvFrom[0].Prefix = "prefix-0-world"
			spec.Steps[0].EnvFrom[0].ConfigMapRef.LocalObjectReference.Name = "config-world"
			spec.Steps[0].EnvFrom[1].Prefix = "prefix-1-world"
			spec.Steps[0].EnvFrom[1].SecretRef.LocalObjectReference.Name = "secret-world"
			spec.Steps[0].Image = "busybox:world"
		}),
	}, {
		name: "stepTemplate parameter",
		args: args{
			ts: stepTemplateTaskSpec,
			tr: &v1alpha1.TaskRun{
				Spec: v1alpha1.TaskRunSpec{
					Inputs: v1alpha1.TaskRunInputs{
						Params: []v1alpha1.Param{{
							Name:  "FOO",
							Value: *builder.ArrayOrString("BAR"),
						}},
					},
				},
			},
			dp: []v1alpha1.ParamSpec{
				{
					Name:    "myimage",
					Default: builder.ArrayOrString("replaced-image-name"),
				},
			},
		},
		want: applyMutation(stepTemplateTaskSpec, func(spec *v1alpha1.TaskSpec) {
			spec.StepTemplate.Env[0].Value = "BAR"
			spec.Steps[0].Image = "replaced-image-name"
		}),
	}, {
		name: "with default parameter",
		args: args{
			ts: simpleTaskSpec,
			tr: &v1alpha1.TaskRun{},
			dp: []v1alpha1.ParamSpec{
				{
					Name:    "myimage",
					Default: builder.ArrayOrString("mydefault"),
				},
			},
		},
		want: applyMutation(simpleTaskSpec, func(spec *v1alpha1.TaskSpec) {
			spec.Steps[0].Image = "mydefault"
			spec.Steps[3].Image = "mydefault"
		}),
	}, {
		name: "with empty string default parameter",
		args: args{
			ts: simpleTaskSpec,
			tr: &v1alpha1.TaskRun{},
			dp: []v1alpha1.ParamSpec{
				{
					Name:    "myimage",
					Default: builder.ArrayOrString(""),
				},
			},
		},
		want: applyMutation(simpleTaskSpec, func(spec *v1alpha1.TaskSpec) {
			spec.Steps[0].Image = ""
			spec.Steps[3].Image = ""
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

func TestVolumeReplacement(t *testing.T) {
	tests := []struct {
		name string
		ts   *v1alpha1.TaskSpec
		repl map[string]string
		want *v1alpha1.TaskSpec
	}{{
		name: "volume replacement",
		ts: &v1alpha1.TaskSpec{
			Volumes: []corev1.Volume{{
				Name: "$(foo)",
			}},
		},
		repl: map[string]string{"foo": "bar"},
		want: &v1alpha1.TaskSpec{
			Volumes: []corev1.Volume{{
				Name: "bar",
			}},
		},
	}, {
		name: "volume configmap",
		ts: &v1alpha1.TaskSpec{
			Volumes: []corev1.Volume{{
				Name: "$(name)",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "$(configmapname)",
						},
					},
				}},
			},
		},
		repl: map[string]string{
			"name":          "myname",
			"configmapname": "cfgmapname",
		},
		want: &v1alpha1.TaskSpec{
			Volumes: []corev1.Volume{{
				Name: "myname",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "cfgmapname",
						},
					},
				}},
			},
		},
	}, {
		name: "volume secretname",
		ts: &v1alpha1.TaskSpec{
			Volumes: []corev1.Volume{{
				Name: "$(name)",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "$(secretname)",
					},
				}},
			},
		},
		repl: map[string]string{
			"name":       "mysecret",
			"secretname": "totallysecure",
		},
		want: &v1alpha1.TaskSpec{
			Volumes: []corev1.Volume{{
				Name: "mysecret",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "totallysecure",
					},
				}},
			},
		},
	}, {
		name: "volume PVC name",
		ts: &v1alpha1.TaskSpec{
			Volumes: []corev1.Volume{{
				Name: "$(name)",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "$(FOO)",
					},
				},
			}},
		},
		repl: map[string]string{
			"name": "mypvc",
			"FOO":  "pvcrocks",
		},
		want: &v1alpha1.TaskSpec{
			Volumes: []corev1.Volume{{
				Name: "mypvc",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "pvcrocks",
					},
				},
			}},
		},
	}, {
		// TODO(#1170): Remove support for ${} syntax
		name: "deprecated volume replacement",
		ts: &v1alpha1.TaskSpec{
			Volumes: []corev1.Volume{{
				Name: "${foo}",
			}},
		},
		repl: map[string]string{"foo": "bar"},
		want: &v1alpha1.TaskSpec{
			Volumes: []corev1.Volume{{
				Name: "bar",
			}},
		},
	}, {
		// TODO(#1170): Remove support for ${} syntax
		name: "deprecated volume configmap",
		ts: &v1alpha1.TaskSpec{
			Volumes: []corev1.Volume{{
				Name: "${name}",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "${configmapname}",
						},
					},
				}},
			},
		},
		repl: map[string]string{
			"name":          "myname",
			"configmapname": "cfgmapname",
		},
		want: &v1alpha1.TaskSpec{
			Volumes: []corev1.Volume{{
				Name: "myname",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "cfgmapname",
						},
					},
				}},
			},
		},
	}, {
		// TODO(#1170): Remove support for ${} syntax
		name: "deprecated volume secretname",
		ts: &v1alpha1.TaskSpec{
			Volumes: []corev1.Volume{{
				Name: "${name}",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "${secretname}",
					},
				}},
			},
		},
		repl: map[string]string{
			"name":       "mysecret",
			"secretname": "totallysecure",
		},
		want: &v1alpha1.TaskSpec{
			Volumes: []corev1.Volume{{
				Name: "mysecret",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "totallysecure",
					},
				}},
			},
		},
	}, {
		// TODO(#1170): Remove support for ${} syntax
		name: "deprecated volume PVC name",
		ts: &v1alpha1.TaskSpec{
			Volumes: []corev1.Volume{{
				Name: "${name}",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "${FOO}",
					},
				},
			}},
		},
		repl: map[string]string{
			"name": "mypvc",
			"FOO":  "pvcrocks",
		},
		want: &v1alpha1.TaskSpec{
			Volumes: []corev1.Volume{{
				Name: "mypvc",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "pvcrocks",
					},
				},
			}},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resources.ApplyReplacements(tt.ts, tt.repl, map[string][]string{})
			if d := cmp.Diff(got, tt.want); d != "" {
				t.Errorf("ApplyResources() diff %s", d)
			}
		})
	}
}
