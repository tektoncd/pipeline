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
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resource"
	"github.com/tektoncd/pipeline/pkg/trustedresources"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/parse"
	test "github.com/tektoncd/pipeline/test/remoteresolution"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

var refSourceSample = &v1.RefSource{
	URI: "bundle.tekton.dev/my-task",
	Digest: map[string]string{
		"sha256": "a1b2c3d4",
	},
	EntryPoint: "path/to/my/task.yaml",
}

func TestGetTaskSpec_Ref(t *testing.T) {
	task := &v1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name: "orchestrate",
		},
		Spec: v1.TaskSpec{
			Steps: []v1.Step{{
				Name: "step1",
			}},
		},
	}
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mytaskrun",
		},
		Spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{
				Name: "orchestrate",
			},
		},
	}

	gt := func(ctx context.Context, n string) (*v1.Task, *v1.RefSource, *trustedresources.VerificationResult, error) {
		return task, refSourceSample.DeepCopy(), nil, nil
	}
	resolvedObjectMeta, taskSpec, err := GetTaskData(t.Context(), tr, gt)
	if err != nil {
		t.Fatalf("Did not expect error getting task spec but got: %s", err)
	}

	if resolvedObjectMeta.Name != "orchestrate" {
		t.Errorf("Expected task name to be `orchestrate` but was %q", resolvedObjectMeta.Name)
	}

	if len(taskSpec.Steps) != 1 || taskSpec.Steps[0].Name != "step1" {
		t.Errorf("Task Spec not resolved as expected, expected referenced Task spec but got: %v", taskSpec)
	}
	if d := cmp.Diff(refSourceSample, resolvedObjectMeta.RefSource); d != "" {
		t.Errorf("refSource did not match: %s", diff.PrintWantGot(d))
	}
}

func TestGetTaskSpec_Embedded(t *testing.T) {
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mytaskrun",
		},
		Spec: v1.TaskRunSpec{
			TaskSpec: &v1.TaskSpec{
				Steps: []v1.Step{{
					Name: "step1",
				}},
			},
		},
	}
	gt := func(ctx context.Context, n string) (*v1.Task, *v1.RefSource, *trustedresources.VerificationResult, error) {
		return nil, nil, nil, errors.New("shouldn't be called")
	}
	resolvedObjectMeta, taskSpec, err := GetTaskData(t.Context(), tr, gt)
	if err != nil {
		t.Fatalf("Did not expect error getting task spec but got: %s", err)
	}

	if resolvedObjectMeta.Name != "mytaskrun" {
		t.Errorf("Expected task name for embedded task to default to name of task run but was %q", resolvedObjectMeta.Name)
	}

	if len(taskSpec.Steps) != 1 || taskSpec.Steps[0].Name != "step1" {
		t.Errorf("Task Spec not resolved as expected, expected embedded Task spec but got: %v", taskSpec)
	}

	// embedded tasks have empty RefSource for now. This may be changed in future.
	if resolvedObjectMeta.RefSource != nil {
		t.Errorf("resolved refSource for embedded task is expected to be empty, but got %v", resolvedObjectMeta.RefSource)
	}
}

func TestGetTaskSpec_Invalid(t *testing.T) {
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mytaskrun",
		},
	}
	gt := func(ctx context.Context, n string) (*v1.Task, *v1.RefSource, *trustedresources.VerificationResult, error) {
		return nil, nil, nil, errors.New("shouldn't be called")
	}
	_, _, err := GetTaskData(t.Context(), tr, gt)
	if err == nil {
		t.Fatalf("Expected error resolving spec with no embedded or referenced task spec but didn't get error")
	}
}

func TestGetTaskSpec_Error(t *testing.T) {
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mytaskrun",
		},
		Spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{
				Name: "orchestrate",
			},
		},
	}
	gt := func(ctx context.Context, n string) (*v1.Task, *v1.RefSource, *trustedresources.VerificationResult, error) {
		return nil, nil, nil, errors.New("something went wrong")
	}
	_, _, err := GetTaskData(t.Context(), tr, gt)
	if err == nil {
		t.Fatalf("Expected error when unable to find referenced Task but got none")
	}
}

func TestGetTaskData_ResolutionSuccess(t *testing.T) {
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mytaskrun",
		},
		Spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{
				ResolverRef: v1.ResolverRef{
					Resolver: "foo",
					Params: []v1.Param{{
						Name: "bar",
						Value: v1.ParamValue{
							Type:      v1.ParamTypeString,
							StringVal: "baz",
						},
					}},
				},
			},
		},
	}
	sourceMeta := metav1.ObjectMeta{
		Name: "task",
	}
	sourceSpec := v1.TaskSpec{
		Steps: []v1.Step{{
			Name:   "step1",
			Image:  "ubuntu",
			Script: `echo "hello world!"`,
		}},
	}

	getTask := func(ctx context.Context, n string) (*v1.Task, *v1.RefSource, *trustedresources.VerificationResult, error) {
		return &v1.Task{
			ObjectMeta: *sourceMeta.DeepCopy(),
			Spec:       *sourceSpec.DeepCopy(),
		}, refSourceSample.DeepCopy(), nil, nil
	}
	ctx := t.Context()
	resolvedMeta, resolvedSpec, err := GetTaskData(ctx, tr, getTask)
	if err != nil {
		t.Fatalf("Unexpected error getting mocked data: %v", err)
	}
	if sourceMeta.Name != resolvedMeta.Name {
		t.Errorf("Expected name %q but resolved to %q", sourceMeta.Name, resolvedMeta.Name)
	}

	if d := cmp.Diff(refSourceSample, resolvedMeta.RefSource); d != "" {
		t.Errorf("refSource did not match: %s", diff.PrintWantGot(d))
	}

	if d := cmp.Diff(sourceSpec, *resolvedSpec); d != "" {
		t.Error(diff.PrintWantGot(d))
	}
}

func TestGetPipelineData_ResolutionError(t *testing.T) {
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mytaskrun",
		},
		Spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{
				ResolverRef: v1.ResolverRef{
					Resolver: "git",
				},
			},
		},
	}
	getTask := func(ctx context.Context, n string) (*v1.Task, *v1.RefSource, *trustedresources.VerificationResult, error) {
		return nil, nil, nil, errors.New("something went wrong")
	}
	ctx := t.Context()
	_, _, err := GetTaskData(ctx, tr, getTask)
	if err == nil {
		t.Fatalf("Expected error when unable to find referenced Task but got none")
	}
}

func TestGetTaskData_ResolvedNilTask(t *testing.T) {
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mytaskrun",
		},
		Spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{
				ResolverRef: v1.ResolverRef{
					Resolver: "git",
				},
			},
		},
	}
	getTask := func(ctx context.Context, n string) (*v1.Task, *v1.RefSource, *trustedresources.VerificationResult, error) {
		return nil, nil, nil, nil
	}
	ctx := t.Context()
	_, _, err := GetTaskData(ctx, tr, getTask)
	if err == nil {
		t.Fatalf("Expected error when unable to find referenced Task but got none")
	}
}

func TestGetTaskData_VerificationResult(t *testing.T) {
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mytaskrun",
		},
		Spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{
				ResolverRef: v1.ResolverRef{
					Resolver: "foo",
					Params: v1.Params{{
						Name: "bar",
						Value: v1.ParamValue{
							Type:      v1.ParamTypeString,
							StringVal: "baz",
						},
					}},
				},
			},
		},
	}
	sourceMeta := metav1.ObjectMeta{
		Name: "task",
	}
	sourceSpec := v1.TaskSpec{
		Steps: []v1.Step{{
			Name:   "step1",
			Image:  "ubuntu",
			Script: `echo "hello world!"`,
		}},
	}

	verificationResult := &trustedresources.VerificationResult{
		VerificationResultType: trustedresources.VerificationError,
		Err:                    trustedresources.ErrResourceVerificationFailed,
	}
	getTask := func(ctx context.Context, n string) (*v1.Task, *v1.RefSource, *trustedresources.VerificationResult, error) {
		return &v1.Task{
			ObjectMeta: *sourceMeta.DeepCopy(),
			Spec:       *sourceSpec.DeepCopy(),
		}, nil, verificationResult, nil
	}
	r, _, err := GetTaskData(t.Context(), tr, getTask)
	if err != nil {
		t.Fatalf("Did not expect error but got: %s", err)
	}
	if d := cmp.Diff(verificationResult, r.VerificationResult, cmpopts.EquateErrors()); d != "" {
		t.Error(diff.PrintWantGot(d))
	}
}

func TestHasStepRefs(t *testing.T) {
	testCases := []struct {
		name     string
		spec     *v1.TaskSpec
		expected bool
	}{
		{
			name: "no steps",
			spec: &v1.TaskSpec{},
		},
		{
			name: "single step without ref",
			spec: &v1.TaskSpec{
				Steps: []v1.Step{
					{Name: "step-1"},
				},
			},
		},
		{
			name: "multiple steps without ref",
			spec: &v1.TaskSpec{
				Steps: []v1.Step{
					{Name: "step-1"},
					{Name: "step-2"},
				},
			},
		},
		{
			name: "single step with ref",
			spec: &v1.TaskSpec{
				Steps: []v1.Step{
					{Ref: &v1.Ref{Name: "step-action"}},
				},
			},
			expected: true,
		},
		{
			name: "multiple steps with one ref",
			spec: &v1.TaskSpec{
				Steps: []v1.Step{
					{Name: "step-1"},
					{Ref: &v1.Ref{Name: "step-action"}},
				},
			},
			expected: true,
		},
		{
			name: "multiple steps with multiple refs",
			spec: &v1.TaskSpec{
				Steps: []v1.Step{
					{Name: "inline-step-1"},
					{Ref: &v1.Ref{Name: "step-action-1"}},
					{Name: "inline-step-2"},
					{Ref: &v1.Ref{Name: "step-action-2"}},
					{Name: "inline-step-3"},
				},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasStepRefs(tc.spec); got != tc.expected {
				t.Errorf("hasStepRefs() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestGetStepActionsData_Provenance(t *testing.T) {
	source := v1.RefSource{
		URI:    "ref-source",
		Digest: map[string]string{"sha256": "abcd123456"},
	}
	stepAction := parse.MustParseV1beta1StepAction(t, `
metadata:
  name: stepAction
  namespace: foo
spec:
  image: myImage
  command: ["ls"]
`)

	stepActionBytes, err := yaml.Marshal(stepAction)
	if err != nil {
		t.Fatal("failed to marshal StepAction", err)
	}
	rr := test.NewResolvedResource(stepActionBytes, map[string]string{}, &source, nil)
	requester := test.NewRequester(rr, nil, resource.ResolverPayload{})
	tests := []struct {
		name string
		tr   *v1.TaskRun
		want *v1.TaskRun
	}{{
		name: "remote-step-action-with-provenance",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Name: "stepname",
						Ref: &v1.Ref{
							ResolverRef: v1.ResolverRef{
								Resolver: "foo",
								Params: []v1.Param{{
									Name: "bar",
									Value: v1.ParamValue{
										Type:      v1.ParamTypeString,
										StringVal: "baz",
									},
								}},
							},
						},
					}},
				},
			},
		},
		want: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Name: "stepname",
						Ref: &v1.Ref{
							ResolverRef: v1.ResolverRef{
								Resolver: "foo",
								Params: []v1.Param{{
									Name: "bar",
									Value: v1.ParamValue{
										Type:      v1.ParamTypeString,
										StringVal: "baz",
									},
								}},
							},
						},
					}},
				},
			},
			Status: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Steps: []v1.StepState{{
						Name: "stepname",
						Provenance: &v1.Provenance{
							RefSource: &source,
						},
					}},
				},
			},
		},
	}, {
		name: "multiple-remote-step-actions-with-provenance",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Name: "step1",
						Ref: &v1.Ref{
							ResolverRef: v1.ResolverRef{
								Resolver: "foo",
								Params: []v1.Param{{
									Name: "bar",
									Value: v1.ParamValue{
										Type:      v1.ParamTypeString,
										StringVal: "baz",
									},
								}},
							},
						},
					}, {
						Name: "step2",
						Ref: &v1.Ref{
							ResolverRef: v1.ResolverRef{
								Resolver: "foo",
								Params: []v1.Param{{
									Name: "bar",
									Value: v1.ParamValue{
										Type:      v1.ParamTypeString,
										StringVal: "baz",
									},
								}},
							},
						},
					}},
				},
			},
		},
		want: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Name: "step1",
						Ref: &v1.Ref{
							ResolverRef: v1.ResolverRef{
								Resolver: "foo",
								Params: []v1.Param{{
									Name: "bar",
									Value: v1.ParamValue{
										Type:      v1.ParamTypeString,
										StringVal: "baz",
									},
								}},
							},
						},
					}, {
						Name: "step2",
						Ref: &v1.Ref{
							ResolverRef: v1.ResolverRef{
								Resolver: "foo",
								Params: []v1.Param{{
									Name: "bar",
									Value: v1.ParamValue{
										Type:      v1.ParamTypeString,
										StringVal: "baz",
									},
								}},
							},
						},
					}},
				},
			},
			Status: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Steps: []v1.StepState{{
						Name: "step1",
						Provenance: &v1.Provenance{
							RefSource: &source,
						},
					}, {
						Name: "step2",
						Provenance: &v1.Provenance{
							RefSource: &source,
						},
					}},
				},
			},
		},
	}, {
		name: "remote-step-action-with-existing-provenance",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Name: "step1",
						Ref: &v1.Ref{
							ResolverRef: v1.ResolverRef{
								Resolver: "foo",
								Params: []v1.Param{{
									Name: "bar",
									Value: v1.ParamValue{
										Type:      v1.ParamTypeString,
										StringVal: "baz",
									},
								}},
							},
						},
					}},
				},
			},
			Status: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Steps: []v1.StepState{{
						Name: "step1",
						Provenance: &v1.Provenance{
							RefSource: &source,
						},
					}},
				},
			},
		},
		want: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Name: "step1",
						Ref: &v1.Ref{
							ResolverRef: v1.ResolverRef{
								Resolver: "foo",
								Params: []v1.Param{{
									Name: "bar",
									Value: v1.ParamValue{
										Type:      v1.ParamTypeString,
										StringVal: "baz",
									},
								}},
							},
						},
					}},
				},
			},
			Status: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Steps: []v1.StepState{{
						Name: "step1",
						Provenance: &v1.Provenance{
							RefSource: &source,
						},
					}},
				},
			},
		},
	}, {
		name: "remote-step-action-with-missing-provenance",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Name: "step1",
						Ref: &v1.Ref{
							ResolverRef: v1.ResolverRef{
								Resolver: "foo",
								Params: []v1.Param{{
									Name: "bar",
									Value: v1.ParamValue{
										Type:      v1.ParamTypeString,
										StringVal: "baz",
									},
								}},
							},
						},
					}},
				},
			},
			Status: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Steps: []v1.StepState{{
						Name: "step1",
					}},
				},
			},
		},
		want: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Name: "step1",
						Ref: &v1.Ref{
							ResolverRef: v1.ResolverRef{
								Resolver: "foo",
								Params: []v1.Param{{
									Name: "bar",
									Value: v1.ParamValue{
										Type:      v1.ParamTypeString,
										StringVal: "baz",
									},
								}},
							},
						},
					}},
				},
			},
			Status: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Steps: []v1.StepState{{
						Name: "step1",
						Provenance: &v1.Provenance{
							RefSource: &source,
						},
					}},
				},
			},
		},
	}, {
		name: "no-remote-step-action",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Name: "step1",
					}},
				},
			},
			Status: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Steps: []v1.StepState{{
						Name: "step1",
					}},
				},
			},
		},
		want: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Name: "step1",
					}},
				},
			},
			Status: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Steps: []v1.StepState{{
						Name:       "step1",
						Provenance: &v1.Provenance{},
					}},
				},
			},
		},
	}}
	for _, tt := range tests {
		ctx := t.Context()
		tektonclient := fake.NewSimpleClientset(stepAction)
		_, err := GetStepActionsData(ctx, *tt.tr.Spec.TaskSpec, tt.tr, tektonclient, nil, requester)
		if err != nil {
			t.Fatalf("Did not expect an error but got : %s", err)
		}
		if d := cmp.Diff(tt.want, tt.tr); d != "" {
			t.Errorf("the taskrun did not match what was expected diff: %s", diff.PrintWantGot(d))
		}
	}
}

func TestGetStepActionsData_Status(t *testing.T) {
	firstStepAction := parse.MustParseV1beta1StepAction(t, `
metadata:
  name: first-stepaction
  namespace: default
spec:
  image: myImage
`)
	firstStepActionSource := v1.RefSource{
		URI:    "ref-source",
		Digest: map[string]string{"sha256": "abcd123456"},
	}

	firstStepActionBytes, err := yaml.Marshal(firstStepAction)
	if err != nil {
		t.Fatal("failed to marshal StepAction", err)
	}
	rr := test.NewResolvedResource(firstStepActionBytes, map[string]string{}, &firstStepActionSource, nil)
	requester := test.NewRequester(rr, nil, resource.ResolverPayload{})

	tests := []struct {
		name        string
		tr          *v1.TaskRun
		stepActions []*v1beta1.StepAction
		want        v1.TaskRunStatus
	}{
		{
			name: "inline only",
			tr: &v1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mytaskrun",
					Namespace: "default",
				},
				Spec: v1.TaskRunSpec{
					TaskSpec: &v1.TaskSpec{
						Steps: []v1.Step{
							{
								Name:  "first-inline",
								Image: "ubuntu",
							},
							{
								Name:  "second-inline",
								Image: "ubuntu",
							},
						},
					},
				},
			},
			want: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Steps: []v1.StepState{
						{
							Name:       "first-inline",
							Provenance: &v1.Provenance{},
						},
						{
							Name:       "second-inline",
							Provenance: &v1.Provenance{},
						},
					},
				},
			},
		}, {
			name: "StepAction only",
			tr: &v1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mytaskrun",
					Namespace: "default",
				},
				Spec: v1.TaskRunSpec{
					TaskSpec: &v1.TaskSpec{
						Steps: []v1.Step{
							{
								Name: "first-remote",
								Ref: &v1.Ref{
									Name: "first-stepaction",
									ResolverRef: v1.ResolverRef{
										Resolver: "foobar",
									},
								},
							},
							{
								Name: "second-remote",
								Ref: &v1.Ref{
									Name: "second-stepaction",
								},
							},
						},
					},
				},
			},
			stepActions: []*v1beta1.StepAction{
				firstStepAction,
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "second-stepaction",
						Namespace: "default",
					},
					Spec: v1beta1.StepActionSpec{
						Image: "myimage",
					},
				},
			},
			want: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Steps: []v1.StepState{
						{
							Name: "first-remote",
							Provenance: &v1.Provenance{
								RefSource: &v1.RefSource{
									URI:    "ref-source",
									Digest: map[string]string{"sha256": "abcd123456"},
								},
							},
						},
						{
							Name:       "second-remote",
							Provenance: &v1.Provenance{},
						},
					},
				},
			},
		},
		{
			name: "mixed inline and StepAction",
			tr: &v1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mytaskrun",
					Namespace: "default",
				},
				Spec: v1.TaskRunSpec{
					TaskSpec: &v1.TaskSpec{
						Steps: []v1.Step{
							{
								Name:  "first-inline",
								Image: "ubuntu",
							},
							{
								Name: "second-remote",
								Ref: &v1.Ref{
									Name: "first-stepaction",
									ResolverRef: v1.ResolverRef{
										Resolver: "foobar",
									},
								},
							},
							{
								Name:  "third-inline",
								Image: "ubuntu",
							},
							{
								Name: "fourth-remote",
								Ref: &v1.Ref{
									Name: "second-stepaction",
								},
							},
						},
					},
				},
			},
			stepActions: []*v1beta1.StepAction{
				firstStepAction,
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "second-stepaction",
						Namespace: "default",
					},
					Spec: v1beta1.StepActionSpec{
						Image: "myimage",
					},
				},
			},
			want: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Steps: []v1.StepState{
						{
							Name:       "first-inline",
							Provenance: &v1.Provenance{},
						},
						{
							Name: "second-remote",
							Provenance: &v1.Provenance{
								RefSource: &v1.RefSource{
									URI:    "ref-source",
									Digest: map[string]string{"sha256": "abcd123456"},
								},
							},
						},
						{
							Name:       "third-inline",
							Provenance: &v1.Provenance{},
						},
						{
							Name:       "fourth-remote",
							Provenance: &v1.Provenance{},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			tektonclient := fake.NewSimpleClientset()
			for _, sa := range tt.stepActions {
				if err := tektonclient.Tracker().Add(sa); err != nil {
					t.Fatal(err)
				}
			}

			_, err := GetStepActionsData(ctx, *tt.tr.Spec.TaskSpec, tt.tr, tektonclient, nil, requester)
			if err != nil {
				t.Fatalf("Did not expect an error but got : %s", err)
			}
			if d := cmp.Diff(tt.want, tt.tr.Status); d != "" {
				t.Errorf("the taskrun status did not match what was expected diff: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetStepActionsData(t *testing.T) {
	taskRunUser := int64(1001)
	stepActionUser := int64(1000)
	tests := []struct {
		name        string
		tr          *v1.TaskRun
		stepActions []*v1beta1.StepAction
		want        []v1.Step
	}{{
		name: "step-action-with-command-args",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Ref: &v1.Ref{
							Name: "stepAction",
						},
						Timeout: &metav1.Duration{Duration: time.Hour},
					}},
				},
			},
		},
		stepActions: []*v1beta1.StepAction{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepAction",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image:   "myimage",
				Command: []string{"ls"},
				Args:    []string{"-lh"},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "$(params.foo)",
					MountPath: "/path",
				}},
			},
		}},
		want: []v1.Step{{
			Image:   "myimage",
			Command: []string{"ls"},
			Args:    []string{"-lh"},
			Timeout: &metav1.Duration{Duration: time.Hour},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "$(params.foo)",
				MountPath: "/path",
			}},
		}},
	}, {
		name: "step-action-with-script",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Ref: &v1.Ref{
							Name: "stepActionWithScript",
						},
					}},
				},
			},
		},
		stepActions: []*v1beta1.StepAction{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepActionWithScript",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image:  "myimage",
				Script: "ls",
			},
		}},
		want: []v1.Step{{
			Image:  "myimage",
			Script: "ls",
		}},
	}, {
		name: "step-action-with-env",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Ref: &v1.Ref{
							Name: "stepActionWithEnv",
						},
					}},
				},
			},
		},
		stepActions: []*v1beta1.StepAction{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepActionWithEnv",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image: "myimage",
				Env: []corev1.EnvVar{{
					Name:  "env1",
					Value: "value1",
				}},
			},
		}},
		want: []v1.Step{{
			Image: "myimage",
			Env: []corev1.EnvVar{{
				Name:  "env1",
				Value: "value1",
			}},
		}},
	}, {
		name: "step-action-with-step-result",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Ref: &v1.Ref{
							Name: "stepActionWithScript",
						},
					}},
				},
			},
		},
		stepActions: []*v1beta1.StepAction{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepActionWithScript",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image:  "myimage",
				Script: "ls",
				Results: []v1.StepResult{{
					Name: "foo",
				}},
			},
		}},
		want: []v1.Step{{
			Image:   "myimage",
			Script:  "ls",
			Results: []v1.StepResult{{Name: "foo", Type: "string"}},
		}},
	}, {
		name: "inline and ref StepAction",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Ref: &v1.Ref{
							Name: "stepAction",
						},
						Timeout: &metav1.Duration{Duration: time.Hour},
					}, {
						Image:   "foo",
						Command: []string{"ls"},
					}},
				},
			},
		},
		stepActions: []*v1beta1.StepAction{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepAction",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image:   "myimage",
				Command: []string{"ls"},
				Args:    []string{"-lh"},
			},
		}},
		want: []v1.Step{{
			Image:   "myimage",
			Command: []string{"ls"},
			Args:    []string{"-lh"},
			Timeout: &metav1.Duration{Duration: time.Hour},
		}, {
			Image:   "foo",
			Command: []string{"ls"},
		}},
	}, {
		name: "multiple ref StepActions",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Ref: &v1.Ref{
							Name: "stepAction1",
						},
						Timeout: &metav1.Duration{Duration: time.Hour},
					}, {
						Ref: &v1.Ref{
							Name: "stepAction2",
						},
					}},
				},
			},
		},
		stepActions: []*v1beta1.StepAction{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepAction1",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image:   "myimage1",
				Command: []string{"ls"},
				Args:    []string{"-l"},
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepAction2",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image:  "myimage2",
				Script: "echo hello",
			},
		}},
		want: []v1.Step{{
			Image:   "myimage1",
			Command: []string{"ls"},
			Args:    []string{"-l"},
			Timeout: &metav1.Duration{Duration: time.Hour},
		}, {
			Image:  "myimage2",
			Script: "echo hello",
		}},
	}, {
		name: "step-action-with-security-context-overwritten",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Ref: &v1.Ref{
							Name: "stepAction",
						},
						SecurityContext: &corev1.SecurityContext{RunAsUser: &taskRunUser},
					}},
				},
			},
		},
		stepActions: []*v1beta1.StepAction{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepAction",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image:           "myimage",
				Command:         []string{"ls"},
				Args:            []string{"-lh"},
				SecurityContext: &corev1.SecurityContext{RunAsUser: &stepActionUser},
			},
		}},
		want: []v1.Step{{
			Image:           "myimage",
			Command:         []string{"ls"},
			Args:            []string{"-lh"},
			SecurityContext: &corev1.SecurityContext{RunAsUser: &stepActionUser},
		}},
	}, {
		name: "params propagated from taskrun",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				Params: v1.Params{{
					Name: "stringparam",
					Value: v1.ParamValue{
						Type:      v1.ParamTypeString,
						StringVal: "taskrun string param",
					},
				}, {
					Name: "arrayparam",
					Value: v1.ParamValue{
						Type:     v1.ParamTypeArray,
						ArrayVal: []string{"taskrun", "array", "param"},
					},
				}, {
					Name: "objectparam",
					Value: v1.ParamValue{
						Type:      v1.ParamTypeObject,
						ObjectVal: map[string]string{"key": "taskrun object param"},
					},
				}},
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Ref: &v1.Ref{
							Name: "stepAction",
						},
						Params: v1.Params{{
							Name:  "string-param",
							Value: *v1.NewStructuredValues("$(params.stringparam)"),
						}, {
							Name:  "array-param",
							Value: *v1.NewStructuredValues("$(params.arrayparam[*])"),
						}, {
							Name:  "object-param",
							Value: *v1.NewStructuredValues("$(params.objectparam[*])"),
						}},
					}},
				},
			},
		},
		stepActions: []*v1beta1.StepAction{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepAction",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image: "myimage",
				Args:  []string{"$(params.string-param)", "$(params.array-param[0])", "$(params.array-param[1])", "$(params.array-param[*])", "$(params.object-param.key)"},
				Params: v1.ParamSpecs{{
					Name: "string-param",
					Type: v1.ParamTypeString,
				}, {
					Name: "array-param",
					Type: v1.ParamTypeArray,
				}, {
					Name:       "object-param",
					Type:       v1.ParamTypeObject,
					Properties: map[string]v1.PropertySpec{"key": {Type: "string"}},
				}},
			},
		}},
		want: []v1.Step{{
			Image: "myimage",
			Args:  []string{"taskrun string param", "taskrun", "array", "taskrun", "array", "param", "taskrun object param"},
		}},
	}, {
		name: "params propagated from taskspec",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Params: v1.ParamSpecs{{
						Name: "stringparam",
						Default: &v1.ParamValue{
							Type:      v1.ParamTypeString,
							StringVal: "taskspec string param",
						},
					}, {
						Name: "arrayparam",
						Default: &v1.ParamValue{
							Type:     v1.ParamTypeArray,
							ArrayVal: []string{"taskspec", "array", "param"},
						},
					}, {
						Name: "objectparam",
						Default: &v1.ParamValue{
							Type:      v1.ParamTypeObject,
							ObjectVal: map[string]string{"key": "taskspec object param"},
						},
					}},
					Steps: []v1.Step{{
						Ref: &v1.Ref{
							Name: "stepAction",
						},
						Params: v1.Params{{
							Name:  "string-param",
							Value: *v1.NewStructuredValues("$(params.stringparam)"),
						}, {
							Name:  "array-param",
							Value: *v1.NewStructuredValues("$(params.arrayparam[*])"),
						}, {
							Name:  "object-param",
							Value: *v1.NewStructuredValues("$(params.objectparam[*])"),
						}},
					}},
				},
			},
		},
		stepActions: []*v1beta1.StepAction{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepAction",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image: "myimage",
				Args:  []string{"$(params.string-param)", "$(params.array-param[0])", "$(params.array-param[1])", "$(params.array-param[*])", "$(params.object-param.key)"},
				Params: v1.ParamSpecs{{
					Name: "string-param",
					Type: v1.ParamTypeString,
				}, {
					Name: "array-param",
					Type: v1.ParamTypeArray,
				}, {
					Name:       "object-param",
					Type:       v1.ParamTypeObject,
					Properties: map[string]v1.PropertySpec{"key": {Type: "string"}},
				}},
			},
		}},
		want: []v1.Step{{
			Image: "myimage",
			Args:  []string{"taskspec string param", "taskspec", "array", "taskspec", "array", "param", "taskspec object param"},
		}},
	}, {
		name: "params from step action defaults",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Ref: &v1.Ref{
							Name: "stepAction",
						},
					}},
				},
			},
		},
		stepActions: []*v1beta1.StepAction{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepAction",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image: "myimage",
				Args:  []string{"$(params.string-param)", "$(params.array-param[0])", "$(params.array-param[1])", "$(params.array-param[*])", "$(params.object-param.key)"},
				Params: v1.ParamSpecs{{
					Name: "string-param",
					Type: v1.ParamTypeString,
					Default: &v1.ParamValue{
						Type:      v1.ParamTypeString,
						StringVal: "step action string param",
					},
				}, {
					Name: "array-param",
					Type: v1.ParamTypeArray,
					Default: &v1.ParamValue{
						Type:     v1.ParamTypeArray,
						ArrayVal: []string{"step action", "array", "param"},
					},
				}, {
					Name:       "object-param",
					Type:       v1.ParamTypeObject,
					Properties: map[string]v1.PropertySpec{"key": {Type: "string"}},
					Default: &v1.ParamValue{
						Type:      v1.ParamTypeObject,
						ObjectVal: map[string]string{"key": "step action object param"},
					},
				}},
			},
		}},
		want: []v1.Step{{
			Image: "myimage",
			Args:  []string{"step action string param", "step action", "array", "step action", "array", "param", "step action object param"},
		}},
	}, {
		name: "params propagated partially from taskrun taskspec and stepaction",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				Params: v1.Params{{
					Name: "stringparam",
					Value: v1.ParamValue{
						Type:      v1.ParamTypeString,
						StringVal: "taskrun string param",
					},
				}, {
					Name: "objectparam",
					Value: v1.ParamValue{
						Type:      v1.ParamTypeObject,
						ObjectVal: map[string]string{"key": "taskrun key"},
					},
				}},
				TaskSpec: &v1.TaskSpec{
					Params: v1.ParamSpecs{{
						Name: "arrayparam",
						Default: &v1.ParamValue{
							Type:     v1.ParamTypeArray,
							ArrayVal: []string{"taskspec", "array", "param"},
						},
					}, {
						Name: "objectparam",
						Default: &v1.ParamValue{
							Type:      v1.ParamTypeObject,
							ObjectVal: map[string]string{"key": "key1", "key2": "taskspec key2"},
						},
					}},
					Steps: []v1.Step{{
						Ref: &v1.Ref{
							Name: "stepAction",
						},
						Params: v1.Params{{
							Name:  "string-param",
							Value: *v1.NewStructuredValues("$(params.stringparam)"),
						}, {
							Name:  "array-param",
							Value: *v1.NewStructuredValues("$(params.arrayparam[*])"),
						}, {
							Name:  "object-param",
							Value: *v1.NewStructuredValues("$(params.objectparam[*])"),
						}},
					}},
				},
			},
		},
		stepActions: []*v1beta1.StepAction{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepAction",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image: "myimage",
				Args:  []string{"$(params.string-param)", "$(params.array-param[0])", "$(params.array-param[1])", "$(params.array-param[*])", "$(params.object-param.key)", "$(params.object-param.key2)", "$(params.object-param.key3)"},
				Params: v1.ParamSpecs{{
					Name: "string-param",
					Type: v1.ParamTypeString,
					Default: &v1.ParamValue{
						Type:      v1.ParamTypeString,
						StringVal: "step action string param",
					},
				}, {
					Name: "array-param",
					Type: v1.ParamTypeArray,
					Default: &v1.ParamValue{
						Type:     v1.ParamTypeArray,
						ArrayVal: []string{"step action", "array", "param"},
					},
				}, {
					Name:       "object-param",
					Type:       v1.ParamTypeObject,
					Properties: map[string]v1.PropertySpec{"key": {Type: "string"}},
					Default: &v1.ParamValue{
						Type:      v1.ParamTypeObject,
						ObjectVal: map[string]string{"key": "step action key1", "key2": "step action key2", "key3": "step action key3"},
					},
				}},
			},
		}},
		want: []v1.Step{{
			Image: "myimage",
			Args:  []string{"taskrun string param", "taskspec", "array", "taskspec", "array", "param", "taskrun key", "taskspec key2", "step action key3"},
		}},
	}, {
		name: "params in step propagated to stepaction only",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				Params: v1.Params{{
					Name: "stringparam",
					Value: v1.ParamValue{
						Type:      v1.ParamTypeString,
						StringVal: "taskrun string param",
					},
				}},
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Ref: &v1.Ref{
							Name: "stepAction",
						},
						Params: v1.Params{{
							Name: "stringparam",
							Value: v1.ParamValue{
								Type:      v1.ParamTypeString,
								StringVal: "step string param",
							},
						}},
						OnError:      v1.OnErrorType("$(params.stringparam)"),
						StdoutConfig: &v1.StepOutputConfig{Path: "$(params.stringparam)"},
						StderrConfig: &v1.StepOutputConfig{Path: "$(params.stringparam)"},
					}},
				},
			},
		},
		stepActions: []*v1beta1.StepAction{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepAction",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image: "myimage",
				Args:  []string{"echo", "$(params.stringparam)"},
				Params: v1.ParamSpecs{{
					Name: "stringparam",
					Type: v1.ParamTypeString,
				}},
			},
		}},
		want: []v1.Step{{
			Image:        "myimage",
			Args:         []string{"echo", "step string param"},
			OnError:      v1.OnErrorType("$(params.stringparam)"),
			StdoutConfig: &v1.StepOutputConfig{Path: "$(params.stringparam)"},
			StderrConfig: &v1.StepOutputConfig{Path: "$(params.stringparam)"},
		}},
	}, {
		name: "propagating step result substitution strings into step actions",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Name:  "inlined-step",
						Image: "ubuntu",
						Results: []v1.StepResult{{
							Name: "result1",
						}, {
							Name: "result2",
							Type: v1.ResultsTypeArray,
						}, {
							Name: "result3",
							Type: v1.ResultsTypeObject,
						}},
					}, {
						Ref: &v1.Ref{
							Name: "stepAction",
						},
						Params: v1.Params{{
							Name:  "string-param",
							Value: *v1.NewStructuredValues("$(steps.inlined-step.results.result1)"),
						}, {
							Name:  "array-param",
							Value: *v1.NewStructuredValues("$(steps.inlined-step.results.result2[*])"),
						}, {
							Name:  "object-param",
							Value: *v1.NewStructuredValues("$(steps.inlined-step.results.result3[*])"),
						}},
					}},
				},
			},
		},
		stepActions: []*v1beta1.StepAction{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepAction",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image:   "myimage",
				Args:    []string{"$(params.string-param)", "$(params.array-param[0])", "$(params.array-param[1])", "$(params.array-param[*])", "$(params.object-param.key)"},
				Command: []string{"$(params[\"string-param\"])", "$(params[\"array-param\"][0])"},
				Env: []corev1.EnvVar{{
					Name:  "env1",
					Value: "$(params['array-param'][0])",
				}, {
					Name:  "env2",
					Value: "$(params['string-param'])",
				}},
				Params: v1.ParamSpecs{{
					Name: "string-param",
					Type: v1.ParamTypeString,
				}, {
					Name: "array-param",
					Type: v1.ParamTypeArray,
				}, {
					Name:       "object-param",
					Type:       v1.ParamTypeObject,
					Properties: map[string]v1.PropertySpec{"key": {Type: "string"}},
				}},
			},
		}},
		want: []v1.Step{{
			Name:  "inlined-step",
			Image: "ubuntu",
			Results: []v1.StepResult{{
				Name: "result1",
			}, {
				Name: "result2",
				Type: v1.ResultsTypeArray,
			}, {
				Name: "result3",
				Type: v1.ResultsTypeObject,
			}},
		}, {
			Image:   "myimage",
			Args:    []string{"$(steps.inlined-step.results.result1)", "$(steps.inlined-step.results.result2[0])", "$(steps.inlined-step.results.result2[1])", "$(steps.inlined-step.results.result2[*])", "$(steps.inlined-step.results.result3.key)"},
			Command: []string{"$(steps.inlined-step.results.result1)", "$(steps.inlined-step.results.result2[0])"},
			Env: []corev1.EnvVar{{
				Name:  "env1",
				Value: "$(steps.inlined-step.results.result2[0])",
			}, {
				Name:  "env2",
				Value: "$(steps.inlined-step.results.result1)",
			}},
		}},
	}, {
		name: "param types are matching",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Name: "test",
						Ref:  &v1.Ref{Name: "stepAction"},
						Params: v1.Params{{
							Name: "commands",
							Value: v1.ParamValue{
								Type:     v1.ParamTypeArray,
								ArrayVal: []string{"Hello, I am of type list"},
							},
						}},
					}},
				},
			},
		},
		stepActions: []*v1beta1.StepAction{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepAction",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image:  "myimage",
				Args:   []string{"$(params.commands)"},
				Script: "echo $@",
				Params: v1.ParamSpecs{{
					Name: "commands",
					Type: v1.ParamTypeArray,
					Default: &v1.ParamValue{
						Type:     v1.ParamTypeArray,
						ArrayVal: []string{"Hello, I am the default value"},
					},
				}},
			},
		}},
		want: []v1.Step{{
			Name:   "test",
			Image:  "myimage",
			Args:   []string{"Hello, I am of type list"},
			Script: "echo $@",
		}},
	}, {
		name: "step result reference in parameter",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Ref: &v1.Ref{
							Name: "stepAction",
						},
						Params: v1.Params{{
							Name:  "message",
							Value: *v1.NewStructuredValues("$(steps.step1.results.output)"),
						}},
					}},
				},
			},
		},
		stepActions: []*v1beta1.StepAction{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepAction",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image: "myimage",
				Args:  []string{"$(params.message)"},
				Params: v1.ParamSpecs{{
					Name: "message",
					Type: v1.ParamTypeString,
				}},
			},
		}},
		want: []v1.Step{{
			Image: "myimage",
			Args:  []string{"$(steps.step1.results.output)"},
		}},
	}, {
		name: "step result reference with array type",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Ref: &v1.Ref{
							Name: "stepAction",
						},
						Params: v1.Params{{
							Name:  "items",
							Value: *v1.NewStructuredValues("$(steps.step1.results.items)"),
						}},
					}},
				},
			},
		},
		stepActions: []*v1beta1.StepAction{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepAction",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image: "myimage",
				Args:  []string{"$(params.items[*])", "$(params.items[0])"},
				Params: v1.ParamSpecs{{
					Name: "items",
					Type: v1.ParamTypeArray,
				}},
			},
		}},
		want: []v1.Step{{
			Image: "myimage",
			Args:  []string{"$(steps.step1.results.items[*])", "$(steps.step1.results.items[0])"},
		}},
	}, {
		name: "step result reference with object type",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Ref: &v1.Ref{
							Name: "stepAction",
						},
						Params: v1.Params{{
							Name:  "config",
							Value: *v1.NewStructuredValues("$(steps.step1.results.config)"),
						}},
					}},
				},
			},
		},
		stepActions: []*v1beta1.StepAction{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepAction",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image: "myimage",
				Args:  []string{"$(params.config.key1)", "$(params.config.key2)"},
				Params: v1.ParamSpecs{{
					Name: "config",
					Type: v1.ParamTypeObject,
					Properties: map[string]v1.PropertySpec{
						"key1": {Type: v1.ParamTypeString},
						"key2": {Type: v1.ParamTypeString},
					},
				}},
			},
		}},
		want: []v1.Step{{
			Image: "myimage",
			Args:  []string{"$(steps.step1.results.config.key1)", "$(steps.step1.results.config.key2)"},
		}},
	}, {
		name: "chained parameter references in defaults",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Ref: &v1.Ref{
							Name: "stepAction",
						},
					}},
				},
			},
		},
		stepActions: []*v1beta1.StepAction{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepAction",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image: "myimage",
				Args:  []string{"$(params.param1)", "$(params.param2)", "$(params.param3)"},
				Params: v1.ParamSpecs{{
					Name: "param1",
					Type: v1.ParamTypeString,
					Default: &v1.ParamValue{
						Type:      v1.ParamTypeString,
						StringVal: "hello",
					},
				}, {
					Name: "param2",
					Type: v1.ParamTypeString,
					Default: &v1.ParamValue{
						Type:      v1.ParamTypeString,
						StringVal: "$(params.param1) world",
					},
				}, {
					Name: "param3",
					Type: v1.ParamTypeString,
					Default: &v1.ParamValue{
						Type:      v1.ParamTypeString,
						StringVal: "$(params.param2)!",
					},
				}},
			},
		}},
		want: []v1.Step{{
			Image: "myimage",
			Args:  []string{"hello", "hello world", "hello world!"},
		}},
	}, {
		name: "parameter substitution with task param reference",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				Params: v1.Params{{
					Name:  "task-param",
					Value: *v1.NewStructuredValues("task-override"),
				}},
				TaskSpec: &v1.TaskSpec{
					Params: []v1.ParamSpec{{
						Name:    "task-param",
						Type:    v1.ParamTypeString,
						Default: v1.NewStructuredValues("task-default"),
					}},
					Steps: []v1.Step{{
						Ref: &v1.Ref{
							Name: "stepAction",
						},
						Params: v1.Params{{
							Name:  "message",
							Value: *v1.NewStructuredValues("override"),
						}},
					}},
				},
			},
		},
		stepActions: []*v1beta1.StepAction{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepAction",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image: "myimage",
				Args:  []string{"$(params.message)"},
				Params: v1.ParamSpecs{{
					Name:    "message",
					Type:    v1.ParamTypeString,
					Default: v1.NewStructuredValues("$(params.task-param)"),
				}},
			},
		}},
		want: []v1.Step{{
			Image: "myimage",
			Args:  []string{"override"},
		}},
	}, {
		name: "inline only",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Image:   "foo",
						Command: []string{"ls"},
					}, {
						Image:   "bar",
						Command: []string{"ls -lh"},
					}},
				},
			},
		},
		stepActions: []*v1beta1.StepAction{},
		want: []v1.Step{{
			Image:   "foo",
			Command: []string{"ls"},
		}, {
			Image:   "bar",
			Command: []string{"ls -lh"},
		}},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			tektonclient := fake.NewSimpleClientset()
			for _, sa := range tt.stepActions {
				if err := tektonclient.Tracker().Add(sa); err != nil {
					t.Fatal(err)
				}
			}

			got, err := GetStepActionsData(ctx, *tt.tr.Spec.TaskSpec, tt.tr, tektonclient, nil, nil)
			if err != nil {
				t.Fatalf("Did not expect an error but got : %s", err)
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("the taskSpec did not match what was expected diff: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetStepActionsData_Error(t *testing.T) {
	tests := []struct {
		name          string
		tr            *v1.TaskRun
		stepAction    *v1beta1.StepAction
		expectedError error
	}{{
		name: "unresolvable parameter reference",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Ref: &v1.Ref{
							Name: "stepAction",
						},
					}},
				},
			},
		},
		stepAction: &v1beta1.StepAction{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepAction",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image: "myimage",
				Args:  []string{"$(params.param1)"},
				Params: v1.ParamSpecs{{
					Name: "param1",
					Type: v1.ParamTypeString,
					Default: &v1.ParamValue{
						Type:      v1.ParamTypeString,
						StringVal: "$(params.nonexistent)",
					},
				}},
			},
		},
		expectedError: errors.New(`failed to resolve step ref for step "" (index 0): parameter "param1" references non-existent parameter "nonexistent"`),
	}, {
		name: "circular dependency in params",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Ref: &v1.Ref{
							Name: "stepAction",
						},
					}},
				},
			},
		},
		stepAction: &v1beta1.StepAction{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepAction",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image: "myimage",
				Args:  []string{"$(params.param1)", "$(params.param2)", "$(params.param3)"},
				Params: v1.ParamSpecs{{
					Name: "param1",
					Type: v1.ParamTypeString,
					Default: &v1.ParamValue{
						Type:      v1.ParamTypeString,
						StringVal: "$(params.param3)",
					},
				}, {
					Name: "param2",
					Type: v1.ParamTypeString,
					Default: &v1.ParamValue{
						Type:      v1.ParamTypeString,
						StringVal: "$(params.param1)",
					},
				}, {
					Name: "param3",
					Type: v1.ParamTypeString,
					Default: &v1.ParamValue{
						Type:      v1.ParamTypeString,
						StringVal: "$(params.param2)",
					},
				}},
			},
		},
		expectedError: errors.New(`failed to resolve step ref for step "" (index 0): circular dependency detected in parameter references`),
	}, {
		name: "namespace missing error",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "mytaskrun",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Ref: &v1.Ref{
							Name: "stepActionError",
						},
					}},
				},
			},
		},
		stepAction:    &v1beta1.StepAction{},
		expectedError: errors.New(`failed to resolve step ref for step "" (index 0): must specify namespace to resolve reference to step action stepActionError`),
	}, {
		name: "params missing",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Ref: &v1.Ref{
							Name: "stepaction",
						},
					}},
				},
			},
		},
		stepAction: &v1beta1.StepAction{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepaction",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image: "myimage",
				Params: v1.ParamSpecs{{
					Name: "string-param",
					Type: v1.ParamTypeString,
				}},
			},
		},
		expectedError: errors.New(`failed to resolve step ref for step "" (index 0): non-existent params in Step: [string-param]`),
	}, {
		name: "params extra",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Ref: &v1.Ref{
							Name: "stepaction",
						},
						Params: v1.Params{{
							Name:  "string-param",
							Value: *v1.NewStructuredValues("$(params.stringparam)"),
						}},
					}},
				},
			},
		},
		stepAction: &v1beta1.StepAction{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepaction",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image: "myimage",
			},
		},
		expectedError: errors.New(`failed to resolve step ref for step "" (index 0): extra params passed by Step to StepAction: [string-param]`),
	}, {
		name: "param types not matching",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytaskrun",
				Namespace: "default",
			},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Steps: []v1.Step{{
						Name: "test",
						Ref:  &v1.Ref{Name: "stepAction"},
						Params: v1.Params{{
							Name:  "commands",
							Value: *v1.NewStructuredValues("Hello, I am of type string"),
						}},
					}},
				},
			},
		},
		stepAction: &v1beta1.StepAction{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stepAction",
				Namespace: "default",
			},
			Spec: v1beta1.StepActionSpec{
				Image:  "myimage",
				Args:   []string{"$(params.commands)"},
				Script: "echo $@",
				Params: v1.ParamSpecs{{
					Name: "commands",
					Type: v1.ParamTypeArray,
					Default: &v1.ParamValue{
						Type:     v1.ParamTypeArray,
						ArrayVal: []string{"Hello, I am the default value"},
					},
				}},
			},
		},
		expectedError: errors.New(`failed to resolve step ref for step "test" (index 0): invalid parameter substitution: commands. Please check the types of the default value and the passed value`),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			tektonclient := fake.NewSimpleClientset(tt.stepAction)

			_, err := GetStepActionsData(ctx, *tt.tr.Spec.TaskSpec, tt.tr, tektonclient, nil, nil)
			if err == nil {
				t.Fatalf("Expected to get an error but did not find any.")
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error()); d != "" {
				t.Errorf("the expected error did not match what was received: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetStepActionsData_InvalidStepResultReference(t *testing.T) {
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mytaskrun",
			Namespace: "default",
		},
		Spec: v1.TaskRunSpec{
			TaskSpec: &v1.TaskSpec{
				Steps: []v1.Step{{
					Name: "step1",
					Ref: &v1.Ref{
						Name: "stepAction",
					},
					Params: v1.Params{{
						Name: "param1",
						Value: v1.ParamValue{
							Type:      v1.ParamTypeString,
							StringVal: "$(steps.invalid.step)",
						},
					}},
				}},
			},
		},
	}

	stepAction := &v1beta1.StepAction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stepAction",
			Namespace: "default",
		},
		Spec: v1beta1.StepActionSpec{
			Image: "myimage",
			Params: v1.ParamSpecs{{
				Name: "param1",
				Type: v1.ParamTypeString,
			}},
		},
	}

	expectedError := `failed to resolve step ref for step "step1" (index 0): must be one of the form 1). "steps.<stepName>.results.<resultName>"; 2). "steps.<stepName>.results.<objectResultName>.<individualAttribute>"`
	ctx := t.Context()
	tektonclient := fake.NewSimpleClientset(stepAction)
	if _, err := GetStepActionsData(ctx, *tr.Spec.TaskSpec, tr, tektonclient, nil, nil); err.Error() != expectedError {
		t.Errorf("Expected error message %s but got %s", expectedError, err.Error())
	}
}
