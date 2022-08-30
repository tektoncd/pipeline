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

package pipelinespec

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetPipelineSpec_Ref(t *testing.T) {
	pipeline := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "orchestrate",
		},
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name: "mytask",
				TaskRef: &v1beta1.TaskRef{
					Name: "mytask",
				},
			}},
		},
	}
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mypipelinerun",
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				Name: "orchestrate",
			},
		},
	}
	gt := func(ctx context.Context, n string) (v1beta1.PipelineObject, *v1beta1.ConfigSource, error) {
		return pipeline, nil, nil
	}
	resolvedObjectMeta, pipelineSpec, err := GetPipelineData(context.Background(), pr, gt)

	if err != nil {
		t.Fatalf("Did not expect error getting pipeline spec but got: %s", err)
	}

	if resolvedObjectMeta.Name != "orchestrate" {
		t.Errorf("Expected pipeline name to be `orchestrate` but was %q", resolvedObjectMeta.Name)
	}

	if len(pipelineSpec.Tasks) != 1 || pipelineSpec.Tasks[0].Name != "mytask" {
		t.Errorf("Pipeline Spec not resolved as expected, expected referenced Pipeline spec but got: %v", pipelineSpec)
	}

	if resolvedObjectMeta.ConfigSource != nil {
		t.Errorf("Expected resolved configsource is nil, but got %v", resolvedObjectMeta.ConfigSource)
	}
}

func TestGetPipelineSpec_Embedded(t *testing.T) {
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mypipelinerun",
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineSpec: &v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name: "mytask",
					TaskRef: &v1beta1.TaskRef{
						Name: "mytask",
					},
				}},
			},
		},
	}
	gt := func(ctx context.Context, n string) (v1beta1.PipelineObject, *v1beta1.ConfigSource, error) {
		return nil, nil, errors.New("shouldn't be called")
	}
	resolvedObjectMeta, pipelineSpec, err := GetPipelineData(context.Background(), pr, gt)

	if err != nil {
		t.Fatalf("Did not expect error getting pipeline spec but got: %s", err)
	}

	if resolvedObjectMeta.Name != "mypipelinerun" {
		t.Errorf("Expected pipeline name for embedded pipeline to default to name of pipeline run but was %q", resolvedObjectMeta.Name)
	}

	if len(pipelineSpec.Tasks) != 1 || pipelineSpec.Tasks[0].Name != "mytask" {
		t.Errorf("Pipeline Spec not resolved as expected, expected embedded Pipeline spec but got: %v", pipelineSpec)
	}

	if resolvedObjectMeta.ConfigSource != nil {
		t.Errorf("Expected resolved configsource is nil, but got %v", resolvedObjectMeta.ConfigSource)
	}
}

func TestGetPipelineSpec_Invalid(t *testing.T) {
	tr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mypipelinerun",
		},
	}
	gt := func(ctx context.Context, n string) (v1beta1.PipelineObject, *v1beta1.ConfigSource, error) {
		return nil, nil, errors.New("shouldn't be called")
	}
	_, _, err := GetPipelineData(context.Background(), tr, gt)
	if err == nil {
		t.Fatalf("Expected error resolving spec with no embedded or referenced pipeline spec but didn't get error")
	}
}

func TestGetPipelineData_ResolutionSuccess(t *testing.T) {
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mypipelinerun",
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				ResolverRef: v1beta1.ResolverRef{
					Resolver: "foo",
					Params: []v1beta1.Param{{
						Name: "bar",
						Value: v1beta1.ParamValue{
							Type:      v1beta1.ParamTypeString,
							StringVal: "baz",
						},
					}},
				},
			},
		},
	}
	sourceMeta := metav1.ObjectMeta{
		Name: "pipeline",
	}
	sourceSpec := v1beta1.PipelineSpec{
		Tasks: []v1beta1.PipelineTask{{
			Name: "pt1",
			TaskRef: &v1beta1.TaskRef{
				Kind: "Task",
				Name: "tref",
			},
		}},
	}
	expectedConfigSource := &v1beta1.ConfigSource{
		URI: "abc.com",
		Digest: map[string]string{
			"sha1": "a123",
		},
		EntryPoint: "foo/bar",
	}

	getPipeline := func(ctx context.Context, n string) (v1beta1.PipelineObject, *v1beta1.ConfigSource, error) {
		return &v1beta1.Pipeline{
			ObjectMeta: *sourceMeta.DeepCopy(),
			Spec:       *sourceSpec.DeepCopy(),
		}, expectedConfigSource.DeepCopy(), nil
	}
	ctx := context.Background()
	resolvedMeta, resolvedSpec, err := GetPipelineData(ctx, pr, getPipeline)
	if err != nil {
		t.Fatalf("Unexpected error getting mocked data: %v", err)
	}
	if sourceMeta.Name != resolvedMeta.Name {
		t.Errorf("Expected name %q but resolved to %q", sourceMeta.Name, resolvedMeta.Name)
	}
	if d := cmp.Diff(sourceSpec, *resolvedSpec); d != "" {
		t.Errorf(diff.PrintWantGot(d))
	}

	if d := cmp.Diff(expectedConfigSource, resolvedMeta.ConfigSource); d != "" {
		t.Errorf("configsource did not match: %s", diff.PrintWantGot(d))
	}
}

func TestGetPipelineSpec_Error(t *testing.T) {
	tr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mypipelinerun",
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				Name: "orchestrate",
			},
		},
	}
	gt := func(ctx context.Context, n string) (v1beta1.PipelineObject, *v1beta1.ConfigSource, error) {
		return nil, nil, errors.New("something went wrong")
	}
	_, _, err := GetPipelineData(context.Background(), tr, gt)
	if err == nil {
		t.Fatalf("Expected error when unable to find referenced Pipeline but got none")
	}
}

func TestGetPipelineData_ResolutionError(t *testing.T) {
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mypipelinerun",
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				ResolverRef: v1beta1.ResolverRef{
					Resolver: "git",
				},
			},
		},
	}
	getPipeline := func(ctx context.Context, n string) (v1beta1.PipelineObject, *v1beta1.ConfigSource, error) {
		return nil, nil, errors.New("something went wrong")
	}
	ctx := context.Background()
	_, _, err := GetPipelineData(ctx, pr, getPipeline)
	if err == nil {
		t.Fatalf("Expected error when unable to find referenced Pipeline but got none")
	}
}

func TestGetPipelineData_ResolvedNilPipeline(t *testing.T) {
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mypipelinerun",
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				ResolverRef: v1beta1.ResolverRef{
					Resolver: "git",
				},
			},
		},
	}
	getPipeline := func(ctx context.Context, n string) (v1beta1.PipelineObject, *v1beta1.ConfigSource, error) {
		return nil, nil, nil
	}
	ctx := context.Background()
	_, _, err := GetPipelineData(ctx, pr, getPipeline)
	if err == nil {
		t.Fatalf("Expected error when unable to find referenced Pipeline but got none")
	}
}
