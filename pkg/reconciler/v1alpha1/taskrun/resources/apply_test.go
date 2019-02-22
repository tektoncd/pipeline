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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var simpleBuild = &buildv1alpha1.Build{
	Spec: buildv1alpha1.BuildSpec{
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
	},
}
var volumeMountBuild = &buildv1alpha1.Build{
	Spec: buildv1alpha1.BuildSpec{
		Steps: []corev1.Container{{
			Name:  "foo",
			Image: "busybox:${inputs.params.FOO}",
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "${inputs.params.FOO}",
				MountPath: "path/to/${inputs.params.FOO}",
				SubPath:   "sub/${inputs.params.FOO}/path",
			}},
		}},
	},
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

func applyMutation(b *buildv1alpha1.Build, f func(b *buildv1alpha1.Build)) *buildv1alpha1.Build {
	b = b.DeepCopy()
	f(b)
	return b
}

func TestApplyParameters(t *testing.T) {
	type args struct {
		b  *buildv1alpha1.Build
		tr *v1alpha1.TaskRun
		dp []v1alpha1.TaskParam
	}
	tests := []struct {
		name string
		args args
		want *buildv1alpha1.Build
	}{{
		name: "single parameter",
		args: args{
			b:  simpleBuild,
			tr: paramTaskRun,
		},
		want: applyMutation(simpleBuild, func(b *buildv1alpha1.Build) {
			b.Spec.Steps[0].Image = "bar"
		}),
	}, {
		name: "volume mount parameter",
		args: args{
			b: volumeMountBuild,
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
		want: applyMutation(volumeMountBuild, func(b *buildv1alpha1.Build) {
			b.Spec.Steps[0].VolumeMounts[0].Name = "world"
			b.Spec.Steps[0].VolumeMounts[0].SubPath = "sub/world/path"
			b.Spec.Steps[0].VolumeMounts[0].MountPath = "path/to/world"
			b.Spec.Steps[0].Image = "busybox:world"
		}),
	}, {
		name: "with default parameter",
		args: args{
			b:  simpleBuild,
			tr: &v1alpha1.TaskRun{},
			dp: []v1alpha1.TaskParam{
				{
					Name:    "myimage",
					Default: "mydefault",
				},
			},
		},
		want: applyMutation(simpleBuild, func(b *buildv1alpha1.Build) {
			b.Spec.Steps[0].Image = "mydefault"
		}),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyParameters(tt.args.b, tt.args.tr, tt.args.dp...)
			if d := cmp.Diff(got, tt.want); d != "" {
				t.Errorf("ApplyParameters() got diff %s", d)
			}
		})
	}
}

type rg struct {
	resources map[string]*v1alpha1.PipelineResource
}

func (rg *rg) Get(name string) (*v1alpha1.PipelineResource, error) {
	if pr, ok := rg.resources[name]; ok {
		return pr, nil
	}
	return nil, fmt.Errorf("resource %s does not exist", name)
}

func (rg *rg) With(name string, pr *v1alpha1.PipelineResource) *rg {
	rg.resources[name] = pr
	return rg
}

var mockGetter = func(n string) (*v1alpha1.PipelineResource, error) { return &v1alpha1.PipelineResource{}, nil }
var gitResourceGetter = func(n string) (*v1alpha1.PipelineResource, error) { return gitResource, nil }
var imageResourceGetter = func(n string) (*v1alpha1.PipelineResource, error) { return imageResource, nil }

func TestApplyResources(t *testing.T) {
	type args struct {
		b      *buildv1alpha1.Build
		r      []v1alpha1.TaskResourceBinding
		getter GetResource
		rStr   string
	}
	tests := []struct {
		name    string
		args    args
		want    *buildv1alpha1.Build
		wantErr bool
	}{
		{
			name: "no replacements specified",
			args: args{
				b:      simpleBuild,
				r:      []v1alpha1.TaskResourceBinding{},
				getter: mockGetter,
				rStr:   "inputs",
			},
			want: simpleBuild,
		},
		{
			name: "input resource specified",
			args: args{
				b:      simpleBuild,
				r:      inputs,
				getter: gitResourceGetter,
				rStr:   "inputs",
			},
			want: applyMutation(simpleBuild, func(b *buildv1alpha1.Build) {
				b.Spec.Steps[1].Args = []string{"https://git-repo"}
			}),
		},
		{
			name: "output resource specified",
			args: args{
				b:      simpleBuild,
				r:      outputs,
				getter: imageResourceGetter,
				rStr:   "outputs",
			},
			want: applyMutation(simpleBuild, func(b *buildv1alpha1.Build) {
				b.Spec.Steps[2].Args = []string{"gcr.io/hans/sandwiches"}
			}),
		},
		{
			name: "resource does not exist",
			args: args{
				b:      simpleBuild,
				r:      inputs,
				getter: mockGetter,
				rStr:   "inputs",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ApplyResources(tt.args.b, tt.args.r, tt.args.getter, tt.args.rStr)
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
		b    *buildv1alpha1.Build
		repl map[string]string
		want *buildv1alpha1.Build
	}{{
		name: "volume replacement",
		b: &buildv1alpha1.Build{
			Spec: buildv1alpha1.BuildSpec{
				Volumes: []corev1.Volume{{
					Name: "${foo}",
				}},
			},
		},
		want: &buildv1alpha1.Build{
			Spec: buildv1alpha1.BuildSpec{
				Volumes: []corev1.Volume{{
					Name: "bar",
				}},
			},
		},
		repl: map[string]string{"foo": "bar"},
	}, {
		name: "volume configmap",
		b: &buildv1alpha1.Build{
			Spec: buildv1alpha1.BuildSpec{
				Volumes: []corev1.Volume{{
					Name: "${name}",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							corev1.LocalObjectReference{"${configmapname}"},
							nil,
							nil,
							nil,
						},
					}},
				},
			},
		},
		repl: map[string]string{
			"name":          "myname",
			"configmapname": "cfgmapname",
		},
		want: &buildv1alpha1.Build{
			Spec: buildv1alpha1.BuildSpec{
				Volumes: []corev1.Volume{{
					Name: "myname",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							corev1.LocalObjectReference{"cfgmapname"},
							nil,
							nil,
							nil,
						},
					}},
				},
			},
		},
	}, {
		name: "volume secretname",
		b: &buildv1alpha1.Build{
			Spec: buildv1alpha1.BuildSpec{
				Volumes: []corev1.Volume{{
					Name: "${name}",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							"${secretname}",
							nil,
							nil,
							nil,
						},
					}},
				},
			},
		},
		repl: map[string]string{
			"name":       "mysecret",
			"secretname": "totallysecure",
		},
		want: &buildv1alpha1.Build{
			Spec: buildv1alpha1.BuildSpec{
				Volumes: []corev1.Volume{{
					Name: "mysecret",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							"totallysecure",
							nil,
							nil,
							nil,
						},
					}},
				},
			},
		},
	}, {
		name: "volume PVC name",
		b: &buildv1alpha1.Build{
			Spec: buildv1alpha1.BuildSpec{
				Volumes: []corev1.Volume{{
					Name: "${name}",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "${FOO}",
						},
					},
				}},
			},
		},
		repl: map[string]string{
			"name": "mypvc",
			"FOO":  "pvcrocks",
		},
		want: &buildv1alpha1.Build{
			Spec: buildv1alpha1.BuildSpec{
				Volumes: []corev1.Volume{{
					Name: "mypvc",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "pvcrocks",
						},
					},
				}},
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyReplacements(tt.b, tt.repl)
			if d := cmp.Diff(got, tt.want); d != "" {
				t.Errorf("ApplyResources() diff %s", d)
			}
		})
	}
}
