/*
Copyright 2019 The Tekton Authors.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var simpleTaskSpec = &v1alpha1.TaskSpec{
	Steps: []corev1.Container{{
		Name:  "foo",
		Image: "${inputs.params.myimage}",
	}, {
		Name:       "baz",
		Image:      "bat",
		WorkingDir: "${inputs.resources.workspace.path}",
		Args:       []string{"${inputs.resources.workspace.url}"},
	}, {
		Name:  "qux",
		Image: "quux",
		Args:  []string{"${outputs.resources.imageToUse.url}"},
	}},
}

var envTaskSpec = &v1alpha1.TaskSpec{
	Steps: []corev1.Container{{
		Name:  "foo",
		Image: "busybox:${inputs.params.FOO}",
		Env: []corev1.EnvVar{{
			Name:  "foo",
			Value: "value-${inputs.params.FOO}",
		}, {
			Name: "bar",
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "config-${inputs.params.FOO}"},
					Key:                  "config-key-${inputs.params.FOO}",
				},
			},
		}, {
			Name: "baz",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "secret-${inputs.params.FOO}"},
					Key:                  "secret-key-${inputs.params.FOO}",
				},
			},
		}},
		EnvFrom: []corev1.EnvFromSource{{
			Prefix: "prefix-0-${inputs.params.FOO}",
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: "config-${inputs.params.FOO}"},
			},
		}, {
			Prefix: "prefix-1-${inputs.params.FOO}",
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: "secret-${inputs.params.FOO}"},
			},
		}},
	}},
}

var containerTemplateTaskSpec = &v1alpha1.TaskSpec{
	ContainerTemplate: &corev1.Container{
		Env: []corev1.EnvVar{{
			Name:  "template-var",
			Value: "${inputs.params.FOO}",
		}},
	},
	Steps: []corev1.Container{{
		Name:  "simple-image",
		Image: "${inputs.params.myimage}",
	}, {
		Name:  "image-with-env-specified",
		Image: "some-other-image",
		Env: []corev1.EnvVar{{
			Name:  "template-var",
			Value: "overridden-value",
		}},
	}},
}

var volumeMountTaskSpec = &v1alpha1.TaskSpec{
	Steps: []corev1.Container{{
		Name:  "foo",
		Image: "busybox:${inputs.params.FOO}",
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "${inputs.params.FOO}",
			MountPath: "path/to/${inputs.params.FOO}",
			SubPath:   "sub/${inputs.params.FOO}/path",
		}},
	}},
}

var gcsTaskSpec = &v1alpha1.TaskSpec{
	Steps: []corev1.Container{{
		Name:  "foobar",
		Image: "someImage",
		Args:  []string{"${outputs.resources.bucket.path}"},
	}},
}

var paramTaskRun = &v1alpha1.TaskRun{
	Spec: v1alpha1.TaskRunSpec{
		Inputs: v1alpha1.TaskRunInputs{
			Params: []v1alpha1.Param{
				{
					Name:  "myimage",
					Value: "bar",
				},
			},
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
		Params: []v1alpha1.Param{
			{
				Name:  "URL",
				Value: "https://git-repo",
			},
		},
	},
})

var imageResource, _ = v1alpha1.ResourceFromType(&v1alpha1.PipelineResource{
	ObjectMeta: metav1.ObjectMeta{
		Name: "image-resource",
	},
	Spec: v1alpha1.PipelineResourceSpec{
		Type: v1alpha1.PipelineResourceTypeImage,
		Params: []v1alpha1.Param{
			{
				Name:  "URL",
				Value: "gcr.io/hans/sandwiches",
			},
		},
	},
})

var gcsResource, _ = v1alpha1.ResourceFromType(&v1alpha1.PipelineResource{
	ObjectMeta: metav1.ObjectMeta{
		Name: "gcs-resource",
	},
	Spec: v1alpha1.PipelineResourceSpec{
		Type: v1alpha1.PipelineResourceTypeStorage,
		Params: []v1alpha1.Param{
			{
				Name:  "type",
				Value: "gcs",
			},
			{
				Name:  "location",
				Value: "theCloud?",
			},
		},
	},
})

func applyMutation(ts *v1alpha1.TaskSpec, f func(*v1alpha1.TaskSpec)) *v1alpha1.TaskSpec {
	ts = ts.DeepCopy()
	f(ts)
	return ts
}

func setup() {
	inputs["workspace"].SetDestinationDirectory("/workspace/workspace")
	outputs["bucket"].SetDestinationDirectory("/workspace/outputs/bucket")
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
							Value: "world",
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
							Value: "world",
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
		name: "containerTemplate parameter",
		args: args{
			ts: containerTemplateTaskSpec,
			tr: &v1alpha1.TaskRun{
				Spec: v1alpha1.TaskRunSpec{
					Inputs: v1alpha1.TaskRunInputs{
						Params: []v1alpha1.Param{{
							Name:  "FOO",
							Value: "BAR",
						}},
					},
				},
			},
			dp: []v1alpha1.ParamSpec{
				{
					Name:    "myimage",
					Default: "replaced-image-name",
				},
			},
		},
		want: applyMutation(containerTemplateTaskSpec, func(spec *v1alpha1.TaskSpec) {
			spec.ContainerTemplate.Env[0].Value = "BAR"
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
					Default: "mydefault",
				},
			},
		},
		want: applyMutation(simpleTaskSpec, func(spec *v1alpha1.TaskSpec) {
			spec.Steps[0].Image = "mydefault"
		}),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyParameters(tt.args.ts, tt.args.tr, tt.args.dp...)
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
		want: simpleTaskSpec,
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
		}),
	}, {
		name: "output resource specified",
		args: args{
			ts:   simpleTaskSpec,
			r:    outputs,
			rStr: "outputs",
		},
		want: applyMutation(simpleTaskSpec, func(spec *v1alpha1.TaskSpec) {
			spec.Steps[2].Args = []string{"gcr.io/hans/sandwiches"}
		}),
	}, {
		name: "output resource specified with path replacement",
		args: args{
			ts:   gcsTaskSpec,
			r:    outputs,
			rStr: "outputs",
		},
		want: applyMutation(gcsTaskSpec, func(spec *v1alpha1.TaskSpec) {
			spec.Steps[0].Args = []string{"/workspace/outputs/bucket"}
		}),
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup()
			got := ApplyResources(tt.args.ts, tt.args.r, tt.args.rStr)
			if d := cmp.Diff(got, tt.want); d != "" {
				t.Errorf("ApplyResources() diff %s", d)
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
		name: "volume configmap",
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
		name: "volume secretname",
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
		name: "volume PVC name",
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
			got := ApplyReplacements(tt.ts, tt.repl)
			if d := cmp.Diff(got, tt.want); d != "" {
				t.Errorf("ApplyResources() diff %s", d)
			}
		})
	}
}
