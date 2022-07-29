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
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetTaskSpec_Ref(t *testing.T) {
	task := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name: "orchestrate",
		},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Name: "step1",
			}},
		},
	}
	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mytaskrun",
		},
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "orchestrate",
			},
		},
	}
	gt := func(ctx context.Context, n string) (v1beta1.TaskObject, error) { return task, nil }
	taskMeta, taskSpec, err := GetTaskData(context.Background(), tr, gt)

	if err != nil {
		t.Fatalf("Did not expect error getting task spec but got: %s", err)
	}

	if taskMeta.Name != "orchestrate" {
		t.Errorf("Expected task name to be `orchestrate` but was %q", taskMeta.Name)
	}

	if len(taskSpec.Steps) != 1 || taskSpec.Steps[0].Name != "step1" {
		t.Errorf("Task Spec not resolved as expected, expected referenced Task spec but got: %v", taskSpec)
	}
}

func TestGetTaskSpec_Embedded(t *testing.T) {
	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mytaskrun",
		},
		Spec: v1beta1.TaskRunSpec{
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{
					Name: "step1",
				}},
			},
		},
	}
	gt := func(ctx context.Context, n string) (v1beta1.TaskObject, error) {
		return nil, errors.New("shouldn't be called")
	}
	taskMeta, taskSpec, err := GetTaskData(context.Background(), tr, gt)

	if err != nil {
		t.Fatalf("Did not expect error getting task spec but got: %s", err)
	}

	if taskMeta.Name != "mytaskrun" {
		t.Errorf("Expected task name for embedded task to default to name of task run but was %q", taskMeta.Name)
	}

	if len(taskSpec.Steps) != 1 || taskSpec.Steps[0].Name != "step1" {
		t.Errorf("Task Spec not resolved as expected, expected embedded Task spec but got: %v", taskSpec)
	}
}

func TestGetTaskSpec_Invalid(t *testing.T) {
	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mytaskrun",
		},
	}
	gt := func(ctx context.Context, n string) (v1beta1.TaskObject, error) {
		return nil, errors.New("shouldn't be called")
	}
	_, _, err := GetTaskData(context.Background(), tr, gt)
	if err == nil {
		t.Fatalf("Expected error resolving spec with no embedded or referenced task spec but didn't get error")
	}
}

func TestGetTaskSpec_Error(t *testing.T) {
	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mytaskrun",
		},
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "orchestrate",
			},
		},
	}
	gt := func(ctx context.Context, n string) (v1beta1.TaskObject, error) {
		return nil, errors.New("something went wrong")
	}
	_, _, err := GetTaskData(context.Background(), tr, gt)
	if err == nil {
		t.Fatalf("Expected error when unable to find referenced Task but got none")
	}
}

func TestGetTaskData_ResolutionSuccess(t *testing.T) {
	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mytaskrun",
		},
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				ResolverRef: v1beta1.ResolverRef{
					Resolver: "foo",
					Params: []v1beta1.ResolverParam{{
						Name:  "bar",
						Value: "baz",
					}},
				},
			},
		},
	}
	sourceMeta := metav1.ObjectMeta{
		Name: "task",
	}
	sourceSpec := v1beta1.TaskSpec{
		Steps: []v1beta1.Step{{
			Name:   "step1",
			Image:  "ubuntu",
			Script: `echo "hello world!"`,
		}},
	}
	getTask := func(ctx context.Context, n string) (v1beta1.TaskObject, error) {
		return &v1beta1.Task{
			ObjectMeta: *sourceMeta.DeepCopy(),
			Spec:       *sourceSpec.DeepCopy(),
		}, nil
	}
	// Enable alpha fields for remote resolution
	ctx := context.Background()
	cfg := config.FromContextOrDefaults(ctx)
	cfg.FeatureFlags.EnableAPIFields = config.AlphaAPIFields
	ctx = config.ToContext(ctx, cfg)
	resolvedMeta, resolvedSpec, err := GetTaskData(ctx, tr, getTask)
	if err != nil {
		t.Fatalf("Unexpected error getting mocked data: %v", err)
	}
	if sourceMeta.Name != resolvedMeta.Name {
		t.Errorf("Expected name %q but resolved to %q", sourceMeta.Name, resolvedMeta.Name)
	}
	if d := cmp.Diff(sourceSpec, *resolvedSpec); d != "" {
		t.Errorf(diff.PrintWantGot(d))
	}
}

func TestGetPipelineData_ResolutionError(t *testing.T) {
	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mytaskrun",
		},
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				ResolverRef: v1beta1.ResolverRef{
					Resolver: "git",
				},
			},
		},
	}
	getTask := func(ctx context.Context, n string) (v1beta1.TaskObject, error) {
		return nil, errors.New("something went wrong")
	}
	// Enable alpha fields for remote resolution
	ctx := context.Background()
	cfg := config.FromContextOrDefaults(ctx)
	cfg.FeatureFlags.EnableAPIFields = config.AlphaAPIFields
	ctx = config.ToContext(ctx, cfg)
	_, _, err := GetTaskData(ctx, tr, getTask)
	if err == nil {
		t.Fatalf("Expected error when unable to find referenced Task but got none")
	}
}

func TestGetTaskData_ResolvedNilTask(t *testing.T) {
	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mytaskrun",
		},
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				ResolverRef: v1beta1.ResolverRef{
					Resolver: "git",
				},
			},
		},
	}
	getTask := func(ctx context.Context, n string) (v1beta1.TaskObject, error) {
		return nil, nil
	}
	// Enable alpha fields for remote resolution
	ctx := context.Background()
	cfg := config.FromContextOrDefaults(ctx)
	cfg.FeatureFlags.EnableAPIFields = config.AlphaAPIFields
	ctx = config.ToContext(ctx, cfg)
	_, _, err := GetTaskData(ctx, tr, getTask)
	if err == nil {
		t.Fatalf("Expected error when unable to find referenced Task but got none")
	}
}
