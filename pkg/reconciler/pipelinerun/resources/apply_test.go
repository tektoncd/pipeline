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
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resources "github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/selection"
)

func TestApplyParameters(t *testing.T) {
	for _, tt := range []struct {
		name     string
		original v1.PipelineSpec
		params   []v1.Param
		expected v1.PipelineSpec
		wc       func(context.Context) context.Context
	}{{
		name: "single parameter",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
				{Name: "second-param", Type: v1.ParamTypeString},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues("$(params.first-param)")},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues("$(params.second-param)")},
					{Name: "first-task-third-param", Value: *v1.NewStructuredValues("static value")},
				},
			}},
		},
		params: v1.Params{{Name: "second-param", Value: *v1.NewStructuredValues("second-value")}},
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
				{Name: "second-param", Type: v1.ParamTypeString},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues("default-value")},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues("second-value")},
					{Name: "first-task-third-param", Value: *v1.NewStructuredValues("static value")},
				},
			}},
		},
	}, {
		name: "parameter propagation string no task or task default winner pipeline",
		original: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Steps: []v1.Step{{
							Name:   "step1",
							Image:  "ubuntu",
							Script: `#!/usr/bin/env bash\necho "$(params.HELLO)"`,
						}},
					},
				},
			}},
		},
		params: v1.Params{{Name: "HELLO", Value: *v1.NewStructuredValues("hello param!")}},
		expected: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Steps: []v1.Step{{
							Name:   "step1",
							Image:  "ubuntu",
							Script: `#!/usr/bin/env bash\necho "hello param!"`,
						}},
					},
				},
			}},
		},
	}, {
		name: "parameter propagation string into finally task",
		original: v1.PipelineSpec{
			Finally: []v1.PipelineTask{{
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Steps: []v1.Step{{
							Name:   "step1",
							Image:  "ubuntu",
							Script: `#!/usr/bin/env bash\necho "$(params.HELLO)"`,
						}},
					},
				},
			}},
		},
		params: v1.Params{{Name: "HELLO", Value: *v1.NewStructuredValues("hello param!")}},
		expected: v1.PipelineSpec{
			Finally: []v1.PipelineTask{{
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Steps: []v1.Step{{
							Name:   "step1",
							Image:  "ubuntu",
							Script: `#!/usr/bin/env bash\necho "hello param!"`,
						}},
					},
				},
			}},
		},
	}, {
		name: "parameter propagation array no task or task default winner pipeline",
		original: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Steps: []v1.Step{{
							Name:  "step1",
							Image: "ubuntu",
							Args:  []string{"#!/usr/bin/env bash\n", "echo", "$(params.HELLO[*])"},
						}},
					},
				},
			}},
		},
		params: v1.Params{{Name: "HELLO", Value: *v1.NewStructuredValues("hello", "param", "!!!")}},
		expected: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Steps: []v1.Step{{
							Name:  "step1",
							Image: "ubuntu",
							Args:  []string{"#!/usr/bin/env bash\n", "echo", "hello", "param", "!!!"},
						}},
					},
				},
			}},
		},
	}, {
		name: "parameter propagation array finally task",
		original: v1.PipelineSpec{
			Finally: []v1.PipelineTask{{
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Steps: []v1.Step{{
							Name:  "step1",
							Image: "ubuntu",
							Args:  []string{"#!/usr/bin/env bash\n", "echo", "$(params.HELLO[*])"},
						}},
					},
				},
			}},
		},
		params: v1.Params{{Name: "HELLO", Value: *v1.NewStructuredValues("hello", "param", "!!!")}},
		expected: v1.PipelineSpec{
			Finally: []v1.PipelineTask{{
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Steps: []v1.Step{{
							Name:  "step1",
							Image: "ubuntu",
							Args:  []string{"#!/usr/bin/env bash\n", "echo", "hello", "param", "!!!"},
						}},
					},
				},
			}},
		},
	}, {
		name: "parameter propagation object no task or task default winner pipeline",
		original: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Steps: []v1.Step{{
							Name:  "step1",
							Image: "ubuntu",
							Args:  []string{"#!/usr/bin/env bash\n", "echo", "$(params.myObject.key1) $(params.myObject.key2)"},
						}},
					},
				},
			}},
		},
		params: v1.Params{{Name: "myObject", Value: *v1.NewObject(map[string]string{"key1": "hello", "key2": "world!"})}},
		expected: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Steps: []v1.Step{{
							Name:  "step1",
							Image: "ubuntu",
							Args:  []string{"#!/usr/bin/env bash\n", "echo", "hello world!"},
						}},
					},
				},
			}},
		},
		wc: config.EnableAlphaAPIFields,
	}, {
		name: "parameter propagation object finally task",
		original: v1.PipelineSpec{
			Finally: []v1.PipelineTask{{
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Steps: []v1.Step{{
							Name:  "step1",
							Image: "ubuntu",
							Args:  []string{"#!/usr/bin/env bash\n", "echo", "$(params.myObject.key1) $(params.myObject.key2)"},
						}},
					},
				},
			}},
		},
		params: v1.Params{{Name: "myObject", Value: *v1.NewObject(map[string]string{"key1": "hello", "key2": "world!"})}},
		expected: v1.PipelineSpec{
			Finally: []v1.PipelineTask{{
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Steps: []v1.Step{{
							Name:  "step1",
							Image: "ubuntu",
							Args:  []string{"#!/usr/bin/env bash\n", "echo", "hello world!"},
						}},
					},
				},
			}},
		},
		wc: config.EnableAlphaAPIFields,
	}, {
		name: "parameter propagation with task default but no task winner pipeline",
		original: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Params: []v1.ParamSpec{{
							Name:    "HELLO",
							Default: v1.NewStructuredValues("default param!"),
						}},
						Steps: []v1.Step{{
							Name:   "step1",
							Image:  "ubuntu",
							Script: `#!/usr/bin/env bash\necho "$(params.HELLO)"`,
						}},
					},
				},
			}},
		},
		params: v1.Params{{Name: "HELLO", Value: *v1.NewStructuredValues("pipeline param!")}},
		expected: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Params: []v1.ParamSpec{{
							Name:    "HELLO",
							Default: v1.NewStructuredValues("default param!"),
						}},
						Steps: []v1.Step{{
							Name:   "step1",
							Image:  "ubuntu",
							Script: `#!/usr/bin/env bash\necho "pipeline param!"`,
						}},
					},
				},
			}},
		},
	}, {
		name: "parameter propagation with task scoping Finally task",
		original: v1.PipelineSpec{
			Finally: []v1.PipelineTask{{
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Params: []v1.ParamSpec{{
							Name:    "HELLO",
							Default: v1.NewStructuredValues("default param!"),
						}},
						Steps: []v1.Step{{
							Name:   "step1",
							Image:  "ubuntu",
							Script: `#!/usr/bin/env bash\necho "$(params.HELLO)"`,
						}},
					},
				},
			}},
		},
		params: v1.Params{{Name: "HELLO", Value: *v1.NewStructuredValues("pipeline param!")}},
		expected: v1.PipelineSpec{
			Finally: []v1.PipelineTask{{
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Params: []v1.ParamSpec{{
							Name:    "HELLO",
							Default: v1.NewStructuredValues("default param!"),
						}},
						Steps: []v1.Step{{
							Name:   "step1",
							Image:  "ubuntu",
							Script: `#!/usr/bin/env bash\necho "pipeline param!"`,
						}},
					},
				},
			}},
		},
	}, {
		name: "parameter propagation array with task default but no task winner pipeline",
		original: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Params: []v1.ParamSpec{{
							Name:    "HELLO",
							Default: v1.NewStructuredValues("default", "param!"),
						}},
						Steps: []v1.Step{{
							Name:  "step1",
							Image: "ubuntu",
							Args:  []string{"#!/usr/bin/env bash\n", "echo", "$(params.HELLO)"},
						}},
					},
				},
			}},
		},
		params: v1.Params{{Name: "HELLO", Value: *v1.NewStructuredValues("pipeline", "param!")}},
		expected: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Params: []v1.ParamSpec{{
							Name:    "HELLO",
							Default: v1.NewStructuredValues("default", "param!"),
						}},
						Steps: []v1.Step{{
							Name:  "step1",
							Image: "ubuntu",
							Args:  []string{"#!/usr/bin/env bash\n", "echo", "pipeline", "param!"},
						}},
					},
				},
			}},
		},
	}, {
		name: "parameter propagation array with task scoping Finally task",
		original: v1.PipelineSpec{
			Finally: []v1.PipelineTask{{
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Params: []v1.ParamSpec{{
							Name:    "HELLO",
							Default: v1.NewStructuredValues("default", "param!"),
						}},
						Steps: []v1.Step{{
							Name:  "step1",
							Image: "ubuntu",
							Args:  []string{"#!/usr/bin/env bash\n", "echo", "$(params.HELLO)"},
						}},
					},
				},
			}},
		},
		params: v1.Params{{Name: "HELLO", Value: *v1.NewStructuredValues("pipeline", "param!")}},
		expected: v1.PipelineSpec{
			Finally: []v1.PipelineTask{{
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Params: []v1.ParamSpec{{
							Name:    "HELLO",
							Default: v1.NewStructuredValues("default", "param!"),
						}},
						Steps: []v1.Step{{
							Name:  "step1",
							Image: "ubuntu",
							Args:  []string{"#!/usr/bin/env bash\n", "echo", "pipeline", "param!"},
						}},
					},
				},
			}},
		},
	}, {
		name: "parameter propagation array with task default and task winner task",
		original: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "HELLO", Value: *v1.NewStructuredValues("task", "param!")},
				},
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Params: []v1.ParamSpec{{
							Name:    "HELLO",
							Default: v1.NewStructuredValues("default", "param!"),
						}},
						Steps: []v1.Step{{
							Name:  "step1",
							Image: "ubuntu",
							Args:  []string{"#!/usr/bin/env bash\n", "echo", "$(params.HELLO)"},
						}},
					},
				},
			}},
		},
		params: v1.Params{{Name: "HELLO", Value: *v1.NewStructuredValues("pipeline", "param!")}},
		expected: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "HELLO", Value: *v1.NewStructuredValues("task", "param!")},
				},
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Params: []v1.ParamSpec{{
							Name:    "HELLO",
							Default: v1.NewStructuredValues("default", "param!"),
						}},
						Steps: []v1.Step{{
							Name:  "step1",
							Image: "ubuntu",
							Args:  []string{"#!/usr/bin/env bash\n", "echo", "task", "param!"},
						}},
					},
				},
			}},
		},
	}, {
		name: "Finally task parameter propagation array with task default and task winner task",
		original: v1.PipelineSpec{
			Finally: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "HELLO", Value: *v1.NewStructuredValues("task", "param!")},
				},
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Params: []v1.ParamSpec{{
							Name:    "HELLO",
							Default: v1.NewStructuredValues("default", "param!"),
						}},
						Steps: []v1.Step{{
							Name:  "step1",
							Image: "ubuntu",
							Args:  []string{"#!/usr/bin/env bash\n", "echo", "$(params.HELLO)"},
						}},
					},
				},
			}},
		},
		params: v1.Params{{Name: "HELLO", Value: *v1.NewStructuredValues("pipeline", "param!")}},
		expected: v1.PipelineSpec{
			Finally: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "HELLO", Value: *v1.NewStructuredValues("task", "param!")},
				},
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Params: []v1.ParamSpec{{
							Name:    "HELLO",
							Default: v1.NewStructuredValues("default", "param!"),
						}},
						Steps: []v1.Step{{
							Name:  "step1",
							Image: "ubuntu",
							Args:  []string{"#!/usr/bin/env bash\n", "echo", "task", "param!"},
						}},
					},
				},
			}},
		},
	}, {
		name: "parameter propagation with task default and task winner task",
		original: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "HELLO", Value: *v1.NewStructuredValues("task param!")},
				},
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Params: []v1.ParamSpec{{
							Name:    "HELLO",
							Default: v1.NewStructuredValues("default param!"),
						}},
						Steps: []v1.Step{{
							Name:   "step1",
							Image:  "ubuntu",
							Script: `#!/usr/bin/env bash\necho "$(params.HELLO)"`,
						}},
					},
				},
			}},
		},
		params: v1.Params{{Name: "HELLO", Value: *v1.NewStructuredValues("pipeline param!")}},
		expected: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "HELLO", Value: *v1.NewStructuredValues("task param!")},
				},
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Params: []v1.ParamSpec{{
							Name:    "HELLO",
							Default: v1.NewStructuredValues("default param!"),
						}},
						Steps: []v1.Step{{
							Name:   "step1",
							Image:  "ubuntu",
							Script: `#!/usr/bin/env bash\necho "task param!"`,
						}},
					},
				},
			}},
		},
	}, {
		name: "Finally task parameter propagation with task default and task winner task",
		original: v1.PipelineSpec{
			Finally: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "HELLO", Value: *v1.NewStructuredValues("task param!")},
				},
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Params: []v1.ParamSpec{{
							Name:    "HELLO",
							Default: v1.NewStructuredValues("default param!"),
						}},
						Steps: []v1.Step{{
							Name:   "step1",
							Image:  "ubuntu",
							Script: `#!/usr/bin/env bash\necho "$(params.HELLO)"`,
						}},
					},
				},
			}},
		},
		params: v1.Params{{Name: "HELLO", Value: *v1.NewStructuredValues("pipeline param!")}},
		expected: v1.PipelineSpec{
			Finally: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "HELLO", Value: *v1.NewStructuredValues("task param!")},
				},
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Params: []v1.ParamSpec{{
							Name:    "HELLO",
							Default: v1.NewStructuredValues("default param!"),
						}},
						Steps: []v1.Step{{
							Name:   "step1",
							Image:  "ubuntu",
							Script: `#!/usr/bin/env bash\necho "task param!"`,
						}},
					},
				},
			}},
		},
	}, {
		name: "parameter propagation object with task default but no task winner pipeline",
		original: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Params: []v1.ParamSpec{{
							Name: "myobject",
							Properties: map[string]v1.PropertySpec{
								"key1": {Type: "string"},
								"key2": {Type: "string"},
							},
							Default: v1.NewObject(map[string]string{
								"key1": "default",
								"key2": "param!",
							}),
						}},
						Steps: []v1.Step{{
							Name:  "step1",
							Image: "ubuntu",
							Args:  []string{"#!/usr/bin/env bash\n", "echo", "$(params.myobject.key1) $(params.myobject.key2)"},
						}},
					},
				},
			}},
		},
		params: v1.Params{{Name: "myobject", Value: *v1.NewObject(map[string]string{
			"key1": "pipeline",
			"key2": "param!!",
		})}},
		expected: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Params: []v1.ParamSpec{{
							Name: "myobject",
							Properties: map[string]v1.PropertySpec{
								"key1": {Type: "string"},
								"key2": {Type: "string"},
							},
							Default: v1.NewObject(map[string]string{
								"key1": "default",
								"key2": "param!",
							}),
						}},
						Steps: []v1.Step{{
							Name:  "step1",
							Image: "ubuntu",
							Args:  []string{"#!/usr/bin/env bash\n", "echo", "pipeline param!!"},
						}},
					},
				},
			}},
		},
		wc: config.EnableAlphaAPIFields,
	}, {
		name: "Finally task parameter propagation object with task default but no task winner pipeline",
		original: v1.PipelineSpec{
			Finally: []v1.PipelineTask{{
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Params: []v1.ParamSpec{{
							Name: "myobject",
							Properties: map[string]v1.PropertySpec{
								"key1": {Type: "string"},
								"key2": {Type: "string"},
							},
							Default: v1.NewObject(map[string]string{
								"key1": "default",
								"key2": "param!",
							}),
						}},
						Steps: []v1.Step{{
							Name:  "step1",
							Image: "ubuntu",
							Args:  []string{"#!/usr/bin/env bash\n", "echo", "$(params.myobject.key1) $(params.myobject.key2)"},
						}},
					},
				},
			}},
		},
		params: v1.Params{{Name: "myobject", Value: *v1.NewObject(map[string]string{
			"key1": "pipeline",
			"key2": "param!!",
		})}},
		expected: v1.PipelineSpec{
			Finally: []v1.PipelineTask{{
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Params: []v1.ParamSpec{{
							Name: "myobject",
							Properties: map[string]v1.PropertySpec{
								"key1": {Type: "string"},
								"key2": {Type: "string"},
							},
							Default: v1.NewObject(map[string]string{
								"key1": "default",
								"key2": "param!",
							}),
						}},
						Steps: []v1.Step{{
							Name:  "step1",
							Image: "ubuntu",
							Args:  []string{"#!/usr/bin/env bash\n", "echo", "pipeline param!!"},
						}},
					},
				},
			}},
		},
		wc: config.EnableAlphaAPIFields,
	}, {
		name: "parameter propagation object with task default and task winner task",
		original: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "myobject", Value: *v1.NewObject(map[string]string{
						"key1": "task",
						"key2": "param!",
					})},
				},
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Params: []v1.ParamSpec{{
							Name: "myobject",
							Properties: map[string]v1.PropertySpec{
								"key1": {Type: "string"},
								"key2": {Type: "string"},
							},
							Default: v1.NewObject(map[string]string{
								"key1": "default",
								"key2": "param!!",
							}),
						}},
						Steps: []v1.Step{{
							Name:  "step1",
							Image: "ubuntu",
							Args:  []string{"#!/usr/bin/env bash\n", "echo", "$(params.myobject.key1) $(params.myobject.key2)"},
						}},
					},
				},
			}},
		},
		params: v1.Params{{Name: "myobject", Value: *v1.NewObject(map[string]string{"key1": "pipeline", "key2": "param!!!"})}},
		expected: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "myobject", Value: *v1.NewObject(map[string]string{
						"key1": "task",
						"key2": "param!",
					})},
				},
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Params: []v1.ParamSpec{{
							Name: "myobject",
							Properties: map[string]v1.PropertySpec{
								"key1": {Type: "string"},
								"key2": {Type: "string"},
							},
							Default: v1.NewObject(map[string]string{
								"key1": "default",
								"key2": "param!!",
							}),
						}},
						Steps: []v1.Step{{
							Name:  "step1",
							Image: "ubuntu",
							Args:  []string{"#!/usr/bin/env bash\n", "echo", "task param!"},
						}},
					},
				},
			}},
		},
		wc: config.EnableAlphaAPIFields,
	}, {
		name: "Finally task parameter propagation object with task default and task winner task",
		original: v1.PipelineSpec{
			Finally: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "myobject", Value: *v1.NewObject(map[string]string{
						"key1": "task",
						"key2": "param!",
					})},
				},
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Params: []v1.ParamSpec{{
							Name: "myobject",
							Properties: map[string]v1.PropertySpec{
								"key1": {Type: "string"},
								"key2": {Type: "string"},
							},
							Default: v1.NewObject(map[string]string{
								"key1": "default",
								"key2": "param!!",
							}),
						}},
						Steps: []v1.Step{{
							Name:  "step1",
							Image: "ubuntu",
							Args:  []string{"#!/usr/bin/env bash\n", "echo", "$(params.myobject.key1) $(params.myobject.key2)"},
						}},
					},
				},
			}},
		},
		params: v1.Params{{Name: "myobject", Value: *v1.NewObject(map[string]string{"key1": "pipeline", "key2": "param!!!"})}},
		expected: v1.PipelineSpec{
			Finally: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "myobject", Value: *v1.NewObject(map[string]string{
						"key1": "task",
						"key2": "param!",
					})},
				},
				TaskSpec: &v1.EmbeddedTask{
					TaskSpec: v1.TaskSpec{
						Params: []v1.ParamSpec{{
							Name: "myobject",
							Properties: map[string]v1.PropertySpec{
								"key1": {Type: "string"},
								"key2": {Type: "string"},
							},
							Default: v1.NewObject(map[string]string{
								"key1": "default",
								"key2": "param!!",
							}),
						}},
						Steps: []v1.Step{{
							Name:  "step1",
							Image: "ubuntu",
							Args:  []string{"#!/usr/bin/env bash\n", "echo", "task param!"},
						}},
					},
				},
			}},
		},
		wc: config.EnableAlphaAPIFields,
	}, {
		name: "single parameter with when expression",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
				{Name: "second-param", Type: v1.ParamTypeString},
			},
			Tasks: []v1.PipelineTask{{
				When: []v1.WhenExpression{{
					Input:    "$(params.first-param)",
					Operator: selection.In,
					Values:   []string{"$(params.second-param)"},
				}},
			}},
		},
		params: v1.Params{{Name: "second-param", Value: *v1.NewStructuredValues("second-value")}},
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
				{Name: "second-param", Type: v1.ParamTypeString},
			},
			Tasks: []v1.PipelineTask{{
				When: []v1.WhenExpression{{
					Input:    "default-value",
					Operator: selection.In,
					Values:   []string{"second-value"},
				}},
			}},
		},
	}, {
		name: "object parameter with when expression",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{
					Name: "myobject",
					Type: v1.ParamTypeObject,
					Properties: map[string]v1.PropertySpec{
						"key1": {Type: "string"},
						"key2": {Type: "string"},
						"key3": {Type: "string"},
					},
					Default: v1.NewObject(map[string]string{
						"key1": "val1",
						"key2": "val2",
						"key3": "val3",
					}),
				},
			},
			Tasks: []v1.PipelineTask{{
				When: []v1.WhenExpression{{
					Input:    "$(params.myobject.key1)",
					Operator: selection.In,
					Values:   []string{"$(params.myobject.key2)", "$(params.myobject.key3)"},
				}},
			}},
		},
		params: v1.Params{{Name: "myobject", Value: *v1.NewObject(map[string]string{
			"key1": "val1",
			"key2": "val2",
			"key3": "val1",
		})}},
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{
					Name: "myobject",
					Type: v1.ParamTypeObject,
					Properties: map[string]v1.PropertySpec{
						"key1": {Type: "string"},
						"key2": {Type: "string"},
						"key3": {Type: "string"},
					},
					Default: v1.NewObject(map[string]string{
						"key1": "val1",
						"key2": "val2",
						"key3": "val3",
					}),
				},
			},
			Tasks: []v1.PipelineTask{{
				When: []v1.WhenExpression{{
					Input:    "val1",
					Operator: selection.In,
					Values:   []string{"val2", "val1"},
				}},
			}},
		},
	}, {
		name: "string pipeline parameter nested inside task parameter",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
				{Name: "second-param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues("$(input.workspace.$(params.first-param))")},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues("$(input.workspace.$(params.second-param))")},
				},
			}},
		},
		params: nil, // no parameter values.
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
				{Name: "second-param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues("$(input.workspace.default-value)")},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues("$(input.workspace.default-value)")},
				},
			}},
		},
	}, {
		name: "array pipeline parameter nested inside task parameter",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeArray, Default: v1.NewStructuredValues("default", "array", "value")},
				{Name: "second-param", Type: v1.ParamTypeArray},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues("firstelement", "$(params.first-param)")},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues("firstelement", "$(params.second-param)")},
				},
			}},
		},
		params: v1.Params{
			{Name: "second-param", Value: *v1.NewStructuredValues("second-value", "array")},
		},
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeArray, Default: v1.NewStructuredValues("default", "array", "value")},
				{Name: "second-param", Type: v1.ParamTypeArray},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues("firstelement", "default", "array", "value")},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues("firstelement", "second-value", "array")},
				},
			}},
		},
	}, {
		name: "object pipeline parameter nested inside task parameter",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{
					Name: "myobject",
					Type: v1.ParamTypeObject,
					Properties: map[string]v1.PropertySpec{
						"key1": {Type: "string"},
						"key2": {Type: "string"},
					},
					Default: v1.NewObject(map[string]string{
						"key1": "val1",
						"key2": "val2",
					}),
				},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues("$(input.workspace.$(params.myobject.key1))")},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues("$(input.workspace.$(params.myobject.key2))")},
				},
			}},
		},
		params: nil, // no parameter values.
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{
					Name: "myobject",
					Type: v1.ParamTypeObject,
					Properties: map[string]v1.PropertySpec{
						"key1": {Type: "string"},
						"key2": {Type: "string"},
					},
					Default: v1.NewObject(map[string]string{
						"key1": "val1",
						"key2": "val2",
					}),
				},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues("$(input.workspace.val1)")},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues("$(input.workspace.val2)")},
				},
			}},
		},
	}, {
		name: "parameter evaluation with final tasks",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
				{Name: "second-param", Type: v1.ParamTypeString},
			},
			Finally: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "final-task-first-param", Value: *v1.NewStructuredValues("$(params.first-param)")},
					{Name: "final-task-second-param", Value: *v1.NewStructuredValues("$(params.second-param)")},
				},
				When: v1.WhenExpressions{{
					Input:    "$(params.first-param)",
					Operator: selection.In,
					Values:   []string{"$(params.second-param)"},
				}},
			}},
		},
		params: v1.Params{{Name: "second-param", Value: *v1.NewStructuredValues("second-value")}},
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
				{Name: "second-param", Type: v1.ParamTypeString},
			},
			Finally: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "final-task-first-param", Value: *v1.NewStructuredValues("default-value")},
					{Name: "final-task-second-param", Value: *v1.NewStructuredValues("second-value")},
				},
				When: v1.WhenExpressions{{
					Input:    "default-value",
					Operator: selection.In,
					Values:   []string{"second-value"},
				}},
			}},
		},
	}, {
		name: "parameter evaluation with both tasks and final tasks",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
				{Name: "second-param", Type: v1.ParamTypeString},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "final-task-first-param", Value: *v1.NewStructuredValues("$(params.first-param)")},
					{Name: "final-task-second-param", Value: *v1.NewStructuredValues("$(params.second-param)")},
				},
			}},
			Finally: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "final-task-first-param", Value: *v1.NewStructuredValues("$(params.first-param)")},
					{Name: "final-task-second-param", Value: *v1.NewStructuredValues("$(params.second-param)")},
				},
				When: v1.WhenExpressions{{
					Input:    "$(params.first-param)",
					Operator: selection.In,
					Values:   []string{"$(params.second-param)"},
				}},
			}},
		},
		params: v1.Params{{Name: "second-param", Value: *v1.NewStructuredValues("second-value")}},
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
				{Name: "second-param", Type: v1.ParamTypeString},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "final-task-first-param", Value: *v1.NewStructuredValues("default-value")},
					{Name: "final-task-second-param", Value: *v1.NewStructuredValues("second-value")},
				},
			}},
			Finally: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "final-task-first-param", Value: *v1.NewStructuredValues("default-value")},
					{Name: "final-task-second-param", Value: *v1.NewStructuredValues("second-value")},
				},
				When: v1.WhenExpressions{{
					Input:    "default-value",
					Operator: selection.In,
					Values:   []string{"second-value"},
				}},
			}},
		},
	}, {
		name: "object parameter evaluation with both tasks and final tasks",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{
					Name: "myobject",
					Type: v1.ParamTypeObject,
					Properties: map[string]v1.PropertySpec{
						"key1": {Type: "string"},
						"key2": {Type: "string"},
					},
					Default: v1.NewObject(map[string]string{
						"key1": "val1",
						"key2": "val2",
					}),
				},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "final-task-first-param", Value: *v1.NewStructuredValues("$(params.myobject.key1)")},
					{Name: "final-task-second-param", Value: *v1.NewStructuredValues("$(params.myobject.key2)")},
				},
			}},
			Finally: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "final-task-first-param", Value: *v1.NewStructuredValues("$(params.myobject.key1)")},
					{Name: "final-task-second-param", Value: *v1.NewStructuredValues("$(params.myobject.key2)")},
				},
				When: v1.WhenExpressions{{
					Input:    "$(params.myobject.key1)",
					Operator: selection.In,
					Values:   []string{"$(params.myobject.key2)"},
				}},
			}},
		},
		params: v1.Params{{Name: "myobject", Value: *v1.NewObject(map[string]string{
			"key1": "foo",
			"key2": "bar",
		})}},
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{
					Name: "myobject",
					Type: v1.ParamTypeObject,
					Properties: map[string]v1.PropertySpec{
						"key1": {Type: "string"},
						"key2": {Type: "string"},
					},
					Default: v1.NewObject(map[string]string{
						"key1": "val1",
						"key2": "val2",
					}),
				},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "final-task-first-param", Value: *v1.NewStructuredValues("foo")},
					{Name: "final-task-second-param", Value: *v1.NewStructuredValues("bar")},
				},
			}},
			Finally: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "final-task-first-param", Value: *v1.NewStructuredValues("foo")},
					{Name: "final-task-second-param", Value: *v1.NewStructuredValues("bar")},
				},
				When: v1.WhenExpressions{{
					Input:    "foo",
					Operator: selection.In,
					Values:   []string{"bar"},
				}},
			}},
		},
	}, {
		name: "parameter references with bracket notation and special characters",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first.param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
				{Name: "second/param", Type: v1.ParamTypeString},
				{Name: "third.param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
				{Name: "fourth/param", Type: v1.ParamTypeString},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues(`$(params["first.param"])`)},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues(`$(params["second/param"])`)},
					{Name: "first-task-third-param", Value: *v1.NewStructuredValues(`$(params['third.param'])`)},
					{Name: "first-task-fourth-param", Value: *v1.NewStructuredValues(`$(params['fourth/param'])`)},
					{Name: "first-task-fifth-param", Value: *v1.NewStructuredValues("static value")},
				},
			}},
		},
		params: v1.Params{
			{Name: "second/param", Value: *v1.NewStructuredValues("second-value")},
			{Name: "fourth/param", Value: *v1.NewStructuredValues("fourth-value")},
		},
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first.param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
				{Name: "second/param", Type: v1.ParamTypeString},
				{Name: "third.param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
				{Name: "fourth/param", Type: v1.ParamTypeString},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues("default-value")},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues("second-value")},
					{Name: "first-task-third-param", Value: *v1.NewStructuredValues("default-value")},
					{Name: "first-task-fourth-param", Value: *v1.NewStructuredValues("fourth-value")},
					{Name: "first-task-fifth-param", Value: *v1.NewStructuredValues("static value")},
				},
			}},
		},
	}, {
		name: "single parameter in workspace subpath",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
				{Name: "second-param", Type: v1.ParamTypeString},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues("$(params.first-param)")},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues("static value")},
				},
				Workspaces: []v1.WorkspacePipelineTaskBinding{
					{
						Name:      "first-workspace",
						Workspace: "first-workspace",
						SubPath:   "$(params.second-param)",
					},
				},
			}},
		},
		params: v1.Params{{Name: "second-param", Value: *v1.NewStructuredValues("second-value")}},
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
				{Name: "second-param", Type: v1.ParamTypeString},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues("default-value")},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues("static value")},
				},
				Workspaces: []v1.WorkspacePipelineTaskBinding{
					{
						Name:      "first-workspace",
						Workspace: "first-workspace",
						SubPath:   "second-value",
					},
				},
			}},
		},
	}, {
		name: "object parameter in workspace subpath",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{
					Name: "myobject",
					Type: v1.ParamTypeObject,
					Properties: map[string]v1.PropertySpec{
						"key1": {Type: "string"},
						"key2": {Type: "string"},
					},
					Default: v1.NewObject(map[string]string{
						"key1": "val1",
						"key2": "val2",
					}),
				},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues("$(params.myobject.key1)")},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues("static value")},
				},
				Workspaces: []v1.WorkspacePipelineTaskBinding{
					{
						Name:      "first-workspace",
						Workspace: "first-workspace",
						SubPath:   "$(params.myobject.key2)",
					},
				},
			}},
		},
		params: v1.Params{{Name: "myobject", Value: *v1.NewObject(map[string]string{
			"key1": "foo",
			"key2": "bar",
		})}},
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{
					Name: "myobject",
					Type: v1.ParamTypeObject,
					Properties: map[string]v1.PropertySpec{
						"key1": {Type: "string"},
						"key2": {Type: "string"},
					},
					Default: v1.NewObject(map[string]string{
						"key1": "val1",
						"key2": "val2",
					}),
				},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues("foo")},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues("static value")},
				},
				Workspaces: []v1.WorkspacePipelineTaskBinding{
					{
						Name:      "first-workspace",
						Workspace: "first-workspace",
						SubPath:   "bar",
					},
				},
			}},
		},
	}, {
		name: "single parameter with resolver",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
				{Name: "second-param", Type: v1.ParamTypeString},
			},
			Tasks: []v1.PipelineTask{{
				TaskRef: &v1.TaskRef{
					ResolverRef: v1.ResolverRef{
						Params: v1.Params{{
							Name:  "first-resolver-param",
							Value: *v1.NewStructuredValues("$(params.first-param)"),
						}, {
							Name:  "second-resolver-param",
							Value: *v1.NewStructuredValues("$(params.second-param)"),
						}},
					},
				},
			}},
		},
		params: v1.Params{{Name: "second-param", Value: *v1.NewStructuredValues("second-value")}},
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
				{Name: "second-param", Type: v1.ParamTypeString},
			},
			Tasks: []v1.PipelineTask{{
				TaskRef: &v1.TaskRef{
					ResolverRef: v1.ResolverRef{
						Params: v1.Params{{
							Name:  "first-resolver-param",
							Value: *v1.NewStructuredValues("default-value"),
						}, {
							Name:  "second-resolver-param",
							Value: *v1.NewStructuredValues("second-value"),
						}},
					},
				},
			}},
		},
	}, {
		name: "object parameter with resolver",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{
					Name: "myobject",
					Type: v1.ParamTypeObject,
					Properties: map[string]v1.PropertySpec{
						"key1": {Type: "string"},
						"key2": {Type: "string"},
						"key3": {Type: "string"},
					},
					Default: v1.NewObject(map[string]string{
						"key1": "val1",
						"key2": "val2",
						"key3": "val3",
					}),
				},
			},
			Tasks: []v1.PipelineTask{{
				TaskRef: &v1.TaskRef{
					ResolverRef: v1.ResolverRef{
						Params: v1.Params{{
							Name:  "first-resolver-param",
							Value: *v1.NewStructuredValues("$(params.myobject.key1)"),
						}, {
							Name:  "second-resolver-param",
							Value: *v1.NewStructuredValues("$(params.myobject.key2)"),
						}, {
							Name:  "third-resolver-param",
							Value: *v1.NewStructuredValues("$(params.myobject.key3)"),
						}},
					},
				},
			}},
		},
		params: v1.Params{{Name: "myobject", Value: *v1.NewObject(map[string]string{
			"key1": "val1",
			"key2": "val2",
			"key3": "val1",
		})}},
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{
					Name: "myobject",
					Type: v1.ParamTypeObject,
					Properties: map[string]v1.PropertySpec{
						"key1": {Type: "string"},
						"key2": {Type: "string"},
						"key3": {Type: "string"},
					},
					Default: v1.NewObject(map[string]string{
						"key1": "val1",
						"key2": "val2",
						"key3": "val3",
					}),
				},
			},
			Tasks: []v1.PipelineTask{{
				TaskRef: &v1.TaskRef{
					ResolverRef: v1.ResolverRef{
						Params: v1.Params{{
							Name:  "first-resolver-param",
							Value: *v1.NewStructuredValues("val1"),
						}, {
							Name:  "second-resolver-param",
							Value: *v1.NewStructuredValues("val2"),
						}, {
							Name:  "third-resolver-param",
							Value: *v1.NewStructuredValues("val1"),
						}},
					},
				},
			}},
		},
		wc: config.EnableAlphaAPIFields,
	}, {
		name: "single parameter in finally workspace subpath",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
				{Name: "second-param", Type: v1.ParamTypeString},
			},
			Finally: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues("$(params.first-param)")},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues("static value")},
				},
				Workspaces: []v1.WorkspacePipelineTaskBinding{
					{
						Name:      "first-workspace",
						Workspace: "first-workspace",
						SubPath:   "$(params.second-param)",
					},
				},
			}},
		},
		params: v1.Params{{Name: "second-param", Value: *v1.NewStructuredValues("second-value")}},
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
				{Name: "second-param", Type: v1.ParamTypeString},
			},
			Finally: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues("default-value")},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues("static value")},
				},
				Workspaces: []v1.WorkspacePipelineTaskBinding{
					{
						Name:      "first-workspace",
						Workspace: "first-workspace",
						SubPath:   "second-value",
					},
				},
			}},
		},
	},
		{
			name: "tasks with the same parameter name but referencing different values",
			original: v1.PipelineSpec{
				Params: []v1.ParamSpec{
					{
						Name: "param1",
						Default: &v1.ParamValue{
							Type:      v1.ParamTypeString,
							StringVal: "a",
						},
						Type: v1.ParamTypeString,
					},
				},
				Tasks: []v1.PipelineTask{
					{
						Name: "previous-task-with-result",
					},
					{
						Name: "print-msg",
						TaskSpec: &v1.EmbeddedTask{
							TaskSpec: v1.TaskSpec{
								Params: []v1.ParamSpec{
									{
										Name: "param1",
										Type: v1.ParamTypeString,
									},
								},
							},
						},
						Params: v1.Params{
							{
								Name: "param1",
								Value: v1.ParamValue{
									Type:      v1.ParamTypeString,
									StringVal: "$(tasks.previous-task-with-result.results.Output)",
								},
							},
						},
					},
					{
						Name: "print-msg-2",
						TaskSpec: &v1.EmbeddedTask{
							TaskSpec: v1.TaskSpec{
								Params: []v1.ParamSpec{
									{
										Name: "param1",
										Type: v1.ParamTypeString,
									},
								},
							},
						},
						Params: v1.Params{
							{
								Name: "param1",
								Value: v1.ParamValue{
									Type:      v1.ParamTypeString,
									StringVal: "$(params.param1)",
								},
							},
						},
					},
				},
			},
			expected: v1.PipelineSpec{
				Params: []v1.ParamSpec{
					{
						Name: "param1",
						Default: &v1.ParamValue{
							Type:      v1.ParamTypeString,
							StringVal: "a",
						},
						Type: v1.ParamTypeString,
					},
				},
				Tasks: []v1.PipelineTask{
					{
						Name: "previous-task-with-result",
					},
					{
						Name: "print-msg",
						TaskSpec: &v1.EmbeddedTask{
							TaskSpec: v1.TaskSpec{
								Params: []v1.ParamSpec{
									{
										Name: "param1",
										Type: v1.ParamTypeString,
									},
								},
							},
						},
						Params: v1.Params{
							{
								Name: "param1",
								Value: v1.ParamValue{
									Type:      v1.ParamTypeString,
									StringVal: "$(tasks.previous-task-with-result.results.Output)",
								},
							},
						},
					},
					{
						Name: "print-msg-2",
						TaskSpec: &v1.EmbeddedTask{
							TaskSpec: v1.TaskSpec{
								Params: []v1.ParamSpec{
									{
										Name: "param1",
										Type: v1.ParamTypeString,
									},
								},
							},
						},
						Params: v1.Params{
							{
								Name: "param1",
								Value: v1.ParamValue{
									Type:      v1.ParamTypeString,
									StringVal: "a",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "finally tasks with the same parameter name but referencing different values",
			original: v1.PipelineSpec{
				Params: []v1.ParamSpec{
					{
						Name: "param1",
						Default: &v1.ParamValue{
							Type:      v1.ParamTypeString,
							StringVal: "a",
						},
						Type: v1.ParamTypeString,
					},
				},
				Tasks: []v1.PipelineTask{
					{
						Name: "previous-task-with-result",
					},
				},
				Finally: []v1.PipelineTask{
					{
						Name: "print-msg",
						TaskSpec: &v1.EmbeddedTask{
							TaskSpec: v1.TaskSpec{
								Params: []v1.ParamSpec{
									{
										Name: "param1",
										Type: v1.ParamTypeString,
									},
								},
							},
						},
						Params: v1.Params{
							{
								Name: "param1",
								Value: v1.ParamValue{
									Type:      v1.ParamTypeString,
									StringVal: "$(tasks.previous-task-with-result.results.Output)",
								},
							},
						},
					},
					{
						Name: "print-msg-2",
						TaskSpec: &v1.EmbeddedTask{
							TaskSpec: v1.TaskSpec{
								Params: []v1.ParamSpec{
									{
										Name: "param1",
										Type: v1.ParamTypeString,
									},
								},
							},
						},
						Params: v1.Params{
							{
								Name: "param1",
								Value: v1.ParamValue{
									Type:      v1.ParamTypeString,
									StringVal: "$(params.param1)",
								},
							},
						},
					},
				},
			},
			expected: v1.PipelineSpec{
				Params: []v1.ParamSpec{
					{
						Name: "param1",
						Default: &v1.ParamValue{
							Type:      v1.ParamTypeString,
							StringVal: "a",
						},
						Type: v1.ParamTypeString,
					},
				},
				Tasks: []v1.PipelineTask{
					{
						Name: "previous-task-with-result",
					},
				},
				Finally: []v1.PipelineTask{
					{
						Name: "print-msg",
						TaskSpec: &v1.EmbeddedTask{
							TaskSpec: v1.TaskSpec{
								Params: []v1.ParamSpec{
									{
										Name: "param1",
										Type: v1.ParamTypeString,
									},
								},
							},
						},
						Params: v1.Params{
							{
								Name: "param1",
								Value: v1.ParamValue{
									Type:      v1.ParamTypeString,
									StringVal: "$(tasks.previous-task-with-result.results.Output)",
								},
							},
						},
					},
					{
						Name: "print-msg-2",
						TaskSpec: &v1.EmbeddedTask{
							TaskSpec: v1.TaskSpec{
								Params: []v1.ParamSpec{
									{
										Name: "param1",
										Type: v1.ParamTypeString,
									},
								},
							},
						},
						Params: v1.Params{
							{
								Name: "param1",
								Value: v1.ParamValue{
									Type:      v1.ParamTypeString,
									StringVal: "a",
								},
							},
						},
					},
				},
			},
		},
	} {
		tt := tt // capture range variable
		ctx := context.Background()
		if tt.wc != nil {
			ctx = tt.wc(ctx)
		}
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			run := &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Params: tt.params,
				},
			}
			got := resources.ApplyParameters(ctx, &tt.original, run)
			if d := cmp.Diff(&tt.expected, got); d != "" {
				t.Errorf("ApplyParameters() got diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestApplyParameters_ArrayIndexing(t *testing.T) {
	for _, tt := range []struct {
		name     string
		original v1.PipelineSpec
		params   []v1.Param
		expected v1.PipelineSpec
	}{{
		name: "single parameter",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeArray, Default: v1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1.ParamTypeString},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues("$(params.first-param[1])")},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues("$(params.second-param[0])")},
					{Name: "first-task-third-param", Value: *v1.NewStructuredValues("static value")},
				},
			}},
		},
		params: v1.Params{{Name: "second-param", Value: *v1.NewStructuredValues("second-value", "second-value-again")}},
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeArray, Default: v1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1.ParamTypeString},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues("default-value-again")},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues("second-value")},
					{Name: "first-task-third-param", Value: *v1.NewStructuredValues("static value")},
				},
			}},
		},
	}, {
		name: "single parameter with when expression",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeArray, Default: v1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1.ParamTypeString},
			},
			Tasks: []v1.PipelineTask{{
				When: []v1.WhenExpression{{
					Input:    "$(params.first-param[1])",
					Operator: selection.In,
					Values:   []string{"$(params.second-param[0])"},
				}},
			}},
		},
		params: v1.Params{{Name: "second-param", Value: *v1.NewStructuredValues("second-value", "second-value-again")}},
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeArray, Default: v1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1.ParamTypeString},
			},
			Tasks: []v1.PipelineTask{{
				When: []v1.WhenExpression{{
					Input:    "default-value-again",
					Operator: selection.In,
					Values:   []string{"second-value"},
				}},
			}},
		},
	}, {
		name: "pipeline parameter nested inside task parameter",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeArray, Default: v1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1.ParamTypeArray, Default: v1.NewStructuredValues("default-value", "default-value-again")},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues("$(input.workspace.$(params.first-param[0]))")},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues("$(input.workspace.$(params.second-param[1]))")},
				},
			}},
		},
		params: nil, // no parameter values.
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeArray, Default: v1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1.ParamTypeArray, Default: v1.NewStructuredValues("default-value", "default-value-again")},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues("$(input.workspace.default-value)")},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues("$(input.workspace.default-value-again)")},
				},
			}},
		},
	}, {
		name: "array parameter",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeArray, Default: v1.NewStructuredValues("default", "array", "value")},
				{Name: "second-param", Type: v1.ParamTypeArray},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues("firstelement", "$(params.first-param)")},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues("firstelement", "$(params.second-param[0])")},
				},
			}},
		},
		params: v1.Params{
			{Name: "second-param", Value: *v1.NewStructuredValues("second-value", "array")},
		},
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeArray, Default: v1.NewStructuredValues("default", "array", "value")},
				{Name: "second-param", Type: v1.ParamTypeArray},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues("firstelement", "default", "array", "value")},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues("firstelement", "second-value")},
				},
			}},
		},
	}, {
		name: "parameter evaluation with final tasks",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeArray, Default: v1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1.ParamTypeArray},
			},
			Finally: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "final-task-first-param", Value: *v1.NewStructuredValues("$(params.first-param[0])")},
					{Name: "final-task-second-param", Value: *v1.NewStructuredValues("$(params.second-param[1])")},
				},
				When: v1.WhenExpressions{{
					Input:    "$(params.first-param[0])",
					Operator: selection.In,
					Values:   []string{"$(params.second-param[1])"},
				}},
			}},
		},
		params: v1.Params{{Name: "second-param", Value: *v1.NewStructuredValues("second-value", "second-value-again")}},
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeArray, Default: v1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1.ParamTypeArray},
			},
			Finally: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "final-task-first-param", Value: *v1.NewStructuredValues("default-value")},
					{Name: "final-task-second-param", Value: *v1.NewStructuredValues("second-value-again")},
				},
				When: v1.WhenExpressions{{
					Input:    "default-value",
					Operator: selection.In,
					Values:   []string{"second-value-again"},
				}},
			}},
		},
	}, {
		name: "parameter evaluation with both tasks and final tasks",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeArray, Default: v1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1.ParamTypeArray},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "final-task-first-param", Value: *v1.NewStructuredValues("$(params.first-param[0])")},
					{Name: "final-task-second-param", Value: *v1.NewStructuredValues("$(params.second-param[1])")},
				},
			}},
			Finally: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "final-task-first-param", Value: *v1.NewStructuredValues("$(params.first-param[0])")},
					{Name: "final-task-second-param", Value: *v1.NewStructuredValues("$(params.second-param[1])")},
				},
				When: v1.WhenExpressions{{
					Input:    "$(params.first-param[0])",
					Operator: selection.In,
					Values:   []string{"$(params.second-param[1])"},
				}},
			}},
		},
		params: v1.Params{{Name: "second-param", Value: *v1.NewStructuredValues("second-value", "second-value-again")}},
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeArray, Default: v1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1.ParamTypeArray},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "final-task-first-param", Value: *v1.NewStructuredValues("default-value")},
					{Name: "final-task-second-param", Value: *v1.NewStructuredValues("second-value-again")},
				},
			}},
			Finally: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "final-task-first-param", Value: *v1.NewStructuredValues("default-value")},
					{Name: "final-task-second-param", Value: *v1.NewStructuredValues("second-value-again")},
				},
				When: v1.WhenExpressions{{
					Input:    "default-value",
					Operator: selection.In,
					Values:   []string{"second-value-again"},
				}},
			}},
		},
	}, {
		name: "parameter references with bracket notation and special characters",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first.param", Type: v1.ParamTypeArray, Default: v1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second/param", Type: v1.ParamTypeArray},
				{Name: "third.param", Type: v1.ParamTypeArray, Default: v1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "fourth/param", Type: v1.ParamTypeArray},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues(`$(params["first.param"][0])`)},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues(`$(params["second/param"][0])`)},
					{Name: "first-task-third-param", Value: *v1.NewStructuredValues(`$(params['third.param'][1])`)},
					{Name: "first-task-fourth-param", Value: *v1.NewStructuredValues(`$(params['fourth/param'][1])`)},
					{Name: "first-task-fifth-param", Value: *v1.NewStructuredValues("static value")},
				},
			}},
		},
		params: v1.Params{
			{Name: "second/param", Value: *v1.NewStructuredValues("second-value", "second-value-again")},
			{Name: "fourth/param", Value: *v1.NewStructuredValues("fourth-value", "fourth-value-again")},
		},
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first.param", Type: v1.ParamTypeArray, Default: v1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second/param", Type: v1.ParamTypeArray},
				{Name: "third.param", Type: v1.ParamTypeArray, Default: v1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "fourth/param", Type: v1.ParamTypeArray},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues("default-value")},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues("second-value")},
					{Name: "first-task-third-param", Value: *v1.NewStructuredValues("default-value-again")},
					{Name: "first-task-fourth-param", Value: *v1.NewStructuredValues("fourth-value-again")},
					{Name: "first-task-fifth-param", Value: *v1.NewStructuredValues("static value")},
				},
			}},
		},
	}, {
		name: "single parameter in workspace subpath",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeArray, Default: v1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1.ParamTypeArray},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues("$(params.first-param[0])")},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues("static value")},
				},
				Workspaces: []v1.WorkspacePipelineTaskBinding{
					{
						Name:      "first-workspace",
						Workspace: "first-workspace",
						SubPath:   "$(params.second-param[1])",
					},
				},
			}},
		},
		params: v1.Params{{Name: "second-param", Value: *v1.NewStructuredValues("second-value", "second-value-again")}},
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{
				{Name: "first-param", Type: v1.ParamTypeArray, Default: v1.NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: v1.ParamTypeArray},
			},
			Tasks: []v1.PipelineTask{{
				Params: v1.Params{
					{Name: "first-task-first-param", Value: *v1.NewStructuredValues("default-value")},
					{Name: "first-task-second-param", Value: *v1.NewStructuredValues("static value")},
				},
				Workspaces: []v1.WorkspacePipelineTaskBinding{
					{
						Name:      "first-workspace",
						Workspace: "first-workspace",
						SubPath:   "second-value-again",
					},
				},
			}},
		},
	},
	} {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			run := &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Params: tt.params,
				},
			}
			got := resources.ApplyParameters(context.Background(), &tt.original, run)
			if d := cmp.Diff(&tt.expected, got); d != "" {
				t.Errorf("ApplyParameters() got diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestApplyReplacementsMatrix(t *testing.T) {
	for _, tt := range []struct {
		name     string
		original v1.PipelineSpec
		params   []v1.Param
		expected v1.PipelineSpec
	}{{
		name: "matrix params replacements",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{{
				Name: "foo", Type: v1.ParamTypeString,
			}, {
				Name: "bar", Type: v1.ParamTypeArray,
			}, {
				Name: "rad", Type: v1.ParamTypeObject,
			}},
			Tasks: []v1.PipelineTask{{
				Matrix: &v1.Matrix{
					Params: v1.Params{{
						// string replacements from string param, array param and object param
						Name: "first-param", Value: *v1.NewStructuredValues("$(params.foo)", "$(params.bar[0])", "$(params.rad.key1)"),
					}, {
						// array replacement from array param
						Name: "second-param", Value: *v1.NewStructuredValues("$(params.bar)"),
					}},
				},
			}},
		},
		params: v1.Params{
			{Name: "foo", Value: *v1.NewStructuredValues("foo")},
			{Name: "bar", Value: *v1.NewStructuredValues("b", "a", "r")},
			{Name: "rad", Value: *v1.NewObject(map[string]string{
				"key1": "r",
				"key2": "a",
				"key3": "d",
			})},
		},
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{{
				Name: "foo", Type: v1.ParamTypeString,
			}, {
				Name: "bar", Type: v1.ParamTypeArray,
			}, {
				Name: "rad", Type: v1.ParamTypeObject,
			}},
			Tasks: []v1.PipelineTask{{
				Matrix: &v1.Matrix{
					Params: v1.Params{{
						// string replacements from string param, array param and object param
						Name: "first-param", Value: *v1.NewStructuredValues("foo", "b", "r"),
					}, {
						// array replacement from array param
						Name: "second-param", Value: *v1.NewStructuredValues("b", "a", "r"),
					}},
				},
			}},
		},
	}, {
		name: "matrix include params replacement",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{{
				Name: "foo", Type: v1.ParamTypeString,
			}, {
				Name: "bar", Type: v1.ParamTypeArray,
			}, {
				Name: "rad", Type: v1.ParamTypeObject,
			}},
			Tasks: []v1.PipelineTask{{
				Matrix: &v1.Matrix{
					Include: []v1.IncludeParams{{
						Name: "build-1",
						Params: v1.Params{{
							// string replacements from string param
							Name: "first-param", Value: *v1.NewStructuredValues("$(params.foo)"),
						}, {
							// string replacements from array param
							Name: "second-param", Value: *v1.NewStructuredValues("$(params.bar[0])"),
						}, {
							// string replacements from object param
							Name: "third-param", Value: *v1.NewStructuredValues("$(params.rad.key1)"),
						}},
					}},
				},
			}},
		},
		params: v1.Params{
			{Name: "foo", Value: *v1.NewStructuredValues("foo")},
			{Name: "bar", Value: *v1.NewStructuredValues("b", "a", "r")},
			{Name: "rad", Value: *v1.NewObject(map[string]string{
				"key1": "r",
				"key2": "a",
				"key3": "d",
			})},
		},
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{{
				Name: "foo", Type: v1.ParamTypeString,
			}, {
				Name: "bar", Type: v1.ParamTypeArray,
			}, {
				Name: "rad", Type: v1.ParamTypeObject,
			}},
			Tasks: []v1.PipelineTask{{
				Matrix: &v1.Matrix{
					Include: []v1.IncludeParams{{
						Name: "build-1",
						Params: v1.Params{{
							// string replacements from string param
							Name: "first-param", Value: *v1.NewStructuredValues("foo"),
						}, {
							// string replacements from array param
							Name: "second-param", Value: *v1.NewStructuredValues("b"),
						}, {
							// string replacements from object param
							Name: "third-param", Value: *v1.NewStructuredValues("r"),
						}},
					}},
				},
			}},
		},
	}, {
		name: "matrix params with final tasks",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{{
				Name: "foo", Type: v1.ParamTypeString,
			}, {
				Name: "bar", Type: v1.ParamTypeArray,
			}, {
				Name: "rad", Type: v1.ParamTypeObject,
			}},
			Finally: []v1.PipelineTask{{
				Matrix: &v1.Matrix{
					Params: v1.Params{{
						// string replacements from string param, array param and object param
						Name: "first-param", Value: *v1.NewStructuredValues("$(params.foo)", "$(params.bar[0])", "$(params.rad.key1)"),
					}, {
						// array replacement from array param
						Name: "second-param", Value: *v1.NewStructuredValues("$(params.bar)"),
					}},
				},
			}},
		},
		params: v1.Params{
			{Name: "foo", Value: *v1.NewStructuredValues("foo")},
			{Name: "bar", Value: *v1.NewStructuredValues("b", "a", "r")},
			{Name: "rad", Value: *v1.NewObject(map[string]string{
				"key1": "r",
				"key2": "a",
				"key3": "d",
			})},
		},
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{{
				Name: "foo", Type: v1.ParamTypeString,
			}, {
				Name: "bar", Type: v1.ParamTypeArray,
			}, {
				Name: "rad", Type: v1.ParamTypeObject,
			}},
			Finally: []v1.PipelineTask{{
				Matrix: &v1.Matrix{
					Params: v1.Params{{
						// string replacements from string param, array param and object param
						Name: "first-param", Value: *v1.NewStructuredValues("foo", "b", "r"),
					}, {
						// array replacement from array param
						Name: "second-param", Value: *v1.NewStructuredValues("b", "a", "r"),
					}},
				},
			}},
		},
	}, {
		name: "matrix include params with final tasks",
		original: v1.PipelineSpec{
			Params: []v1.ParamSpec{{
				Name: "foo", Type: v1.ParamTypeString,
			}, {
				Name: "bar", Type: v1.ParamTypeArray,
			}, {
				Name: "rad", Type: v1.ParamTypeObject,
			}},
			Finally: []v1.PipelineTask{{
				Matrix: &v1.Matrix{
					Include: []v1.IncludeParams{{
						Name: "build-1",
						Params: v1.Params{{
							// string replacements from string param
							Name: "first-param", Value: *v1.NewStructuredValues("$(params.foo)"),
						}, {
							// string replacements from array param
							Name: "second-param", Value: *v1.NewStructuredValues("$(params.bar[0])"),
						}, {
							// string replacements from object param
							Name: "third-param", Value: *v1.NewStructuredValues("$(params.rad.key1)"),
						}},
					}},
				},
			}},
		},
		params: v1.Params{
			{Name: "foo", Value: *v1.NewStructuredValues("foo")},
			{Name: "bar", Value: *v1.NewStructuredValues("b", "a", "r")},
			{Name: "rad", Value: *v1.NewObject(map[string]string{
				"key1": "r",
				"key2": "a",
				"key3": "d",
			})},
		},
		expected: v1.PipelineSpec{
			Params: []v1.ParamSpec{{
				Name: "foo", Type: v1.ParamTypeString,
			}, {
				Name: "bar", Type: v1.ParamTypeArray,
			}, {
				Name: "rad", Type: v1.ParamTypeObject,
			}},
			Finally: []v1.PipelineTask{{
				Matrix: &v1.Matrix{
					Include: []v1.IncludeParams{{
						Name: "build-1",
						Params: v1.Params{{
							// string replacements from string param
							Name: "first-param", Value: *v1.NewStructuredValues("foo"),
						}, {
							// string replacements from array param
							Name: "second-param", Value: *v1.NewStructuredValues("b"),
						}, {
							// string replacements from object param
							Name: "third-param", Value: *v1.NewStructuredValues("r"),
						}},
					}},
				},
			}},
		},
	},
	} {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			run := &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Params: tt.params,
				},
			}
			got := resources.ApplyParameters(context.Background(), &tt.original, run)
			if d := cmp.Diff(&tt.expected, got); d != "" {
				t.Errorf("ApplyParameters() got diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestApplyTaskResults_MinimalExpression(t *testing.T) {
	for _, tt := range []struct {
		name               string
		targets            resources.PipelineRunState
		resolvedResultRefs resources.ResolvedResultRefs
		want               resources.PipelineRunState
	}{{
		name: "Test result substitution on minimal variable substitution expression - params",
		resolvedResultRefs: resources.ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("aResultValue"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "a.Result",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Params: v1.Params{{
					Name:  "bParam",
					Value: *v1.NewStructuredValues(`$(tasks.aTask.results["a.Result"])`),
				}},
			},
		}},
		want: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Params: v1.Params{{
					Name:  "bParam",
					Value: *v1.NewStructuredValues("aResultValue"),
				}},
			},
		}},
	}, {
		name: "Test array indexing result substitution on minimal variable substitution expression - params",
		resolvedResultRefs: resources.ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("arrayResultValueOne", "arrayResultValueTwo"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "a.Result",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Params: v1.Params{{
					Name:  "bParam",
					Value: *v1.NewStructuredValues(`$(tasks.aTask.results["a.Result"][1])`),
				}},
			},
		}},
		want: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Params: v1.Params{{
					Name:  "bParam",
					Value: *v1.NewStructuredValues("arrayResultValueTwo"),
				}},
			},
		}},
	}, {
		name: "Test array indexing result substitution out of bound - params",
		resolvedResultRefs: resources.ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("arrayResultValueOne", "arrayResultValueTwo"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "a.Result",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Params: v1.Params{{
					Name:  "bParam",
					Value: *v1.NewStructuredValues(`$(tasks.aTask.results["a.Result"][3])`),
				}},
			},
		}},
		want: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Params: v1.Params{{
					Name: "bParam",
					// index validation is done in ResolveResultRefs() before ApplyTaskResults()
					Value: *v1.NewStructuredValues(`$(tasks.aTask.results["a.Result"][3])`),
				}},
			},
		}},
	}, {
		name: "Test array result substitution on minimal variable substitution expression - params",
		resolvedResultRefs: resources.ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("arrayResultValueOne", "arrayResultValueTwo"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "a.Result",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Params: v1.Params{{
					Name: "bParam",
					Value: v1.ParamValue{Type: v1.ParamTypeArray,
						ArrayVal: []string{`$(tasks.aTask.results["a.Result"][*])`},
					},
				}},
			},
		}},
		want: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Params: v1.Params{{
					Name:  "bParam",
					Value: *v1.NewStructuredValues("arrayResultValueOne", "arrayResultValueTwo"),
				}},
			},
		}},
	}, {
		name: "Test object result as a whole substitution - params",
		resolvedResultRefs: resources.ResolvedResultRefs{{
			Value: *v1.NewObject(map[string]string{
				"key1": "val1",
				"key2": "val2",
			}),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "resultName",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Params: v1.Params{{
					Name:  "bParam",
					Value: *v1.NewStructuredValues(`$(tasks.aTask.results.resultName[*])`),
				}},
			},
		}},
		want: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Params: v1.Params{{
					Name: "bParam",
					// index validation is done in ResolveResultRefs() before ApplyTaskResults()
					Value: *v1.NewObject(map[string]string{
						"key1": "val1",
						"key2": "val2",
					}),
				}},
			},
		}},
	}, {
		name: "Test object result element substitution - params",
		resolvedResultRefs: resources.ResolvedResultRefs{{
			Value: *v1.NewObject(map[string]string{
				"key1": "val1",
				"key2": "val2",
			}),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "resultName",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Params: v1.Params{{
					Name:  "bParam",
					Value: *v1.NewStructuredValues(`$(tasks.aTask.results.resultName.key1)`),
				}},
			},
		}},
		want: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Params: v1.Params{{
					Name: "bParam",
					// index validation is done in ResolveResultRefs() before ApplyTaskResults()
					Value: *v1.NewStructuredValues("val1"),
				}},
			},
		}},
	}, {
		name: "Test result substitution on minimal variable substitution expression - matrix",
		resolvedResultRefs: resources.ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("aResultValue"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "a.Result",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Matrix: &v1.Matrix{
					Params: v1.Params{{
						Name:  "bParam",
						Value: *v1.NewStructuredValues(`$(tasks.aTask.results["a.Result"])`),
					}}},
			},
		}},
		want: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Matrix: &v1.Matrix{
					Params: v1.Params{{
						Name:  "bParam",
						Value: *v1.NewStructuredValues("aResultValue"),
					}}},
			},
		}},
	}, {
		name: "Test array indexing result substitution on minimal variable substitution expression - matrix",
		resolvedResultRefs: resources.ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("arrayResultValueOne", "arrayResultValueTwo"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "a.Result",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Matrix: &v1.Matrix{
					Params: v1.Params{{
						Name:  "bParam",
						Value: *v1.NewStructuredValues(`$(tasks.aTask.results["a.Result"][1])`),
					}}},
			},
		}},
		want: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Matrix: &v1.Matrix{
					Params: v1.Params{{
						Name:  "bParam",
						Value: *v1.NewStructuredValues("arrayResultValueTwo"),
					}}},
			},
		}},
	}, {
		name: "Test array indexing result substitution out of bound - matrix",
		resolvedResultRefs: resources.ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("arrayResultValueOne", "arrayResultValueTwo"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "a.Result",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Matrix: &v1.Matrix{
					Params: v1.Params{{
						Name:  "bParam",
						Value: *v1.NewStructuredValues(`$(tasks.aTask.results["a.Result"][3])`),
					}}},
			},
		}},
		want: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Matrix: &v1.Matrix{
					Params: v1.Params{{
						Name:  "bParam",
						Value: *v1.NewStructuredValues(`$(tasks.aTask.results["a.Result"][3])`),
					}}},
			},
		}},
	}, {
		name: "Test array result substitution on minimal variable substitution expression - when expressions",
		resolvedResultRefs: resources.ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("arrayResultValueOne", "arrayResultValueTwo"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				When: []v1.WhenExpression{{
					// Note that Input doesn't support array replacement.
					Input:    "anInput",
					Operator: selection.In,
					Values:   []string{"$(tasks.aTask.results.aResult[*])"},
				}},
			},
		}},
		want: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				When: []v1.WhenExpression{{
					Input:    "anInput",
					Operator: selection.In,
					Values:   []string{"arrayResultValueOne", "arrayResultValueTwo"},
				}},
			},
		}},
	}, {
		name: "Test result substitution on minimal variable substitution expression - when expressions",
		resolvedResultRefs: resources.ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("aResultValue"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				When: []v1.WhenExpression{{
					Input:    "$(tasks.aTask.results.aResult)",
					Operator: selection.In,
					Values:   []string{"$(tasks.aTask.results.aResult)"},
				}},
			},
		}},
		want: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				When: []v1.WhenExpression{{
					Input:    "aResultValue",
					Operator: selection.In,
					Values:   []string{"aResultValue"},
				}},
			},
		}},
	}, {
		name: "Test array indexing result substitution on minimal variable substitution expression - when expressions",
		resolvedResultRefs: resources.ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("arrayResultValueOne", "arrayResultValueTwo"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				When: []v1.WhenExpression{{
					Input:    "$(tasks.aTask.results.aResult[1])",
					Operator: selection.In,
					Values:   []string{"$(tasks.aTask.results.aResult[0])"},
				}},
			},
		}},
		want: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				When: []v1.WhenExpression{{
					Input:    "arrayResultValueTwo",
					Operator: selection.In,
					Values:   []string{"arrayResultValueOne"},
				}},
			},
		}},
	}, {
		name: "Test array result substitution on minimal variable substitution expression - resolver params",
		resolvedResultRefs: resources.ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("arrayResultValueOne", "arrayResultValueTwo"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				TaskRef: &v1.TaskRef{
					ResolverRef: v1.ResolverRef{
						Params: v1.Params{{
							Name: "bParam",
							Value: v1.ParamValue{Type: v1.ParamTypeArray,
								ArrayVal: []string{`$(tasks.aTask.results["aResult"][*])`},
							},
						}},
					},
				},
			},
		}},
		want: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				TaskRef: &v1.TaskRef{
					ResolverRef: v1.ResolverRef{
						Params: v1.Params{{
							Name: "bParam",
							Value: v1.ParamValue{Type: v1.ParamTypeArray,
								ArrayVal: []string{"arrayResultValueOne", "arrayResultValueTwo"},
							},
						}},
					},
				},
			},
		}},
	}, {
		name: "Test result substitution on minimal variable substitution expression - resolver params",
		resolvedResultRefs: resources.ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("aResultValue"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				TaskRef: &v1.TaskRef{
					ResolverRef: v1.ResolverRef{
						Params: v1.Params{{
							Name:  "bParam",
							Value: *v1.NewStructuredValues("$(tasks.aTask.results.aResult)"),
						}},
					},
				},
			},
		}},
		want: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				TaskRef: &v1.TaskRef{
					ResolverRef: v1.ResolverRef{
						Params: v1.Params{{
							Name:  "bParam",
							Value: *v1.NewStructuredValues("aResultValue"),
						}},
					},
				},
			},
		}},
	}, {
		name: "Test array indexing result substitution on minimal variable substitution expression - resolver params",
		resolvedResultRefs: resources.ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("arrayResultValueOne", "arrayResultValueTwo"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				TaskRef: &v1.TaskRef{
					ResolverRef: v1.ResolverRef{
						Params: v1.Params{{
							Name:  "bParam",
							Value: *v1.NewStructuredValues("$(tasks.aTask.results.aResult[0])"),
						}, {
							Name:  "cParam",
							Value: *v1.NewStructuredValues("$(tasks.aTask.results.aResult[1])"),
						}},
					},
				},
			},
		}},
		want: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				TaskRef: &v1.TaskRef{
					ResolverRef: v1.ResolverRef{
						Params: v1.Params{{
							Name:  "bParam",
							Value: *v1.NewStructuredValues("arrayResultValueOne"),
						}, {
							Name:  "cParam",
							Value: *v1.NewStructuredValues("arrayResultValueTwo"),
						}},
					},
				},
			},
		}},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			resources.ApplyTaskResults(tt.targets, tt.resolvedResultRefs)
			if d := cmp.Diff(tt.want, tt.targets); d != "" {
				t.Fatalf("ApplyTaskResults() %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestApplyTaskResults_EmbeddedExpression(t *testing.T) {
	for _, tt := range []struct {
		name               string
		targets            resources.PipelineRunState
		resolvedResultRefs resources.ResolvedResultRefs
		want               resources.PipelineRunState
	}{{
		name: "Test result substitution on embedded variable substitution expression - params",
		resolvedResultRefs: resources.ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("aResultValue"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Params: v1.Params{{
					Name:  "bParam",
					Value: *v1.NewStructuredValues("Result value --> $(tasks.aTask.results.aResult)"),
				}},
			},
		}},
		want: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Params: v1.Params{{
					Name:  "bParam",
					Value: *v1.NewStructuredValues("Result value --> aResultValue"),
				}},
			},
		}},
	}, {
		name: "Test array indexing result substitution on embedded variable substitution expression - params",
		resolvedResultRefs: resources.ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("arrayResultValueOne", "arrayResultValueTwo"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Params: v1.Params{{
					Name:  "bParam",
					Value: *v1.NewStructuredValues("Result value --> $(tasks.aTask.results.aResult[0])"),
				}},
			},
		}},
		want: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Params: v1.Params{{
					Name:  "bParam",
					Value: *v1.NewStructuredValues("Result value --> arrayResultValueOne"),
				}},
			},
		}},
	}, {
		name: "Test result substitution on embedded variable substitution expression - matrix",
		resolvedResultRefs: resources.ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("aResultValue"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Matrix: &v1.Matrix{
					Params: v1.Params{{
						Name:  "bParam",
						Value: *v1.NewStructuredValues("Result value --> $(tasks.aTask.results.aResult)"),
					}}},
			},
		}},
		want: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Matrix: &v1.Matrix{
					Params: v1.Params{{
						Name:  "bParam",
						Value: *v1.NewStructuredValues("Result value --> aResultValue"),
					}}},
			},
		}},
	}, {
		name: "test result substitution for strings and arrays in matrix params",
		resolvedResultRefs: resources.ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("foo"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "foo",
			},
			FromTaskRun: "aTaskRun",
		}, {
			Value: *v1.NewStructuredValues("b", "a", "r"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "bar",
			},
			FromTaskRun: "aTaskRun",
		}, {
			Value: *v1.NewObject(map[string]string{
				"key1": "r",
				"key2": "a",
				"key3": "d",
			}),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "rad",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Matrix: &v1.Matrix{
					Params: v1.Params{{
						// string replacements from string param, array param and object results
						Name: "first-param", Value: *v1.NewStructuredValues("$(tasks.aTask.results.foo)", "$(tasks.aTask.results.bar[0])", "$(tasks.aTask.results.rad.key1)"),
					}},
				},
			},
		}},
		want: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Matrix: &v1.Matrix{
					Params: v1.Params{{
						// string replacements from string param, array param and object results
						Name: "first-param", Value: *v1.NewStructuredValues("foo", "b", "r"),
					}},
				},
			},
		}},
	}, {
		name: "test result substitution for strings from string, arr, obj results in matrix include params",
		resolvedResultRefs: resources.ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("foo"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "foo",
			},
			FromTaskRun: "aTaskRun",
		}, {
			Value: *v1.NewStructuredValues("b", "a", "r"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "bar",
			},
			FromTaskRun: "aTaskRun",
		}, {
			Value: *v1.NewObject(map[string]string{
				"key1": "r",
				"key2": "a",
				"key3": "d",
			}),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "rad",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Matrix: &v1.Matrix{
					Include: []v1.IncludeParams{{
						Name: "build-1",
						Params: v1.Params{{
							// string replacements from string results, array results and object results
							Name: "first-param", Value: *v1.NewStructuredValues("foo", "b", "r"),
						}},
					}},
				},
			},
		}},
		want: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				Matrix: &v1.Matrix{
					Include: []v1.IncludeParams{{
						Name: "build-1",
						Params: v1.Params{{
							// string replacements from string results, array results and object results
							Name: "first-param", Value: *v1.NewStructuredValues("foo", "b", "r"),
						}},
					}},
				},
			},
		}},
	}, {
		name: "Test result substitution on embedded variable substitution expression - when expressions",
		resolvedResultRefs: resources.ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("aResultValue"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				When: []v1.WhenExpression{
					{
						Input:    "Result value --> $(tasks.aTask.results.aResult)",
						Operator: selection.In,
						Values:   []string{"Result value --> $(tasks.aTask.results.aResult)"},
					},
				},
			},
		}},
		want: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				When: []v1.WhenExpression{{
					Input:    "Result value --> aResultValue",
					Operator: selection.In,
					Values:   []string{"Result value --> aResultValue"},
				}},
			},
		}},
	}, {
		name: "Test array indexing result substitution on embedded variable substitution expression - when expressions",
		resolvedResultRefs: resources.ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("arrayResultValueOne", "arrayResultValueTwo"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				When: []v1.WhenExpression{
					{
						Input:    "Result value --> $(tasks.aTask.results.aResult[1])",
						Operator: selection.In,
						Values:   []string{"Result value --> $(tasks.aTask.results.aResult[0])"},
					},
				},
			},
		}},
		want: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				Name:    "bTask",
				TaskRef: &v1.TaskRef{Name: "bTask"},
				When: []v1.WhenExpression{{
					Input:    "Result value --> arrayResultValueTwo",
					Operator: selection.In,
					Values:   []string{"Result value --> arrayResultValueOne"},
				}},
			},
		}},
	}, {
		name: "Test result substitution on embedded variable substitution expression - resolver params",
		resolvedResultRefs: resources.ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("aResultValue"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				TaskRef: &v1.TaskRef{
					ResolverRef: v1.ResolverRef{
						Params: v1.Params{{
							Name:  "bParam",
							Value: *v1.NewStructuredValues("Result value --> $(tasks.aTask.results.aResult)"),
						}},
					},
				},
			},
		}},
		want: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				TaskRef: &v1.TaskRef{
					ResolverRef: v1.ResolverRef{
						Params: v1.Params{{
							Name:  "bParam",
							Value: *v1.NewStructuredValues("Result value --> aResultValue"),
						}},
					},
				},
			},
		}},
	}, {
		name: "Test array indexing result substitution on embedded variable substitution expression - resolver params",
		resolvedResultRefs: resources.ResolvedResultRefs{{
			Value: *v1.NewStructuredValues("arrayResultValueOne", "arrayResultValueTwo"),
			ResultReference: v1.ResultRef{
				PipelineTask: "aTask",
				Result:       "aResult",
			},
			FromTaskRun: "aTaskRun",
		}},
		targets: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				TaskRef: &v1.TaskRef{
					ResolverRef: v1.ResolverRef{
						Params: v1.Params{{
							Name:  "bParam",
							Value: *v1.NewStructuredValues("Result value --> $(tasks.aTask.results.aResult[0])"),
						}, {
							Name:  "cParam",
							Value: *v1.NewStructuredValues("Result value --> $(tasks.aTask.results.aResult[1])"),
						}},
					},
				},
			},
		}},
		want: resources.PipelineRunState{{
			PipelineTask: &v1.PipelineTask{
				TaskRef: &v1.TaskRef{
					ResolverRef: v1.ResolverRef{
						Params: v1.Params{{
							Name:  "bParam",
							Value: *v1.NewStructuredValues("Result value --> arrayResultValueOne"),
						}, {
							Name:  "cParam",
							Value: *v1.NewStructuredValues("Result value --> arrayResultValueTwo"),
						}},
					},
				},
			},
		}},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			resources.ApplyTaskResults(tt.targets, tt.resolvedResultRefs)
			if d := cmp.Diff(tt.want, tt.targets); d != "" {
				t.Fatalf("ApplyTaskResults() %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestContext(t *testing.T) {
	for _, tc := range []struct {
		description string
		pr          *v1.PipelineRun
		original    v1.Param
		expected    v1.Param
	}{{
		description: "context.pipeline.name defined",
		pr: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "name"},
		},
		original: v1.Param{Value: *v1.NewStructuredValues("$(context.pipeline.name)-1")},
		expected: v1.Param{Value: *v1.NewStructuredValues("test-pipeline-1")},
	}, {
		description: "context.pipelineRun.name defined",
		pr: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "name"},
		},
		original: v1.Param{Value: *v1.NewStructuredValues("$(context.pipelineRun.name)-1")},
		expected: v1.Param{Value: *v1.NewStructuredValues("name-1")},
	}, {
		description: "context.pipelineRun.name undefined",
		pr: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: ""},
		},
		original: v1.Param{Value: *v1.NewStructuredValues("$(context.pipelineRun.name)-1")},
		expected: v1.Param{Value: *v1.NewStructuredValues("-1")},
	}, {
		description: "context.pipelineRun.namespace defined",
		pr: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Namespace: "namespace"},
		},
		original: v1.Param{Value: *v1.NewStructuredValues("$(context.pipelineRun.namespace)-1")},
		expected: v1.Param{Value: *v1.NewStructuredValues("namespace-1")},
	}, {
		description: "context.pipelineRun.namespace undefined",
		pr: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Namespace: ""},
		},
		original: v1.Param{Value: *v1.NewStructuredValues("$(context.pipelineRun.namespace)-1")},
		expected: v1.Param{Value: *v1.NewStructuredValues("-1")},
	}, {
		description: "context.pipelineRun.uid defined",
		pr: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{UID: "UID"},
		},
		original: v1.Param{Value: *v1.NewStructuredValues("$(context.pipelineRun.uid)-1")},
		expected: v1.Param{Value: *v1.NewStructuredValues("UID-1")},
	}, {
		description: "context.pipelineRun.uid undefined",
		pr: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{UID: ""},
		},
		original: v1.Param{Value: *v1.NewStructuredValues("$(context.pipelineRun.uid)-1")},
		expected: v1.Param{Value: *v1.NewStructuredValues("-1")},
	}} {
		t.Run(tc.description, func(t *testing.T) {
			orig := &v1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline"},
				Spec: v1.PipelineSpec{
					Tasks: []v1.PipelineTask{{
						Params: v1.Params{tc.original},
						Matrix: &v1.Matrix{
							Params: v1.Params{tc.original},
						}}},
				},
			}
			got := resources.ApplyContexts(&orig.Spec, orig.Name, tc.pr)
			if d := cmp.Diff(tc.expected, got.Tasks[0].Params[0]); d != "" {
				t.Errorf(diff.PrintWantGot(d))
			}
			if d := cmp.Diff(tc.expected, got.Tasks[0].Matrix.Params[0]); d != "" {
				t.Errorf(diff.PrintWantGot(d))
			}
		})
	}
}

func TestApplyPipelineTaskContexts(t *testing.T) {
	for _, tc := range []struct {
		description string
		pt          v1.PipelineTask
		want        v1.PipelineTask
	}{{
		description: "context retries replacement",
		pt: v1.PipelineTask{
			Retries: 5,
			Params: v1.Params{{
				Name:  "retries",
				Value: *v1.NewStructuredValues("$(context.pipelineTask.retries)"),
			}},
			Matrix: &v1.Matrix{
				Params: v1.Params{{
					Name:  "retries",
					Value: *v1.NewStructuredValues("$(context.pipelineTask.retries)"),
				}},
				Include: []v1.IncludeParams{{
					Name: "build-1",
					Params: v1.Params{{
						Name:  "retries",
						Value: *v1.NewStructuredValues("$(context.pipelineTask.retries)"),
					}},
				}},
			},
		},
		want: v1.PipelineTask{
			Retries: 5,
			Params: v1.Params{{
				Name:  "retries",
				Value: *v1.NewStructuredValues("5"),
			}},
			Matrix: &v1.Matrix{
				Params: v1.Params{{
					Name:  "retries",
					Value: *v1.NewStructuredValues("5"),
				}},
				Include: []v1.IncludeParams{{
					Name: "build-1",
					Params: v1.Params{{
						Name:  "retries",
						Value: *v1.NewStructuredValues("5"),
					}},
				}},
			},
		},
	}, {
		description: "context retries replacement with no defined retries",
		pt: v1.PipelineTask{
			Params: v1.Params{{
				Name:  "retries",
				Value: *v1.NewStructuredValues("$(context.pipelineTask.retries)"),
			}},
			Matrix: &v1.Matrix{
				Params: v1.Params{{
					Name:  "retries",
					Value: *v1.NewStructuredValues("$(context.pipelineTask.retries)"),
				}},
				Include: []v1.IncludeParams{{
					Name: "build-1",
					Params: v1.Params{{
						Name:  "retries",
						Value: *v1.NewStructuredValues("$(context.pipelineTask.retries)"),
					}},
				}},
			},
		},
		want: v1.PipelineTask{
			Params: v1.Params{{
				Name:  "retries",
				Value: *v1.NewStructuredValues("0"),
			}},
			Matrix: &v1.Matrix{
				Params: v1.Params{{
					Name:  "retries",
					Value: *v1.NewStructuredValues("0"),
				}},
				Include: []v1.IncludeParams{{
					Name: "build-1",
					Params: v1.Params{{
						Name:  "retries",
						Value: *v1.NewStructuredValues("0"),
					}},
				}},
			},
		},
	}} {
		t.Run(tc.description, func(t *testing.T) {
			got := resources.ApplyPipelineTaskContexts(&tc.pt)
			if d := cmp.Diff(&tc.want, got); d != "" {
				t.Errorf(diff.PrintWantGot(d))
			}
		})
	}
}

func TestApplyWorkspaces(t *testing.T) {
	for _, tc := range []struct {
		description         string
		declarations        []v1.PipelineWorkspaceDeclaration
		bindings            []v1.WorkspaceBinding
		variableUsage       string
		expectedReplacement string
	}{{
		description: "workspace declared and bound",
		declarations: []v1.PipelineWorkspaceDeclaration{{
			Name: "foo",
		}},
		bindings: []v1.WorkspaceBinding{{
			Name: "foo",
		}},
		variableUsage:       "$(workspaces.foo.bound)",
		expectedReplacement: "true",
	}, {
		description: "workspace declared not bound",
		declarations: []v1.PipelineWorkspaceDeclaration{{
			Name:     "foo",
			Optional: true,
		}},
		bindings:            []v1.WorkspaceBinding{},
		variableUsage:       "$(workspaces.foo.bound)",
		expectedReplacement: "false",
	}} {
		t.Run(tc.description, func(t *testing.T) {
			p1 := v1.PipelineSpec{
				Tasks: []v1.PipelineTask{{
					Params: v1.Params{{Value: *v1.NewStructuredValues(tc.variableUsage)}},
				}},
				Workspaces: tc.declarations,
			}
			pr := &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					PipelineRef: &v1.PipelineRef{
						Name: "test-pipeline",
					},
					Workspaces: tc.bindings,
				},
			}
			p2 := resources.ApplyWorkspaces(&p1, pr)
			str := p2.Tasks[0].Params[0].Value.StringVal
			if str != tc.expectedReplacement {
				t.Errorf("expected %q, received %q", tc.expectedReplacement, str)
			}
		})
	}
}

func TestApplyFinallyResultsToPipelineResults(t *testing.T) {
	for _, tc := range []struct {
		description   string
		results       []v1.PipelineResult
		taskResults   map[string][]v1.TaskRunResult
		runResults    map[string][]v1beta1.CustomRunResult
		skippedTasks  []v1.SkippedTask
		expected      []v1.PipelineRunResult
		expectedError error
	}{{
		description: "single-string-result-single-successful-task",
		results: []v1.PipelineResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("$(finally.pt1.results.foo)"),
		}},
		taskResults: map[string][]v1.TaskRunResult{
			"pt1": {
				{
					Name:  "foo",
					Value: *v1.NewStructuredValues("do"),
				},
			},
		},
		expected: []v1.PipelineRunResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("do"),
		}},
	}, {
		description: "single-array-result-single-successful-task",
		results: []v1.PipelineResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("$(finally.pt1.results.foo[*])"),
		}},
		taskResults: map[string][]v1.TaskRunResult{
			"pt1": {
				{
					Name:  "foo",
					Value: *v1.NewStructuredValues("do", "rae"),
				},
			},
		},
		expected: []v1.PipelineRunResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("do", "rae"),
		}},
	}, {
		description: "multiple-results-custom-and-normal-tasks",
		results: []v1.PipelineResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("$(finally.customtask.results.foo)"),
		}},
		runResults: map[string][]v1beta1.CustomRunResult{
			"customtask": {
				{
					Name:  "foo",
					Value: "do",
				},
			},
		},
		expected: []v1.PipelineRunResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("do"),
		}},
	}, {
		description: "apply-object-results",
		results: []v1.PipelineResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("$(finally.pt1.results.foo[*])"),
		}},
		taskResults: map[string][]v1.TaskRunResult{
			"pt1": {
				{
					Name: "foo",
					Value: *v1.NewObject(map[string]string{
						"key1": "val1",
						"key2": "val2",
					}),
				},
			},
		},
		expected: []v1.PipelineRunResult{{
			Name: "pipeline-result-1",
			Value: *v1.NewObject(map[string]string{
				"key1": "val1",
				"key2": "val2",
			}),
		}},
	},
		{
			description: "referencing-invalid-finally-task",
			results: []v1.PipelineResult{{
				Name:  "pipeline-result-1",
				Value: *v1.NewStructuredValues("$(finally.pt2.results.foo)"),
			}},
			taskResults: map[string][]v1.TaskRunResult{
				"pt1": {
					{
						Name:  "foo",
						Value: *v1.NewStructuredValues("do"),
					},
				},
			},
			expected:      nil,
			expectedError: fmt.Errorf("invalid pipelineresults [pipeline-result-1], the referred result don't exist"),
		},
	} {
		t.Run(tc.description, func(t *testing.T) {
			received, _ := resources.ApplyTaskResultsToPipelineResults(context.Background(), tc.results, tc.taskResults, tc.runResults, nil /* skippedTasks */)
			if d := cmp.Diff(tc.expected, received); d != "" {
				t.Errorf(diff.PrintWantGot(d))
			}
		})
	}
}

func TestApplyTaskResultsToPipelineResults_Success(t *testing.T) {
	for _, tc := range []struct {
		description     string
		results         []v1.PipelineResult
		taskResults     map[string][]v1.TaskRunResult
		runResults      map[string][]v1beta1.CustomRunResult
		taskstatus      map[string]string
		skippedTasks    []v1.SkippedTask
		expectedResults []v1.PipelineRunResult
	}{{
		description: "non-reference-results",
		results: []v1.PipelineResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("resultName"),
		}},
		taskResults: map[string][]v1.TaskRunResult{
			"pt1": {
				{
					Name:  "foo",
					Value: *v1.NewStructuredValues("do", "rae", "mi"),
				},
			},
		},
		expectedResults: nil,
	}, {
		description: "apply-array-results",
		results: []v1.PipelineResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("$(tasks.pt1.results.foo[*])"),
		}},
		taskResults: map[string][]v1.TaskRunResult{
			"pt1": {
				{
					Name:  "foo",
					Value: *v1.NewStructuredValues("do", "rae", "mi"),
				},
			},
		},
		expectedResults: []v1.PipelineRunResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("do", "rae", "mi"),
		}},
	}, {
		description: "apply-array-indexing-results",
		results: []v1.PipelineResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("$(tasks.pt1.results.foo[1])"),
		}},
		taskResults: map[string][]v1.TaskRunResult{
			"pt1": {
				{
					Name:  "foo",
					Value: *v1.NewStructuredValues("do", "rae", "mi"),
				},
			},
		},
		expectedResults: []v1.PipelineRunResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("rae"),
		}},
	}, {
		description: "apply-object-results",
		results: []v1.PipelineResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("$(tasks.pt1.results.foo[*])"),
		}},
		taskResults: map[string][]v1.TaskRunResult{
			"pt1": {
				{
					Name: "foo",
					Value: *v1.NewObject(map[string]string{
						"key1": "val1",
						"key2": "val2",
					}),
				},
			},
		},
		expectedResults: []v1.PipelineRunResult{{
			Name: "pipeline-result-1",
			Value: *v1.NewObject(map[string]string{
				"key1": "val1",
				"key2": "val2",
			}),
		}},
	}, {
		description: "object-results-from-array-indexing-and-object-element",
		results: []v1.PipelineResult{{
			Name: "pipeline-result-1",
			Value: *v1.NewObject(map[string]string{
				"pkey1": "$(tasks.pt1.results.foo.key1)",
				"pkey2": "$(tasks.pt2.results.bar[1])",
			}),
		}},
		taskResults: map[string][]v1.TaskRunResult{
			"pt1": {
				{
					Name: "foo",
					Value: *v1.NewObject(map[string]string{
						"key1": "val1",
						"key2": "val2",
					}),
				},
			},
			"pt2": {
				{
					Name:  "bar",
					Value: *v1.NewStructuredValues("do", "rae", "mi"),
				},
			},
		},
		expectedResults: []v1.PipelineRunResult{{
			Name: "pipeline-result-1",
			Value: *v1.NewObject(map[string]string{
				"pkey1": "val1",
				"pkey2": "rae",
			}),
		}},
	}, {
		description: "array-results-from-array-indexing-and-object-element",
		results: []v1.PipelineResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("$(tasks.pt1.results.foo.key1)", "$(tasks.pt2.results.bar[1])"),
		}},
		taskResults: map[string][]v1.TaskRunResult{
			"pt1": {
				{
					Name: "foo",
					Value: *v1.NewObject(map[string]string{
						"key1": "val1",
						"key2": "val2",
					}),
				},
			},
			"pt2": {
				{
					Name:  "bar",
					Value: *v1.NewStructuredValues("do", "rae", "mi"),
				},
			},
		},
		expectedResults: []v1.PipelineRunResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("val1", "rae"),
		}},
	}, {
		description: "apply-object-element",
		results: []v1.PipelineResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("$(tasks.pt1.results.foo.key1)"),
		}},
		taskResults: map[string][]v1.TaskRunResult{
			"pt1": {
				{
					Name: "foo",
					Value: *v1.NewObject(map[string]string{
						"key1": "val1",
						"key2": "val2",
					}),
				},
			},
		},
		expectedResults: []v1.PipelineRunResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("val1"),
		}},
	}, {
		description: "multiple-array-results-multiple-successful-tasks ",
		results: []v1.PipelineResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("$(tasks.pt1.results.foo[*])"),
		}, {
			Name:  "pipeline-result-2",
			Value: *v1.NewStructuredValues("$(tasks.pt2.results.bar[*])"),
		}},
		taskResults: map[string][]v1.TaskRunResult{
			"pt1": {
				{
					Name:  "foo",
					Value: *v1.NewStructuredValues("do", "rae"),
				},
			},
			"pt2": {
				{
					Name:  "bar",
					Value: *v1.NewStructuredValues("do", "rae", "mi"),
				},
			},
		},
		expectedResults: []v1.PipelineRunResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("do", "rae"),
		}, {
			Name:  "pipeline-result-2",
			Value: *v1.NewStructuredValues("do", "rae", "mi"),
		}},
	}, {
		description: "no-pipeline-results-no-returned-results",
		results:     []v1.PipelineResult{},
		taskResults: map[string][]v1.TaskRunResult{
			"pt1": {{
				Name:  "foo",
				Value: *v1.NewStructuredValues("bar"),
			}},
		},
		expectedResults: nil,
	}, {
		description: "multiple-results-multiple-successful-tasks ",
		results: []v1.PipelineResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("$(tasks.pt1.results.foo)"),
		}, {
			Name:  "pipeline-result-2",
			Value: *v1.NewStructuredValues("$(tasks.pt1.results.foo), $(tasks.pt2.results.baz), $(tasks.pt1.results.bar), $(tasks.pt2.results.baz), $(tasks.pt1.results.foo)"),
		}},
		taskResults: map[string][]v1.TaskRunResult{
			"pt1": {
				{
					Name:  "foo",
					Value: *v1.NewStructuredValues("do"),
				}, {
					Name:  "bar",
					Value: *v1.NewStructuredValues("mi"),
				},
			},
			"pt2": {{
				Name:  "baz",
				Value: *v1.NewStructuredValues("rae"),
			}},
		},
		expectedResults: []v1.PipelineRunResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("do"),
		}, {
			Name:  "pipeline-result-2",
			Value: *v1.NewStructuredValues("do, rae, mi, rae, do"),
		}},
	}, {
		description: "multiple-results-custom-and-normal-tasks",
		results: []v1.PipelineResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("$(tasks.customtask.results.foo)"),
		}, {
			Name:  "pipeline-result-2",
			Value: *v1.NewStructuredValues("$(tasks.customtask.results.foo), $(tasks.normaltask.results.baz), $(tasks.customtask.results.bar), $(tasks.normaltask.results.baz), $(tasks.customtask.results.foo)"),
		}},
		runResults: map[string][]v1beta1.CustomRunResult{
			"customtask": {
				{
					Name:  "foo",
					Value: "do",
				}, {
					Name:  "bar",
					Value: "mi",
				},
			},
		},
		taskResults: map[string][]v1.TaskRunResult{
			"normaltask": {{
				Name:  "baz",
				Value: *v1.NewStructuredValues("rae"),
			}},
		},
		expectedResults: []v1.PipelineRunResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("do"),
		}, {
			Name:  "pipeline-result-2",
			Value: *v1.NewStructuredValues("do, rae, mi, rae, do"),
		}},
	}, {
		description: "multiple-results-skipped-and-normal-tasks",
		results: []v1.PipelineResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("$(tasks.skippedTask.results.foo)"),
		}, {
			Name:  "pipeline-result-2",
			Value: *v1.NewStructuredValues("$(tasks.skippedTask.results.foo), $(tasks.normaltask.results.baz)"),
		}},
		taskResults: map[string][]v1.TaskRunResult{
			"normaltask": {{
				Name:  "baz",
				Value: *v1.NewStructuredValues("rae"),
			}},
		},
		taskstatus:      map[string]string{resources.PipelineTaskStatusPrefix + "skippedTask" + resources.PipelineTaskStatusSuffix: resources.PipelineTaskStateNone},
		expectedResults: nil,
	}, {
		description: "unsuccessful-taskrun-no-results",
		results: []v1.PipelineResult{{
			Name:  "foo",
			Value: *v1.NewStructuredValues("$(tasks.pt1.results.foo)"),
		}},
		taskResults:     map[string][]v1.TaskRunResult{},
		taskstatus:      map[string]string{resources.PipelineTaskStatusPrefix + "pt1" + resources.PipelineTaskStatusSuffix: v1beta1.TaskRunReasonFailed.String()},
		expectedResults: nil,
	}, {
		description: "unsuccessful-taskrun-no-returned-result-object-ref",
		results: []v1.PipelineResult{{
			Name:  "foo",
			Value: *v1.NewStructuredValues("$(tasks.pt1.results.foo.key1)"),
		}},
		taskResults:     map[string][]v1.TaskRunResult{},
		taskstatus:      map[string]string{resources.PipelineTaskStatusPrefix + "pt1" + resources.PipelineTaskStatusSuffix: v1beta1.TaskRunReasonFailed.String()},
		expectedResults: nil,
	}, {
		description: "unsuccessful-taskrun-with-results",
		results: []v1.PipelineResult{{
			Name:  "foo",
			Value: *v1.NewStructuredValues("$(tasks.pt1.results.foo[*])"),
		}},
		taskResults: map[string][]v1.TaskRunResult{
			"pt1": {
				{
					Name:  "foo",
					Value: *v1.NewStructuredValues("do", "rae", "mi"),
				},
			}},
		taskstatus: map[string]string{resources.PipelineTaskStatusPrefix + "pt1" + resources.PipelineTaskStatusSuffix: v1beta1.TaskRunReasonFailed.String()},
		expectedResults: []v1.PipelineRunResult{{
			Name:  "foo",
			Value: *v1.NewStructuredValues("do", "rae", "mi"),
		}},
	}} {
		t.Run(tc.description, func(t *testing.T) {
			received, err := resources.ApplyTaskResultsToPipelineResults(context.Background(), tc.results, tc.taskResults, tc.runResults, tc.taskstatus)
			if err != nil {
				t.Errorf("Got unecpected error:%v", err)
			}
			if d := cmp.Diff(tc.expectedResults, received); d != "" {
				t.Errorf(diff.PrintWantGot(d))
			}
		})
	}
}

func TestApplyTaskResultsToPipelineResults_Error(t *testing.T) {
	for _, tc := range []struct {
		description     string
		results         []v1.PipelineResult
		taskResults     map[string][]v1.TaskRunResult
		runResults      map[string][]v1beta1.CustomRunResult
		expectedResults []v1.PipelineRunResult
		expectedError   error
	}{{
		description: "array-index-out-of-bound",
		results: []v1.PipelineResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("$(tasks.pt1.results.foo[4])"),
		}},
		taskResults: map[string][]v1.TaskRunResult{
			"pt1": {
				{
					Name:  "foo",
					Value: *v1.NewStructuredValues("do", "rae", "mi"),
				},
			},
		},
		expectedResults: nil,
		expectedError:   fmt.Errorf("invalid pipelineresults [pipeline-result-1], the referred results don't exist"),
	}, {
		description: "object-reference-key-not-exist",
		results: []v1.PipelineResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("$(tasks.pt1.results.foo.key3)"),
		}},
		taskResults: map[string][]v1.TaskRunResult{
			"pt1": {
				{
					Name: "foo",
					Value: *v1.NewObject(map[string]string{
						"key1": "val1",
						"key2": "val2",
					}),
				},
			},
		},
		expectedResults: nil,
		expectedError:   fmt.Errorf("invalid pipelineresults [pipeline-result-1], the referred results don't exist"),
	}, {
		description: "object-results-resultname-not-exist",
		results: []v1.PipelineResult{{
			Name:  "pipeline-result-1",
			Value: *v1.NewStructuredValues("$(tasks.pt1.results.bar.key1)"),
		}},
		taskResults: map[string][]v1.TaskRunResult{
			"pt1": {
				{
					Name: "foo",
					Value: *v1.NewObject(map[string]string{
						"key1": "val1",
						"key2": "val2",
					}),
				},
			},
		},
		expectedResults: nil,
		expectedError:   fmt.Errorf("invalid pipelineresults [pipeline-result-1], the referred results don't exist"),
	}, {
		description: "invalid-result-variable-no-returned-result",
		results: []v1.PipelineResult{{
			Name:  "foo",
			Value: *v1.NewStructuredValues("$(tasks.pt1_results.foo)"),
		}},
		taskResults: map[string][]v1.TaskRunResult{
			"pt1": {{
				Name:  "foo",
				Value: *v1.NewStructuredValues("bar"),
			}},
		},
		expectedResults: nil,
		expectedError:   fmt.Errorf("invalid pipelineresults [foo], the referred results don't exist"),
	}, {
		description: "no-taskrun-results-no-returned-results",
		results: []v1.PipelineResult{{
			Name:  "foo",
			Value: *v1.NewStructuredValues("$(tasks.pt1.results.foo)"),
		}},
		taskResults: map[string][]v1.TaskRunResult{
			"pt1": {},
		},
		expectedResults: nil,
		expectedError:   fmt.Errorf("invalid pipelineresults [foo], the referred results don't exist"),
	}, {
		description: "invalid-taskrun-name-no-returned-result",
		results: []v1.PipelineResult{{
			Name:  "foo",
			Value: *v1.NewStructuredValues("$(tasks.pt1.results.foo)"),
		}},
		taskResults: map[string][]v1.TaskRunResult{
			"definitely-not-pt1": {{
				Name:  "foo",
				Value: *v1.NewStructuredValues("bar"),
			}},
		},
		expectedResults: nil,
		expectedError:   fmt.Errorf("invalid pipelineresults [foo], the referred results don't exist"),
	}, {
		description: "invalid-result-name-no-returned-result",
		results: []v1.PipelineResult{{
			Name:  "foo",
			Value: *v1.NewStructuredValues("$(tasks.pt1.results.foo)"),
		}},
		taskResults: map[string][]v1.TaskRunResult{
			"pt1": {{
				Name:  "definitely-not-foo",
				Value: *v1.NewStructuredValues("bar"),
			}},
		},
		expectedResults: nil,
		expectedError:   fmt.Errorf("invalid pipelineresults [foo], the referred results don't exist"),
	}, {
		description: "unsuccessful-taskrun-no-returned-result",
		results: []v1.PipelineResult{{
			Name:  "foo",
			Value: *v1.NewStructuredValues("$(tasks.pt1.results.foo)"),
		}},
		taskResults:     map[string][]v1.TaskRunResult{},
		expectedResults: nil,
		expectedError:   fmt.Errorf("invalid pipelineresults [foo], the referred results don't exist"),
	}, {
		description: "mixed-success-tasks-some-returned-results",
		results: []v1.PipelineResult{{
			Name:  "foo",
			Value: *v1.NewStructuredValues("$(tasks.pt1.results.foo)"),
		}, {
			Name:  "bar",
			Value: *v1.NewStructuredValues("$(tasks.pt2.results.bar)"),
		}},
		taskResults: map[string][]v1.TaskRunResult{
			"pt2": {{
				Name:  "bar",
				Value: *v1.NewStructuredValues("rae"),
			}},
		},
		expectedResults: []v1.PipelineRunResult{{
			Name:  "bar",
			Value: *v1.NewStructuredValues("rae"),
		}},
		expectedError: fmt.Errorf("invalid pipelineresults [foo], the referred results don't exist"),
	}, {
		description: "no-run-results-no-returned-results",
		results: []v1.PipelineResult{{
			Name:  "foo",
			Value: *v1.NewStructuredValues("$(tasks.customtask.results.foo)"),
		}},
		runResults:      map[string][]v1beta1.CustomRunResult{},
		expectedResults: nil,
		expectedError:   fmt.Errorf("invalid pipelineresults [foo], the referred results don't exist"),
	}, {
		description: "wrong-customtask-name-no-returned-result",
		results: []v1.PipelineResult{{
			Name:  "foo",
			Value: *v1.NewStructuredValues("$(tasks.customtask.results.foo)"),
		}},
		runResults: map[string][]v1beta1.CustomRunResult{
			"differentcustomtask": {{
				Name:  "foo",
				Value: "bar",
			}},
		},
		expectedResults: nil,
		expectedError:   fmt.Errorf("invalid pipelineresults [foo], the referred results don't exist"),
	}, {
		description: "right-customtask-name-wrong-result-name-no-returned-result",
		results: []v1.PipelineResult{{
			Name:  "foo",
			Value: *v1.NewStructuredValues("$(tasks.customtask.results.foo)"),
		}},
		runResults: map[string][]v1beta1.CustomRunResult{
			"customtask": {{
				Name:  "notfoo",
				Value: "bar",
			}},
		},
		expectedResults: nil,
		expectedError:   fmt.Errorf("invalid pipelineresults [foo], the referred results don't exist"),
	}, {
		description: "unsuccessful-run-no-returned-result",
		results: []v1.PipelineResult{{
			Name:  "foo",
			Value: *v1.NewStructuredValues("$(tasks.customtask.results.foo)"),
		}},
		runResults: map[string][]v1beta1.CustomRunResult{
			"customtask": {},
		},
		expectedResults: nil,
		expectedError:   fmt.Errorf("invalid pipelineresults [foo], the referred results don't exist"),
	}, {
		description: "wrong-result-reference-expression",
		results: []v1.PipelineResult{{
			Name:  "foo",
			Value: *v1.NewStructuredValues("$(tasks.task.results.foo.foo.foo)"),
		}},
		runResults: map[string][]v1beta1.CustomRunResult{
			"customtask": {},
		},
		expectedResults: nil,
		expectedError:   fmt.Errorf("invalid pipelineresults [foo], the referred results don't exist"),
	}} {
		t.Run(tc.description, func(t *testing.T) {
			received, err := resources.ApplyTaskResultsToPipelineResults(context.Background(), tc.results, tc.taskResults, tc.runResults, nil /*skipped tasks*/)
			if err == nil {
				t.Errorf("Expect error but got nil")
				return
			}

			if d := cmp.Diff(tc.expectedError.Error(), err.Error()); d != "" {
				t.Errorf("ApplyTaskResultsToPipelineResults() errors diff %s", diff.PrintWantGot(d))
			}

			if d := cmp.Diff(tc.expectedResults, received); d != "" {
				t.Errorf(diff.PrintWantGot(d))
			}
		})
	}
}

func TestApplyTaskRunContext(t *testing.T) {
	r := map[string]string{
		"tasks.task1.status": "succeeded",
		"tasks.task3.status": "none",
	}
	state := resources.PipelineRunState{{
		PipelineTask: &v1.PipelineTask{
			Name:    "task4",
			TaskRef: &v1.TaskRef{Name: "task"},
			Params: v1.Params{{
				Name:  "task1",
				Value: *v1.NewStructuredValues("$(tasks.task1.status)"),
			}, {
				Name:  "task3",
				Value: *v1.NewStructuredValues("$(tasks.task3.status)"),
			}},
			When: v1.WhenExpressions{{
				Input:    "$(tasks.task1.status)",
				Operator: selection.In,
				Values:   []string{"$(tasks.task3.status)"},
			}},
		},
	}}
	expectedState := resources.PipelineRunState{{
		PipelineTask: &v1.PipelineTask{
			Name:    "task4",
			TaskRef: &v1.TaskRef{Name: "task"},
			Params: v1.Params{{
				Name:  "task1",
				Value: *v1.NewStructuredValues("succeeded"),
			}, {
				Name:  "task3",
				Value: *v1.NewStructuredValues("none"),
			}},
			When: v1.WhenExpressions{{
				Input:    "succeeded",
				Operator: selection.In,
				Values:   []string{"none"},
			}},
		},
	}}
	resources.ApplyPipelineTaskStateContext(state, r)
	if d := cmp.Diff(expectedState, state); d != "" {
		t.Fatalf("ApplyTaskRunContext() %s", diff.PrintWantGot(d))
	}
}
