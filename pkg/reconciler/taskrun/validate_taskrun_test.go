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

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/internalversion"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestValidateResolvedTask_ValidParams(t *testing.T) {
	ctx := context.Background()
	task := &internalversion.Task{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: internalversion.TaskSpec{
			Steps: []internalversion.Step{{
				Image:   "myimage",
				Command: []string{"mycmd"},
			}},
			Params: []internalversion.ParamSpec{
				{
					Name: "foo",
					Type: internalversion.ParamTypeString,
				}, {
					Name: "bar",
					Type: internalversion.ParamTypeString,
				}, {
					Name: "zoo",
					Type: internalversion.ParamTypeString,
				}, {
					Name: "matrixParam",
					Type: internalversion.ParamTypeString,
				}, {
					Name: "include",
					Type: internalversion.ParamTypeString,
				}, {
					Name: "arrayResultRef",
					Type: internalversion.ParamTypeArray,
				}, {
					Name: "myObjWithoutDefault",
					Type: internalversion.ParamTypeObject,
					Properties: map[string]internalversion.PropertySpec{
						"key1": {},
						"key2": {},
					},
				}, {
					Name: "myObjWithDefault",
					Type: internalversion.ParamTypeObject,
					Properties: map[string]internalversion.PropertySpec{
						"key1": {},
						"key2": {},
						"key3": {},
					},
					Default: &internalversion.ParamValue{
						Type: internalversion.ParamTypeObject,
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
	rtr := &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	}
	p := internalversion.Params{{
		Name:  "foo",
		Value: *internalversion.NewStructuredValues("somethinggood"),
	}, {
		Name:  "bar",
		Value: *internalversion.NewStructuredValues("somethinggood"),
	}, {
		Name:  "arrayResultRef",
		Value: *internalversion.NewStructuredValues("$(results.resultname[*])"),
	}, {
		Name: "myObjWithoutDefault",
		Value: *internalversion.NewObject(map[string]string{
			"key1":      "val1",
			"key2":      "val2",
			"extra_key": "val3",
		}),
	}, {
		Name: "myObjWithDefault",
		Value: *internalversion.NewObject(map[string]string{
			"key2": "val2",
			"key3": "val3",
		}),
	}}
	m := &v1.Matrix{
		Params: v1.Params{{
			Name:  "zoo",
			Value: *v1.NewStructuredValues("a", "b", "c"),
		}, {
			Name: "matrixParam", Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{}},
		}},
		Include: []v1.IncludeParams{{
			Name: "build-1",
			Params: v1.Params{{
				Name: "include", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "string-1"},
			}},
		}},
	}
	if err := ValidateResolvedTask(ctx, p, m, rtr); err != nil {
		t.Errorf("Did not expect to see error when validating TaskRun with correct params but saw %v", err)
	}
}
func TestValidateResolvedTask_ExtraValidParams(t *testing.T) {
	ctx := context.Background()
	tcs := []struct {
		name   string
		task   internalversion.Task
		params internalversion.Params
		matrix *v1.Matrix
	}{{
		name: "extra-str-param",
		task: internalversion.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: internalversion.TaskSpec{
				Params: []internalversion.ParamSpec{{
					Name: "foo",
					Type: internalversion.ParamTypeString,
				}},
			},
		},
		params: internalversion.Params{{
			Name:  "foo",
			Value: *internalversion.NewStructuredValues("string"),
		}, {
			Name:  "extrastr",
			Value: *internalversion.NewStructuredValues("extra"),
		}},
	}, {
		name: "extra-arr-param",
		task: internalversion.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: internalversion.TaskSpec{
				Params: []internalversion.ParamSpec{{
					Name: "foo",
					Type: internalversion.ParamTypeString,
				}},
			},
		},
		params: internalversion.Params{{
			Name:  "foo",
			Value: *internalversion.NewStructuredValues("string"),
		}, {
			Name:  "extraArr",
			Value: *internalversion.NewStructuredValues("extra", "arr"),
		}},
	}, {
		name: "extra-obj-param",
		task: internalversion.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: internalversion.TaskSpec{
				Params: []internalversion.ParamSpec{{
					Name: "foo",
					Type: internalversion.ParamTypeString,
				}},
			},
		},
		params: internalversion.Params{{
			Name:  "foo",
			Value: *internalversion.NewStructuredValues("string"),
		}, {
			Name: "myObjWithDefault",
			Value: *internalversion.NewObject(map[string]string{
				"key2": "val2",
				"key3": "val3",
			}),
		}},
	}, {
		name: "extra-param-matrix",
		task: internalversion.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: internalversion.TaskSpec{
				Params: []internalversion.ParamSpec{{
					Name: "include",
					Type: internalversion.ParamTypeString,
				}},
			},
		},
		params: internalversion.Params{{}},
		matrix: &v1.Matrix{
			Params: v1.Params{{
				Name:  "extraArr",
				Value: *v1.NewStructuredValues("extra", "arr"),
			}},
			Include: []v1.IncludeParams{{
				Name: "build-1",
				Params: v1.Params{{
					Name: "include", Value: *v1.NewStructuredValues("string"),
				}},
			}},
		},
	}}
	for _, tc := range tcs {
		rtr := &resources.ResolvedTask{
			TaskSpec: &tc.task.Spec,
		}
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateResolvedTask(ctx, tc.params, tc.matrix, rtr); err != nil {
				t.Errorf("Did not expect to see error when validating TaskRun with correct params but saw %v", err)
			}
		})
	}
}

func TestValidateResolvedTask_InvalidParams(t *testing.T) {
	ctx := context.Background()
	tcs := []struct {
		name    string
		task    internalversion.Task
		params  internalversion.Params
		matrix  *v1.Matrix
		wantErr string
	}{{
		name: "missing-params-string",
		task: internalversion.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: internalversion.TaskSpec{
				Params: []internalversion.ParamSpec{{
					Name: "foo",
					Type: internalversion.ParamTypeString,
				}},
			},
		},
		params: internalversion.Params{{
			Name:  "missing",
			Value: *internalversion.NewStructuredValues("somethingfun"),
		}},
		matrix:  &v1.Matrix{},
		wantErr: "invalid input params for task : missing values for these params which have no default values: [foo]",
	}, {
		name: "missing-params-arr",
		task: internalversion.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: internalversion.TaskSpec{
				Params: []internalversion.ParamSpec{{
					Name: "foo",
					Type: internalversion.ParamTypeArray,
				}},
			},
		},
		params: internalversion.Params{{
			Name:  "missing",
			Value: *internalversion.NewStructuredValues("array", "param"),
		}},
		matrix:  &v1.Matrix{},
		wantErr: "invalid input params for task : missing values for these params which have no default values: [foo]",
	}, {
		name: "invalid-string-param",
		task: internalversion.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: internalversion.TaskSpec{
				Params: []internalversion.ParamSpec{{
					Name: "foo",
					Type: internalversion.ParamTypeString,
				}},
			},
		},
		params: internalversion.Params{{
			Name:  "foo",
			Value: *internalversion.NewStructuredValues("array", "param"),
		}},
		matrix:  &v1.Matrix{},
		wantErr: "invalid input params for task : param types don't match the user-specified type: [foo]",
	}, {
		name: "invalid-arr-param",
		task: internalversion.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: internalversion.TaskSpec{
				Params: []internalversion.ParamSpec{{
					Name: "foo",
					Type: internalversion.ParamTypeArray,
				}},
			},
		},
		params: internalversion.Params{{
			Name:  "foo",
			Value: *internalversion.NewStructuredValues("string"),
		}},
		matrix:  &v1.Matrix{},
		wantErr: "invalid input params for task : param types don't match the user-specified type: [foo]",
	}, {name: "missing-param-in-matrix",
		task: internalversion.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: internalversion.TaskSpec{
				Params: []internalversion.ParamSpec{{
					Name: "bar",
					Type: internalversion.ParamTypeArray,
				}},
			},
		},
		params: internalversion.Params{{}},
		matrix: &v1.Matrix{
			Params: v1.Params{{
				Name:  "missing",
				Value: *v1.NewStructuredValues("foo"),
			}}},
		wantErr: "invalid input params for task : missing values for these params which have no default values: [bar]",
	}, {
		name: "missing-param-in-matrix-include",
		task: internalversion.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: internalversion.TaskSpec{
				Params: []internalversion.ParamSpec{{
					Name: "bar",
					Type: internalversion.ParamTypeArray,
				}},
			},
		},
		params: internalversion.Params{{}},
		matrix: &v1.Matrix{
			Include: []v1.IncludeParams{{
				Name: "build-1",
				Params: v1.Params{{
					Name: "missing", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "string"},
				}},
			}},
		},
		wantErr: "invalid input params for task : missing values for these params which have no default values: [bar]",
	}, {
		name: "invalid-arr-in-matrix-param",
		task: internalversion.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: internalversion.TaskSpec{
				Params: []internalversion.ParamSpec{{
					Name: "bar",
					Type: internalversion.ParamTypeArray,
				}},
			},
		},
		params: internalversion.Params{{}},
		matrix: &v1.Matrix{
			Params: v1.Params{{
				Name:  "bar",
				Value: *v1.NewStructuredValues("foo"),
			}}},
		wantErr: "invalid input params for task : param types don't match the user-specified type: [bar]",
	}, {
		name: "invalid-str-in-matrix-include-param",
		task: internalversion.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: internalversion.TaskSpec{
				Params: []internalversion.ParamSpec{{
					Name: "bar",
					Type: internalversion.ParamTypeArray,
				}},
			},
		},
		params: internalversion.Params{{}},
		matrix: &v1.Matrix{
			Include: []v1.IncludeParams{{
				Name: "build-1",
				Params: v1.Params{{
					Name: "bar", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "string"},
				}},
			}},
		},
		wantErr: "invalid input params for task : param types don't match the user-specified type: [bar]",
	}, {
		name: "missing-params-obj",
		task: internalversion.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: internalversion.TaskSpec{
				Params: []internalversion.ParamSpec{{
					Name: "foo",
					Type: internalversion.ParamTypeArray,
				}},
			},
		},
		params: internalversion.Params{{
			Name: "missing-obj",
			Value: *internalversion.NewObject(map[string]string{
				"key1":    "val1",
				"misskey": "val2",
			}),
		}},
		matrix:  &v1.Matrix{},
		wantErr: "invalid input params for task : missing values for these params which have no default values: [foo]",
	}, {
		name: "missing object param keys",
		task: internalversion.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: internalversion.TaskSpec{
				Params: []internalversion.ParamSpec{{
					Name: "myObjWithoutDefault",
					Type: internalversion.ParamTypeObject,
					Properties: map[string]internalversion.PropertySpec{
						"key1": {},
						"key2": {},
					},
				}},
			},
		},
		params: internalversion.Params{{
			Name: "myObjWithoutDefault",
			Value: *internalversion.NewObject(map[string]string{
				"key1":    "val1",
				"misskey": "val2",
			}),
		}},
		matrix:  &v1.Matrix{},
		wantErr: "invalid input params for task : missing keys for these params which are required in ParamSpec's properties map[myObjWithoutDefault:[key2]]",
	}}
	for _, tc := range tcs {
		rtr := &resources.ResolvedTask{
			TaskSpec: &tc.task.Spec,
		}
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateResolvedTask(ctx, tc.params, tc.matrix, rtr)
			if d := cmp.Diff(tc.wantErr, err.Error()); d != "" {
				t.Errorf("Did not get the expected Error  %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestValidateOverrides(t *testing.T) {
	tcs := []struct {
		name    string
		ts      *internalversion.TaskSpec
		trs     *v1.TaskRunSpec
		wantErr bool
	}{{
		name: "valid stepOverrides",
		ts: &internalversion.TaskSpec{
			Steps: []internalversion.Step{{
				Name: "step1",
			}, {
				Name: "step2",
			}},
		},
		trs: &v1.TaskRunSpec{
			StepSpecs: []v1.TaskRunStepSpec{{
				Name: "step1",
			}},
		},
	}, {
		name: "valid sidecarOverrides",
		ts: &internalversion.TaskSpec{
			Sidecars: []internalversion.Sidecar{{
				Name: "step1",
			}, {
				Name: "step2",
			}},
		},
		trs: &v1.TaskRunSpec{
			SidecarSpecs: []v1.TaskRunSidecarSpec{{
				Name: "step1",
			}},
		},
	}, {
		name: "invalid stepOverrides",
		ts:   &internalversion.TaskSpec{},
		trs: &v1.TaskRunSpec{
			StepSpecs: []v1.TaskRunStepSpec{{
				Name: "step1",
			}},
		},
		wantErr: true,
	}, {
		name: "invalid sidecarOverrides",
		ts:   &internalversion.TaskSpec{},
		trs: &v1.TaskRunSpec{
			SidecarSpecs: []v1.TaskRunSidecarSpec{{
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
		tr      *v1.TaskRun
		rtr     *internalversion.TaskSpec
		wantErr bool
	}{{
		name: "valid taskrun spec results",
		tr: &v1.TaskRun{
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Results: []v1.TaskResult{
						{
							Name: "string-result",
							Type: v1.ResultsTypeString,
						},
						{
							Name: "array-result",
							Type: v1.ResultsTypeArray,
						},
						{
							Name:       "object-result",
							Type:       v1.ResultsTypeObject,
							Properties: map[string]v1.PropertySpec{"hello": {Type: "string"}},
						},
					},
				},
			},
			Status: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Results: []v1.TaskRunResult{
						{
							Name:  "string-result",
							Type:  v1.ResultsTypeString,
							Value: *v1.NewStructuredValues("hello"),
						},
						{
							Name:  "array-result",
							Type:  v1.ResultsTypeArray,
							Value: *v1.NewStructuredValues("hello", "world"),
						},
						{
							Name:  "object-result",
							Type:  v1.ResultsTypeObject,
							Value: *v1.NewObject(map[string]string{"hello": "world"}),
						},
					},
				},
			},
		},
		rtr: &internalversion.TaskSpec{
			Results: []internalversion.TaskResult{},
		},
		wantErr: false,
	}, {
		name: "valid taskspec results",
		tr: &v1.TaskRun{
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Results: []v1.TaskResult{
						{
							Name: "string-result",
							Type: v1.ResultsTypeString,
						},
						{
							Name: "array-result",
							Type: v1.ResultsTypeArray,
						},
						{
							Name: "object-result",
							Type: v1.ResultsTypeObject,
						},
					},
				},
			},
			Status: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Results: []v1.TaskRunResult{
						{
							Name:  "string-result",
							Type:  v1.ResultsTypeString,
							Value: *v1.NewStructuredValues("hello"),
						},
						{
							Name:  "array-result",
							Type:  v1.ResultsTypeArray,
							Value: *v1.NewStructuredValues("hello", "world"),
						},
						{
							Name:  "object-result",
							Type:  v1.ResultsTypeObject,
							Value: *v1.NewObject(map[string]string{"hello": "world"}),
						},
					},
				},
			},
		},
		rtr: &internalversion.TaskSpec{
			Results: []internalversion.TaskResult{},
		},
		wantErr: false,
	}, {
		name: "invalid taskrun spec results types",
		tr: &v1.TaskRun{
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Results: []v1.TaskResult{
						{
							Name: "string-result",
							Type: v1.ResultsTypeString,
						},
						{
							Name: "array-result",
							Type: v1.ResultsTypeArray,
						},
						{
							Name:       "object-result",
							Type:       v1.ResultsTypeObject,
							Properties: map[string]v1.PropertySpec{"hello": {Type: "string"}},
						},
					},
				},
			},
			Status: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Results: []v1.TaskRunResult{
						{
							Name:  "string-result",
							Type:  v1.ResultsTypeArray,
							Value: *v1.NewStructuredValues("hello", "world"),
						},
						{
							Name:  "array-result",
							Type:  v1.ResultsTypeObject,
							Value: *v1.NewObject(map[string]string{"hello": "world"}),
						},
						{
							Name:  "object-result",
							Type:  v1.ResultsTypeString,
							Value: *v1.NewStructuredValues("hello"),
						},
					},
				},
			},
		},
		rtr: &internalversion.TaskSpec{
			Results: []internalversion.TaskResult{},
		},
		wantErr: true,
	}, {
		name: "invalid taskspec results types",
		tr: &v1.TaskRun{
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Results: []v1.TaskResult{},
				},
			},
			Status: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Results: []v1.TaskRunResult{
						{
							Name:  "string-result",
							Type:  v1.ResultsTypeArray,
							Value: *v1.NewStructuredValues("hello", "world"),
						},
						{
							Name:  "array-result",
							Type:  v1.ResultsTypeObject,
							Value: *v1.NewObject(map[string]string{"hello": "world"}),
						},
						{
							Name:  "object-result",
							Type:  v1.ResultsTypeString,
							Value: *v1.NewStructuredValues("hello"),
						},
					},
				},
			},
		},
		rtr: &internalversion.TaskSpec{
			Results: []internalversion.TaskResult{
				{
					Name: "string-result",
					Type: internalversion.ResultsTypeString,
				},
				{
					Name: "array-result",
					Type: internalversion.ResultsTypeArray,
				},
				{
					Name:       "object-result",
					Type:       internalversion.ResultsTypeObject,
					Properties: map[string]internalversion.PropertySpec{"hello": {Type: "string"}},
				},
			},
		},
		wantErr: true,
	}, {
		name: "invalid taskrun spec results object properties",
		tr: &v1.TaskRun{
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Results: []v1.TaskResult{
						{
							Name:       "object-result",
							Type:       v1.ResultsTypeObject,
							Properties: map[string]v1.PropertySpec{"world": {Type: "string"}},
						},
					},
				},
			},
			Status: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Results: []v1.TaskRunResult{
						{
							Name:  "object-result",
							Type:  v1.ResultsTypeObject,
							Value: *v1.NewObject(map[string]string{"hello": "world"}),
						},
					},
				},
			},
		},
		rtr: &internalversion.TaskSpec{
			Results: []internalversion.TaskResult{},
		},
		wantErr: true,
	}, {
		name: "invalid taskspec results object properties",
		tr: &v1.TaskRun{
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Results: []v1.TaskResult{},
				},
			},
			Status: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Results: []v1.TaskRunResult{
						{
							Name:  "object-result",
							Type:  v1.ResultsTypeObject,
							Value: *v1.NewObject(map[string]string{"hello": "world"}),
						},
					},
				},
			},
		},
		rtr: &internalversion.TaskSpec{
			Results: []internalversion.TaskResult{
				{
					Name:       "object-result",
					Type:       internalversion.ResultsTypeObject,
					Properties: map[string]internalversion.PropertySpec{"world": {Type: "string"}},
				},
			},
		},
		wantErr: true,
	}, {
		name: "invalid taskrun spec results types with other valid types",
		tr: &v1.TaskRun{
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Results: []v1.TaskResult{
						{
							Name: "string-result",
							Type: v1.ResultsTypeString,
						},
						{
							Name: "array-result-1",
							Type: v1.ResultsTypeArray,
						}, {
							Name: "array-result-2",
							Type: v1.ResultsTypeArray,
						},
						{
							Name:       "object-result",
							Type:       v1.ResultsTypeObject,
							Properties: map[string]v1.PropertySpec{"hello": {Type: "string"}},
						},
					},
				},
			},
			Status: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Results: []v1.TaskRunResult{
						{
							Name:  "string-result",
							Type:  v1.ResultsTypeString,
							Value: *v1.NewStructuredValues("hello"),
						},
						{
							Name:  "array-result-1",
							Type:  v1.ResultsTypeObject,
							Value: *v1.NewObject(map[string]string{"hello": "world"}),
						}, {
							Name:  "array-result-2",
							Type:  v1.ResultsTypeString,
							Value: *v1.NewStructuredValues(""),
						},
						{
							Name:  "object-result",
							Type:  v1.ResultsTypeObject,
							Value: *v1.NewObject(map[string]string{"hello": "world"}),
						},
					},
				},
			},
		},
		rtr: &internalversion.TaskSpec{
			Results: []internalversion.TaskResult{},
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
