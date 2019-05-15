/*
Copyright 2018 The Knative Authors.

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
		Name:  "baz",
		Image: "bat",
		Args:  []string{"${inputs.resources.workspace.url}"},
	}, {
		Name:  "qux",
		Image: "quux",
		Args:  []string{"${outputs.resources.imageToUse.url}"},
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

var inputs = []v1alpha1.TaskResourceBinding{{
	ResourceRef: v1alpha1.PipelineResourceRef{
		Name: "git-resource",
	},
	Name: "workspace",
}}

var outputs = []v1alpha1.TaskResourceBinding{{
	ResourceRef: v1alpha1.PipelineResourceRef{
		Name: "image-resource",
	},
	Name: "imageToUse",
}}

var gitResource = &v1alpha1.PipelineResource{
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
}

var imageResource = &v1alpha1.PipelineResource{
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
}

func applyMutation(ts *v1alpha1.TaskSpec, f func(*v1alpha1.TaskSpec)) *v1alpha1.TaskSpec {
	ts = ts.DeepCopy()
	f(ts)
	return ts
}

func TestApplyParameters(t *testing.T) {
	type args struct {
		ts *v1alpha1.TaskSpec
		tr *v1alpha1.TaskRun
		dp []v1alpha1.TaskParam
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
		name: "with default parameter",
		args: args{
			ts: simpleTaskSpec,
			tr: &v1alpha1.TaskRun{},
			dp: []v1alpha1.TaskParam{
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

var mockGetter = func(n string) (*v1alpha1.PipelineResource, error) { return &v1alpha1.PipelineResource{}, nil }
var gitResourceGetter = func(n string) (*v1alpha1.PipelineResource, error) { return gitResource, nil }
var imageResourceGetter = func(n string) (*v1alpha1.PipelineResource, error) { return imageResource, nil }

func TestApplyResources(t *testing.T) {
	type args struct {
		ts     *v1alpha1.TaskSpec
		r      []v1alpha1.TaskResourceBinding
		getter GetResource
		rStr   string
	}
	tests := []struct {
		name    string
		args    args
		want    *v1alpha1.TaskSpec
		wantErr bool
	}{{
		name: "no replacements specified",
		args: args{
			ts:     simpleTaskSpec,
			r:      []v1alpha1.TaskResourceBinding{},
			getter: mockGetter,
			rStr:   "inputs",
		},
		want: simpleTaskSpec,
	}, {
		name: "input resource specified",
		args: args{
			ts:     simpleTaskSpec,
			r:      inputs,
			getter: gitResourceGetter,
			rStr:   "inputs",
		},
		want: applyMutation(simpleTaskSpec, func(spec *v1alpha1.TaskSpec) {
			spec.Steps[1].Args = []string{"https://git-repo"}
		}),
	}, {
		name: "output resource specified",
		args: args{
			ts:     simpleTaskSpec,
			r:      outputs,
			getter: imageResourceGetter,
			rStr:   "outputs",
		},
		want: applyMutation(simpleTaskSpec, func(spec *v1alpha1.TaskSpec) {
			spec.Steps[2].Args = []string{"gcr.io/hans/sandwiches"}
		}),
	}, {
		name: "resource does not exist",
		args: args{
			ts:     simpleTaskSpec,
			r:      inputs,
			getter: mockGetter,
			rStr:   "inputs",
		},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ApplyResources(tt.args.ts, tt.args.r, tt.args.getter, tt.args.rStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ApplyResources() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
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
