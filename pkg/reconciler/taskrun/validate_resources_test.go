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

package taskrun

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
)

func TestValidateResolvedTaskResources_ValidParams(t *testing.T) {
	ctx := context.Background()
	task := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Image:   "myimage",
				Command: []string{"mycmd"},
			}},
			Params: []v1beta1.ParamSpec{
				{
					Name: "foo",
					Type: v1beta1.ParamTypeString,
				},
				{
					Name: "bar",
					Type: v1beta1.ParamTypeString,
				},
				{
					Name: "zoo",
					Type: v1beta1.ParamTypeString,
				}, {
					Name: "arrayResultRef",
					Type: v1beta1.ParamTypeArray,
				}, {
					Name: "myObjWithoutDefault",
					Type: v1beta1.ParamTypeObject,
					Properties: map[string]v1beta1.PropertySpec{
						"key1": {},
						"key2": {},
					},
				}, {
					Name: "myObjWithDefault",
					Type: v1beta1.ParamTypeObject,
					Properties: map[string]v1beta1.PropertySpec{
						"key1": {},
						"key2": {},
						"key3": {},
					},
					Default: &v1beta1.ParamValue{
						Type: v1beta1.ParamTypeObject,
						ObjectVal: map[string]string{
							"key1": "val1-default",
							"key2": "val2-default", // key2 is also provided and will be overridden by taskrun
							// key3 will be provided by taskrun
						},
					},
				},
			},
		},
	}
	rtr := &resources.ResolvedTaskResources{
		TaskSpec: &task.Spec,
	}
	p := []v1beta1.Param{{
		Name:  "foo",
		Value: *v1beta1.NewStructuredValues("somethinggood"),
	}, {
		Name:  "bar",
		Value: *v1beta1.NewStructuredValues("somethinggood"),
	}, {
		Name:  "arrayResultRef",
		Value: *v1beta1.NewStructuredValues("$(results.resultname[*])"),
	}, {
		Name: "myObjWithoutDefault",
		Value: *v1beta1.NewObject(map[string]string{
			"key1":      "val1",
			"key2":      "val2",
			"extra_key": "val3",
		}),
	}, {
		Name: "myObjWithDefault",
		Value: *v1beta1.NewObject(map[string]string{
			"key2": "val2",
			"key3": "val3",
		}),
	}}
	m := []v1beta1.Param{{
		Name:  "zoo",
		Value: *v1beta1.NewStructuredValues("a", "b", "c"),
	}}
	if err := ValidateResolvedTaskResources(ctx, p, &v1beta1.Matrix{Params: m}, rtr); err != nil {
		t.Fatalf("Did not expect to see error when validating TaskRun with correct params but saw %v", err)
	}

	t.Run("alpha-extra-params", func(t *testing.T) {
		ctx := config.ToContext(ctx, &config.Config{FeatureFlags: &config.FeatureFlags{EnableAPIFields: "alpha"}})
		extra := v1beta1.Param{
			Name:  "extra",
			Value: *v1beta1.NewStructuredValues("i am an extra param"),
		}
		extraarray := v1beta1.Param{
			Name:  "extraarray",
			Value: *v1beta1.NewStructuredValues("i", "am", "an", "extra", "array", "param"),
		}
		if err := ValidateResolvedTaskResources(ctx, append(p, extra), &v1beta1.Matrix{Params: append(m, extraarray)}, rtr); err != nil {
			t.Fatalf("Did not expect to see error when validating TaskRun with correct params but saw %v", err)
		}
	})
}

func TestValidateResolvedTaskResources_InvalidParams(t *testing.T) {
	ctx := context.Background()
	task := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Image:   "myimage",
				Command: []string{"mycmd"},
			}},
			Params: []v1beta1.ParamSpec{
				{
					Name: "foo",
					Type: v1beta1.ParamTypeString,
				}, {
					Name: "bar",
					Type: v1beta1.ParamTypeArray,
				},
				{
					Name: "myObjWithoutDefault",
					Type: v1beta1.ParamTypeObject,
					Properties: map[string]v1beta1.PropertySpec{
						"key1": {},
						"key2": {},
					},
				}, {
					Name: "myObjWithDefault",
					Type: v1beta1.ParamTypeObject,
					Properties: map[string]v1beta1.PropertySpec{
						"key1": {},
						"key2": {},
						"key3": {},
					},
					Default: &v1beta1.ParamValue{
						Type: v1beta1.ParamTypeObject,
						ObjectVal: map[string]string{
							"key1": "default",
							// key2 is not provided by default nor taskrun, which is why error is epected.
							// key3 is provided by taskrun
						},
					},
				},
			},
		},
	}
	tcs := []struct {
		name   string
		rtr    *resources.ResolvedTaskResources
		params []v1beta1.Param
		matrix *v1beta1.Matrix
	}{{
		name: "missing-params",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
		params: []v1beta1.Param{{
			Name:  "foobar",
			Value: *v1beta1.NewStructuredValues("somethingfun"),
		}},
		matrix: &v1beta1.Matrix{
			Params: []v1beta1.Param{{
				Name:  "barfoo",
				Value: *v1beta1.NewStructuredValues("bar", "foo"),
			}},
		},
	}, {
		name: "invalid-type-in-params",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
		params: []v1beta1.Param{{
			Name:  "foo",
			Value: *v1beta1.NewStructuredValues("bar", "foo"),
		}},
	}, {
		name: "invalid-type-in-matrix",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
		matrix: &v1beta1.Matrix{
			Params: []v1beta1.Param{{
				Name:  "bar",
				Value: *v1beta1.NewStructuredValues("bar", "foo"),
			}}},
	}, {
		name: "missing object param keys",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
		params: []v1beta1.Param{{
			Name:  "foo",
			Value: *v1beta1.NewStructuredValues("test"),
		}, {
			Name:  "bar",
			Value: *v1beta1.NewStructuredValues("a", "b"),
		}, {
			Name: "myObjWithoutDefault",
			Value: *v1beta1.NewObject(map[string]string{
				"key1":    "val1",
				"misskey": "val2",
			}),
		}, {
			Name: "myObjWithDefault",
			Value: *v1beta1.NewObject(map[string]string{
				"key3": "val3",
			}),
		}},
	},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateResolvedTaskResources(ctx, tc.params, tc.matrix, tc.rtr); err == nil {
				t.Errorf("Expected to see error when validating invalid resolved TaskRun with wrong params but saw none")
			}
		})
	}
}

func TestValidateOverrides(t *testing.T) {
	tcs := []struct {
		name    string
		ts      *v1beta1.TaskSpec
		trs     *v1beta1.TaskRunSpec
		wantErr bool
	}{{
		name: "valid stepOverrides",
		ts: &v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Name: "step1",
			}, {
				Name: "step2",
			}},
		},
		trs: &v1beta1.TaskRunSpec{
			StepOverrides: []v1beta1.TaskRunStepOverride{{
				Name: "step1",
			}},
		},
	}, {
		name: "valid sidecarOverrides",
		ts: &v1beta1.TaskSpec{
			Sidecars: []v1beta1.Sidecar{{
				Name: "step1",
			}, {
				Name: "step2",
			}},
		},
		trs: &v1beta1.TaskRunSpec{
			SidecarOverrides: []v1beta1.TaskRunSidecarOverride{{
				Name: "step1",
			}},
		},
	}, {
		name: "invalid stepOverrides",
		ts:   &v1beta1.TaskSpec{},
		trs: &v1beta1.TaskRunSpec{
			StepOverrides: []v1beta1.TaskRunStepOverride{{
				Name: "step1",
			}},
		},
		wantErr: true,
	}, {
		name: "invalid sidecarOverrides",
		ts:   &v1beta1.TaskSpec{},
		trs: &v1beta1.TaskRunSpec{
			SidecarOverrides: []v1beta1.TaskRunSidecarOverride{{
				Name: "step1",
			}},
		},
		wantErr: true,
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := validateOverrides(tc.ts, tc.trs)
			if (err != nil) != tc.wantErr {
				t.Errorf("expected err: %t, but got err %s", tc.wantErr, err)
			}
		})
	}
}

func TestValidateResult(t *testing.T) {
	tcs := []struct {
		name    string
		tr      *v1beta1.TaskRun
		rtr     *v1beta1.TaskSpec
		wantErr bool
	}{{
		name: "valid taskrun spec results",
		tr: &v1beta1.TaskRun{
			Spec: v1beta1.TaskRunSpec{
				TaskSpec: &v1beta1.TaskSpec{
					Results: []v1beta1.TaskResult{
						{
							Name: "string-result",
							Type: v1beta1.ResultsTypeString,
						},
						{
							Name: "array-result",
							Type: v1beta1.ResultsTypeArray,
						},
						{
							Name:       "object-result",
							Type:       v1beta1.ResultsTypeObject,
							Properties: map[string]v1beta1.PropertySpec{"hello": {Type: "string"}},
						},
					},
				},
			},
			Status: v1beta1.TaskRunStatus{
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					TaskRunResults: []v1beta1.TaskRunResult{
						{
							Name:  "string-result",
							Type:  v1beta1.ResultsTypeString,
							Value: *v1beta1.NewStructuredValues("hello"),
						},
						{
							Name:  "array-result",
							Type:  v1beta1.ResultsTypeArray,
							Value: *v1beta1.NewStructuredValues("hello", "world"),
						},
						{
							Name:  "object-result",
							Type:  v1beta1.ResultsTypeObject,
							Value: *v1beta1.NewObject(map[string]string{"hello": "world"}),
						},
					},
				},
			},
		},
		rtr: &v1beta1.TaskSpec{
			Results: []v1beta1.TaskResult{},
		},
		wantErr: false,
	}, {
		name: "valid taskspec results",
		tr: &v1beta1.TaskRun{
			Spec: v1beta1.TaskRunSpec{
				TaskSpec: &v1beta1.TaskSpec{
					Results: []v1beta1.TaskResult{
						{
							Name: "string-result",
							Type: v1beta1.ResultsTypeString,
						},
						{
							Name: "array-result",
							Type: v1beta1.ResultsTypeArray,
						},
						{
							Name: "object-result",
							Type: v1beta1.ResultsTypeObject,
						},
					},
				},
			},
			Status: v1beta1.TaskRunStatus{
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					TaskRunResults: []v1beta1.TaskRunResult{
						{
							Name:  "string-result",
							Type:  v1beta1.ResultsTypeString,
							Value: *v1beta1.NewStructuredValues("hello"),
						},
						{
							Name:  "array-result",
							Type:  v1beta1.ResultsTypeArray,
							Value: *v1beta1.NewStructuredValues("hello", "world"),
						},
						{
							Name:  "object-result",
							Type:  v1beta1.ResultsTypeObject,
							Value: *v1beta1.NewObject(map[string]string{"hello": "world"}),
						},
					},
				},
			},
		},
		rtr: &v1beta1.TaskSpec{
			Results: []v1beta1.TaskResult{},
		},
		wantErr: false,
	}, {
		name: "invalid taskrun spec results types",
		tr: &v1beta1.TaskRun{
			Spec: v1beta1.TaskRunSpec{
				TaskSpec: &v1beta1.TaskSpec{
					Results: []v1beta1.TaskResult{
						{
							Name: "string-result",
							Type: v1beta1.ResultsTypeString,
						},
						{
							Name: "array-result",
							Type: v1beta1.ResultsTypeArray,
						},
						{
							Name:       "object-result",
							Type:       v1beta1.ResultsTypeObject,
							Properties: map[string]v1beta1.PropertySpec{"hello": {Type: "string"}},
						},
					},
				},
			},
			Status: v1beta1.TaskRunStatus{
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					TaskRunResults: []v1beta1.TaskRunResult{
						{
							Name:  "string-result",
							Type:  v1beta1.ResultsTypeArray,
							Value: *v1beta1.NewStructuredValues("hello", "world"),
						},
						{
							Name:  "array-result",
							Type:  v1beta1.ResultsTypeObject,
							Value: *v1beta1.NewObject(map[string]string{"hello": "world"}),
						},
						{
							Name:  "object-result",
							Type:  v1beta1.ResultsTypeString,
							Value: *v1beta1.NewStructuredValues("hello"),
						},
					},
				},
			},
		},
		rtr: &v1beta1.TaskSpec{
			Results: []v1beta1.TaskResult{},
		},
		wantErr: true,
	}, {
		name: "invalid taskspec results types",
		tr: &v1beta1.TaskRun{
			Spec: v1beta1.TaskRunSpec{
				TaskSpec: &v1beta1.TaskSpec{
					Results: []v1beta1.TaskResult{},
				},
			},
			Status: v1beta1.TaskRunStatus{
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					TaskRunResults: []v1beta1.TaskRunResult{
						{
							Name:  "string-result",
							Type:  v1beta1.ResultsTypeArray,
							Value: *v1beta1.NewStructuredValues("hello", "world"),
						},
						{
							Name:  "array-result",
							Type:  v1beta1.ResultsTypeObject,
							Value: *v1beta1.NewObject(map[string]string{"hello": "world"}),
						},
						{
							Name:  "object-result",
							Type:  v1beta1.ResultsTypeString,
							Value: *v1beta1.NewStructuredValues("hello"),
						},
					},
				},
			},
		},
		rtr: &v1beta1.TaskSpec{
			Results: []v1beta1.TaskResult{
				{
					Name: "string-result",
					Type: v1beta1.ResultsTypeString,
				},
				{
					Name: "array-result",
					Type: v1beta1.ResultsTypeArray,
				},
				{
					Name:       "object-result",
					Type:       v1beta1.ResultsTypeObject,
					Properties: map[string]v1beta1.PropertySpec{"hello": {Type: "string"}},
				},
			},
		},
		wantErr: true,
	}, {
		name: "invalid taskrun spec results object properties",
		tr: &v1beta1.TaskRun{
			Spec: v1beta1.TaskRunSpec{
				TaskSpec: &v1beta1.TaskSpec{
					Results: []v1beta1.TaskResult{
						{
							Name:       "object-result",
							Type:       v1beta1.ResultsTypeObject,
							Properties: map[string]v1beta1.PropertySpec{"world": {Type: "string"}},
						},
					},
				},
			},
			Status: v1beta1.TaskRunStatus{
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					TaskRunResults: []v1beta1.TaskRunResult{
						{
							Name:  "object-result",
							Type:  v1beta1.ResultsTypeObject,
							Value: *v1beta1.NewObject(map[string]string{"hello": "world"}),
						},
					},
				},
			},
		},
		rtr: &v1beta1.TaskSpec{
			Results: []v1beta1.TaskResult{},
		},
		wantErr: true,
	}, {
		name: "invalid taskspec results object properties",
		tr: &v1beta1.TaskRun{
			Spec: v1beta1.TaskRunSpec{
				TaskSpec: &v1beta1.TaskSpec{
					Results: []v1beta1.TaskResult{},
				},
			},
			Status: v1beta1.TaskRunStatus{
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					TaskRunResults: []v1beta1.TaskRunResult{
						{
							Name:  "object-result",
							Type:  v1beta1.ResultsTypeObject,
							Value: *v1beta1.NewObject(map[string]string{"hello": "world"}),
						},
					},
				},
			},
		},
		rtr: &v1beta1.TaskSpec{
			Results: []v1beta1.TaskResult{
				{
					Name:       "object-result",
					Type:       v1beta1.ResultsTypeObject,
					Properties: map[string]v1beta1.PropertySpec{"world": {Type: "string"}},
				},
			},
		},
		wantErr: true,
	}, {
		name: "invalid taskrun spec results types with other valid types",
		tr: &v1beta1.TaskRun{
			Spec: v1beta1.TaskRunSpec{
				TaskSpec: &v1beta1.TaskSpec{
					Results: []v1beta1.TaskResult{
						{
							Name: "string-result",
							Type: v1beta1.ResultsTypeString,
						},
						{
							Name: "array-result-1",
							Type: v1beta1.ResultsTypeArray,
						}, {
							Name: "array-result-2",
							Type: v1beta1.ResultsTypeArray,
						},
						{
							Name:       "object-result",
							Type:       v1beta1.ResultsTypeObject,
							Properties: map[string]v1beta1.PropertySpec{"hello": {Type: "string"}},
						},
					},
				},
			},
			Status: v1beta1.TaskRunStatus{
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					TaskRunResults: []v1beta1.TaskRunResult{
						{
							Name:  "string-result",
							Type:  v1beta1.ResultsTypeString,
							Value: *v1beta1.NewStructuredValues("hello"),
						},
						{
							Name:  "array-result-1",
							Type:  v1beta1.ResultsTypeObject,
							Value: *v1beta1.NewObject(map[string]string{"hello": "world"}),
						}, {
							Name:  "array-result-2",
							Type:  v1beta1.ResultsTypeString,
							Value: *v1beta1.NewStructuredValues(""),
						},
						{
							Name:  "object-result",
							Type:  v1beta1.ResultsTypeObject,
							Value: *v1beta1.NewObject(map[string]string{"hello": "world"}),
						},
					},
				},
			},
		},
		rtr: &v1beta1.TaskSpec{
			Results: []v1beta1.TaskResult{},
		},
		wantErr: true,
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTaskRunResults(tc.tr, tc.rtr)
			if err == nil && tc.wantErr {
				t.Errorf("expected err: %t, but got different err: %s", tc.wantErr, err)
			} else if err != nil && !tc.wantErr {
				t.Errorf("did not expect any err, but got err: %s", err)
			}
		})
	}
}
