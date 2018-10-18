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
		Steps: []corev1.Container{
			{
				Name:  "foo",
				Image: "${inputs.params.myimage}",
			},
			{
				Name:  "baz",
				Image: "bat",
				Args:  []string{"${inputs.resources.git-resource.url}"},
			},
		},
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

var resourceTaskRun = &v1alpha1.TaskRun{
	Spec: v1alpha1.TaskRunSpec{
		Inputs: v1alpha1.TaskRunInputs{
			Resources: []v1alpha1.PipelineResourceVersion{
				{
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "git-resource",
					},
				},
			},
		},
	},
}

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

func applyMutation(b *buildv1alpha1.Build, f func(b *buildv1alpha1.Build)) *buildv1alpha1.Build {
	b = b.DeepCopy()
	f(b)
	return b
}

func TestApplyParameters(t *testing.T) {
	type args struct {
		b  *buildv1alpha1.Build
		tr *v1alpha1.TaskRun
	}
	tests := []struct {
		name string
		args args
		want *buildv1alpha1.Build
	}{
		{
			name: "single parameter",
			args: args{
				b:  simpleBuild,
				tr: paramTaskRun,
			},
			want: applyMutation(simpleBuild, func(b *buildv1alpha1.Build) {
				b.Spec.Steps[0].Image = "bar"
			}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyParameters(tt.args.b, tt.args.tr)
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
	return nil, fmt.Errorf("Resource %s does not exist.", name)
}

func (rg *rg) With(name string, pr *v1alpha1.PipelineResource) *rg {
	rg.resources[name] = pr
	return rg
}

func mockGetter() *rg {
	return &rg{
		resources: map[string]*v1alpha1.PipelineResource{},
	}
}

func TestApplyResources(t *testing.T) {
	type args struct {
		b      *buildv1alpha1.Build
		tr     *v1alpha1.TaskRun
		getter ResourceGetter
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
				tr:     paramTaskRun,
				getter: mockGetter(),
			},
			want: simpleBuild,
		},
		{
			name: "resource specified",
			args: args{
				b:      simpleBuild,
				tr:     resourceTaskRun,
				getter: mockGetter().With("git-resource", gitResource),
			},
			want: applyMutation(simpleBuild, func(b *buildv1alpha1.Build) {
				b.Spec.Steps[1].Args = []string{"https://git-repo"}
			}),
		},
		{
			name: "resource does not exist",
			args: args{
				b:      simpleBuild,
				tr:     resourceTaskRun,
				getter: mockGetter(),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ApplyResources(tt.args.b, tt.args.tr, tt.args.getter)
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
