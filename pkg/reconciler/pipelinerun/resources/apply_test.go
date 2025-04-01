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
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	cfgtesting "github.com/tektoncd/pipeline/pkg/apis/config/testing"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	taskresources "github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/selection"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestApplyParameters(t *testing.T) {
	for _, tt := range []struct {
		name     string
		original v1.PipelineSpec
		params   []v1.Param
		expected v1.PipelineSpec
		wc       func(context.Context) context.Context
	}{
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
			wc: cfgtesting.EnableAlphaAPIFields,
		},
		{
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
			wc: cfgtesting.EnableAlphaAPIFields,
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
			wc: cfgtesting.EnableAlphaAPIFields,
		},
		{
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
			wc: cfgtesting.EnableAlphaAPIFields,
		},
		{
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
			wc: cfgtesting.EnableAlphaAPIFields,
		},
		{
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
			wc: cfgtesting.EnableAlphaAPIFields,
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
			wc: cfgtesting.EnableAlphaAPIFields,
		},
		{
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
		{
			name:   "parameter/s in tasks and finally display name",
			params: v1.Params{{Name: "second-param", Value: *v1.NewStructuredValues("second-value")}},
			original: v1.PipelineSpec{
				Params: []v1.ParamSpec{
					{Name: "first-param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
					{Name: "second-param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
				},
				Tasks: []v1.PipelineTask{{
					DisplayName: "Human Readable Name $(params.first-param) $(params.second-param)",
				}},
				Finally: []v1.PipelineTask{{
					DisplayName: "Human Readable Name $(params.first-param) $(params.second-param)",
				}},
			},
			expected: v1.PipelineSpec{
				Params: []v1.ParamSpec{
					{Name: "first-param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
					{Name: "second-param", Type: v1.ParamTypeString, Default: v1.NewStructuredValues("default-value")},
				},
				Tasks: []v1.PipelineTask{{
					DisplayName: "Human Readable Name default-value second-value",
				}},
				Finally: []v1.PipelineTask{{
					DisplayName: "Human Readable Name default-value second-value",
				}},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			if tt.wc != nil {
				ctx = tt.wc(ctx)
			}
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
	}{
		{
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
	}{
		{
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
					Value: v1.ParamValue{
						Type:     v1.ParamTypeArray,
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
						Name:  "bParam",
						Value: *v1.NewStructuredValues("aResultValue"),
					}},
				},
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
						Name:  "bParam",
						Value: *v1.NewStructuredValues("arrayResultValueTwo"),
					}},
				},
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
						Name:  "bParam",
						Value: *v1.NewStructuredValues(`$(tasks.aTask.results["a.Result"][3])`),
					}},
				},
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
							Value: v1.ParamValue{
								Type:     v1.ParamTypeArray,
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
							Value: v1.ParamValue{
								Type:     v1.ParamTypeArray,
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
	}{
		{
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
		},
		{
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
		},
		{
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
							Name:  "bParam",
							Value: *v1.NewStructuredValues("Result value --> aResultValue"),
						}},
					},
				},
			}},
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
			name: "Test result substitution on embedded variable substitution expression - displayName",
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
					Name:        "bTask",
					TaskRef:     &v1.TaskRef{Name: "bTask"},
					DisplayName: "Result value --> $(tasks.aTask.results.aResult)",
				},
			}},
			want: resources.PipelineRunState{{
				PipelineTask: &v1.PipelineTask{
					Name:        "bTask",
					TaskRef:     &v1.TaskRef{Name: "bTask"},
					DisplayName: "Result value --> aResultValue",
				},
			}},
		},
		{
			name: "Test result substitution on embedded variable substitution expression - workspace.subPath",
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
					Name:       "bTask",
					TaskRef:    &v1.TaskRef{Name: "bTask"},
					Workspaces: []v1.WorkspacePipelineTaskBinding{{Name: "ws-1", Workspace: "ws-1", SubPath: "$(tasks.aTask.results.aResult)"}},
				},
			}},
			want: resources.PipelineRunState{{
				PipelineTask: &v1.PipelineTask{
					Name:       "bTask",
					TaskRef:    &v1.TaskRef{Name: "bTask"},
					Workspaces: []v1.WorkspacePipelineTaskBinding{{Name: "ws-1", Workspace: "ws-1", SubPath: "aResultValue"}},
				},
			}},
		},
	} {
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
		description         string
		pr                  *v1.PipelineRun
		original            v1.Param
		expected            v1.Param
		displayName         string
		expectedDisplayName string
	}{{
		description: "context.pipeline.name defined",
		pr: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "name"},
		},
		original:            v1.Param{Value: *v1.NewStructuredValues("$(context.pipeline.name)-1")},
		expected:            v1.Param{Value: *v1.NewStructuredValues("test-pipeline-1")},
		displayName:         "$(context.pipeline.name)-1",
		expectedDisplayName: "test-pipeline-1",
	}, {
		description: "context.pipelineRun.name defined",
		pr: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "name"},
		},
		original:            v1.Param{Value: *v1.NewStructuredValues("$(context.pipelineRun.name)-1")},
		expected:            v1.Param{Value: *v1.NewStructuredValues("name-1")},
		displayName:         "$(context.pipelineRun.name)-1",
		expectedDisplayName: "name-1",
	}, {
		description: "context.pipelineRun.name undefined",
		pr: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: ""},
		},
		original:            v1.Param{Value: *v1.NewStructuredValues("$(context.pipelineRun.name)-1")},
		expected:            v1.Param{Value: *v1.NewStructuredValues("-1")},
		displayName:         "$(context.pipelineRun.name)-1",
		expectedDisplayName: "-1",
	}, {
		description: "context.pipelineRun.namespace defined",
		pr: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Namespace: "namespace"},
		},
		original:            v1.Param{Value: *v1.NewStructuredValues("$(context.pipelineRun.namespace)-1")},
		expected:            v1.Param{Value: *v1.NewStructuredValues("namespace-1")},
		displayName:         "$(context.pipelineRun.namespace)-1",
		expectedDisplayName: "namespace-1",
	}, {
		description: "context.pipelineRun.namespace undefined",
		pr: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Namespace: ""},
		},
		original:            v1.Param{Value: *v1.NewStructuredValues("$(context.pipelineRun.namespace)-1")},
		expected:            v1.Param{Value: *v1.NewStructuredValues("-1")},
		displayName:         "$(context.pipelineRun.namespace)-1",
		expectedDisplayName: "-1",
	}, {
		description: "context.pipelineRun.uid defined",
		pr: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{UID: "UID"},
		},
		original:            v1.Param{Value: *v1.NewStructuredValues("$(context.pipelineRun.uid)-1")},
		expected:            v1.Param{Value: *v1.NewStructuredValues("UID-1")},
		displayName:         "$(context.pipelineRun.uid)-1",
		expectedDisplayName: "UID-1",
	}, {
		description: "context.pipelineRun.uid undefined",
		pr: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{UID: ""},
		},
		original:            v1.Param{Value: *v1.NewStructuredValues("$(context.pipelineRun.uid)-1")},
		expected:            v1.Param{Value: *v1.NewStructuredValues("-1")},
		displayName:         "$(context.pipelineRun.uid)-1",
		expectedDisplayName: "-1",
	}} {
		t.Run(tc.description, func(t *testing.T) {
			orig := &v1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline"},
				Spec: v1.PipelineSpec{
					Tasks: []v1.PipelineTask{
						{
							DisplayName: tc.displayName,
							Params:      v1.Params{tc.original},
							Matrix: &v1.Matrix{
								Params: v1.Params{tc.original},
							},
						},
					},
					Finally: []v1.PipelineTask{{
						DisplayName: tc.displayName,
						Params:      v1.Params{tc.original},
					}},
				},
			}
			got := resources.ApplyContexts(&orig.Spec, orig.Name, tc.pr)
			if d := cmp.Diff(tc.expected, got.Tasks[0].Params[0]); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
			if d := cmp.Diff(tc.expected, got.Finally[0].Params[0]); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
			if d := cmp.Diff(tc.expected, got.Tasks[0].Matrix.Params[0]); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
			if d := cmp.Diff(tc.expectedDisplayName, got.Tasks[0].DisplayName); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
			if d := cmp.Diff(tc.expectedDisplayName, got.Finally[0].DisplayName); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestApplyMatrixIncludeWhenExpressions(t *testing.T) {
	for _, tc := range []struct {
		description string
		pt          *v1.PipelineTask
		pr          *v1.PipelineRun
		want        v1.IncludeParamsList
	}{
		{
			description: "string replacement",
			pt: &v1.PipelineTask{
				Matrix: &v1.Matrix{
					Include: v1.IncludeParamsList{
						{
							When: v1.WhenExpressions{
								{
									Input:    "arm64",
									Operator: selection.In,
									Values:   []string{"$(params.platform)"},
								},
							},
						},
					},
				},
			},
			pr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Params: v1.Params{
						{
							Name:  "platform",
							Value: *v1.NewStructuredValues("arm64"),
						},
					},
				},
			},
			want: v1.IncludeParamsList{
				{
					When: v1.WhenExpressions{
						{
							Input:    "arm64",
							Operator: selection.In,
							Values:   []string{"arm64"},
						},
					},
				},
			},
		}, {
			description: "array replacement",
			pt: &v1.PipelineTask{
				Matrix: &v1.Matrix{
					Include: v1.IncludeParamsList{
						{
							When: v1.WhenExpressions{
								{
									Input:    "arm64",
									Operator: selection.In,
									Values:   []string{"$(params.platform[*])"},
								},
							},
						},
					},
				},
			},
			pr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Params: v1.Params{
						{
							Name: "platform",
							Value: v1.ParamValue{
								Type:     v1.ParamTypeArray,
								ArrayVal: []string{"arm64", "amd64"},
							},
						},
					},
				},
			},
			want: v1.IncludeParamsList{
				{
					When: v1.WhenExpressions{
						{
							Input:    "arm64",
							Operator: selection.In,
							Values:   []string{"arm64", "amd64"},
						},
					},
				},
			},
		},
	} {
		t.Run(tc.description, func(t *testing.T) {
			resources.ApplyMatrixIncludeWhenExpressions(tc.pt, tc.pr)
			if d := cmp.Diff(tc.want, tc.pt.Matrix.Include); d != "" {
				t.Errorf(diff.PrintWantGot(d))
			}
		})
	}
}

func TestApplyPipelineTaskContexts(t *testing.T) {
	for _, tc := range []struct {
		description string
		pt          v1.PipelineTask
		prstatus    v1.PipelineRunStatus
		facts       *resources.PipelineRunFacts
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
	}, {
		description: "matrix length context variable",
		pt: v1.PipelineTask{
			Params: v1.Params{{
				Name:  "matrixlength",
				Value: *v1.NewStructuredValues("$(tasks.matrixed-task-run.matrix.length)"),
			}},
		},
		prstatus: v1.PipelineRunStatus{
			PipelineRunStatusFields: v1.PipelineRunStatusFields{
				PipelineSpec: &v1.PipelineSpec{
					Tasks: []v1.PipelineTask{{
						Name: "matrixed-task-run",
						Matrix: &v1.Matrix{
							Params: v1.Params{
								{Name: "platform", Value: *v1.NewStructuredValues("linux", "mac", "windows")},
								{Name: "browser", Value: *v1.NewStructuredValues("chrome", "firefox", "safari")},
							},
						},
					}},
				},
			},
		},
		want: v1.PipelineTask{
			Params: v1.Params{{
				Name:  "matrixlength",
				Value: *v1.NewStructuredValues("9"),
			}},
		},
	}, {
		description: "matrix length and matrix results length context variables in matrix include params ",
		pt: v1.PipelineTask{
			DisplayName: "A task created $(tasks.matrix-emitting-results.matrix.length) instances and each produced $(tasks.matrix-emitting-results.matrix.IMAGE-DIGEST.length) results",
			Params: v1.Params{{
				Name:  "matrixlength",
				Value: *v1.NewStructuredValues("$(tasks.matrix-emitting-results.matrix.length)"),
			}, {
				Name:  "matrixresultslength",
				Value: *v1.NewStructuredValues("$(tasks.matrix-emitting-results.matrix.IMAGE-DIGEST.length)"),
			}},
		},
		prstatus: v1.PipelineRunStatus{
			PipelineRunStatusFields: v1.PipelineRunStatusFields{
				PipelineSpec: &v1.PipelineSpec{
					Tasks: []v1.PipelineTask{
						{
							Name: "matrix-emitting-results",
							TaskSpec: &v1.EmbeddedTask{
								TaskSpec: v1.TaskSpec{
									Params: []v1.ParamSpec{{
										Name: "IMAGE",
										Type: v1.ParamTypeString,
									}, {
										Name: "DIGEST",
										Type: v1.ParamTypeString,
									}},
									Results: []v1.TaskResult{{
										Name: "IMAGE-DIGEST",
									}},
									Steps: []v1.Step{{
										Name:   "produce-results",
										Image:  "bash:latest",
										Script: `#!/usr/bin/env bash\necho -n "$(params.DIGEST)" | sha256sum | tee $(results.IMAGE-DIGEST.path)"`,
									}},
								},
							},
							Matrix: &v1.Matrix{
								Include: []v1.IncludeParams{{
									Name: "build-1",
									Params: v1.Params{{
										Name: "DOCKERFILE", Value: *v1.NewStructuredValues("path/to/Dockerfile1"),
									}, {
										Name: "IMAGE", Value: *v1.NewStructuredValues("image-1"),
									}},
								}, {
									Name: "build-2",
									Params: v1.Params{{
										Name: "DOCKERFILE", Value: *v1.NewStructuredValues("path/to/Dockerfile2"),
									}, {
										Name: "IMAGE", Value: *v1.NewStructuredValues("image-2"),
									}},
								}, {
									Name: "build-3",
									Params: v1.Params{{
										Name: "DOCKERFILE", Value: *v1.NewStructuredValues("path/to/Dockerfile3"),
									}, {
										Name: "IMAGE", Value: *v1.NewStructuredValues("image-3"),
									}},
								}},
							},
						},
					},
				},
			},
		},
		facts: &resources.PipelineRunFacts{
			State: resources.PipelineRunState{{
				PipelineTask: &v1.PipelineTask{
					Name: "matrix-emitting-results",
				},
				TaskRunNames: []string{"matrix-emitting-results-0"},
				TaskRuns: []*v1.TaskRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "matrix-emitting-results-0",
						},
						Status: v1.TaskRunStatus{
							TaskRunStatusFields: v1.TaskRunStatusFields{
								Results: []v1.TaskRunResult{{
									Name:  "IMAGE-DIGEST",
									Value: *v1.NewStructuredValues("123"),
								}},
							},
						},
					}, {
						ObjectMeta: metav1.ObjectMeta{
							Name: "matrix-emitting-results-1",
						},
						Status: v1.TaskRunStatus{
							TaskRunStatusFields: v1.TaskRunStatusFields{
								Results: []v1.TaskRunResult{{
									Name:  "IMAGE-DIGEST",
									Value: *v1.NewStructuredValues("456"),
								}},
							},
						},
					}, {
						ObjectMeta: metav1.ObjectMeta{
							Name: "matrix-emitting-results-2",
						},
						Status: v1.TaskRunStatus{
							TaskRunStatusFields: v1.TaskRunStatusFields{
								Results: []v1.TaskRunResult{{
									Name:  "IMAGE-DIGEST",
									Value: *v1.NewStructuredValues("789"),
								}},
							},
						},
					},
				},
				ResultsCache: map[string][]string{},
			}},
		},
		want: v1.PipelineTask{
			DisplayName: "A task created 3 instances and each produced 3 results",
			Params: v1.Params{{
				Name:  "matrixlength",
				Value: *v1.NewStructuredValues("3"),
			}, {
				Name:  "matrixresultslength",
				Value: *v1.NewStructuredValues("3"),
			}},
		},
	}} {
		t.Run(tc.description, func(t *testing.T) {
			got := resources.ApplyPipelineTaskContexts(&tc.pt, tc.prstatus, tc.facts)
			if d := cmp.Diff(&tc.want, got); d != "" {
				t.Error(diff.PrintWantGot(d))
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
	}{
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
			expectedError: errors.New("invalid pipelineresults [pipeline-result-1], the referred result don't exist"),
		},
	} {
		t.Run(tc.description, func(t *testing.T) {
			received, _ := resources.ApplyTaskResultsToPipelineResults(context.Background(), tc.results, tc.taskResults, tc.runResults, nil /* skippedTasks */)
			if d := cmp.Diff(tc.expected, received); d != "" {
				t.Error(diff.PrintWantGot(d))
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
			},
		},
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
				t.Error(diff.PrintWantGot(d))
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
		expectedError:   errors.New("invalid pipelineresults [pipeline-result-1], the referenced results don't exist"),
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
		expectedError:   errors.New("invalid pipelineresults [pipeline-result-1], the referenced results don't exist"),
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
		expectedError:   errors.New("invalid pipelineresults [pipeline-result-1], the referenced results don't exist"),
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
		expectedError:   errors.New("invalid pipelineresults [foo], the referenced results don't exist"),
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
		expectedError:   errors.New("invalid pipelineresults [foo], the referenced results don't exist"),
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
		expectedError:   errors.New("invalid pipelineresults [foo], the referenced results don't exist"),
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
		expectedError:   errors.New("invalid pipelineresults [foo], the referenced results don't exist"),
	}, {
		description: "unsuccessful-taskrun-no-returned-result",
		results: []v1.PipelineResult{{
			Name:  "foo",
			Value: *v1.NewStructuredValues("$(tasks.pt1.results.foo)"),
		}},
		taskResults:     map[string][]v1.TaskRunResult{},
		expectedResults: nil,
		expectedError:   errors.New("invalid pipelineresults [foo], the referenced results don't exist"),
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
		expectedError: errors.New("invalid pipelineresults [foo], the referenced results don't exist"),
	}, {
		description: "no-run-results-no-returned-results",
		results: []v1.PipelineResult{{
			Name:  "foo",
			Value: *v1.NewStructuredValues("$(tasks.customtask.results.foo)"),
		}},
		runResults:      map[string][]v1beta1.CustomRunResult{},
		expectedResults: nil,
		expectedError:   errors.New("invalid pipelineresults [foo], the referenced results don't exist"),
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
		expectedError:   errors.New("invalid pipelineresults [foo], the referenced results don't exist"),
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
		expectedError:   errors.New("invalid pipelineresults [foo], the referenced results don't exist"),
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
		expectedError:   errors.New("invalid pipelineresults [foo], the referenced results don't exist"),
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
		expectedError:   errors.New("invalid pipelineresults [foo], the referenced results don't exist"),
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
				t.Error(diff.PrintWantGot(d))
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
			Name:        "task4",
			DisplayName: "Task 1 $(tasks.task1.status) but Task 3 exited with $(tasks.task3.status)",
			TaskRef:     &v1.TaskRef{Name: "task"},
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
			Name:        "task4",
			DisplayName: "Task 1 succeeded but Task 3 exited with none",
			TaskRef:     &v1.TaskRef{Name: "task"},
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

func TestPropagateResults(t *testing.T) {
	for _, tt := range []struct {
		name                 string
		resolvedTask         *resources.ResolvedPipelineTask
		runStates            resources.PipelineRunState
		expectedResolvedTask *resources.ResolvedPipelineTask
	}{
		{
			name: "propagate string result",
			resolvedTask: &resources.ResolvedPipelineTask{
				ResolvedTask: &taskresources.ResolvedTask{
					TaskSpec: &v1.TaskSpec{
						Steps: []v1.Step{
							{
								Name:    "$(tasks.pt1.results.r1)",
								Command: []string{"$(tasks.pt1.results.r2)"},
								Args:    []string{"$(tasks.pt2.results.r1)"},
							},
						},
					},
				},
			},
			runStates: resources.PipelineRunState{
				{
					PipelineTask: &v1.PipelineTask{
						Name: "pt1",
					},
					TaskRuns: []*v1.TaskRun{
						{
							Status: v1.TaskRunStatus{
								Status: duckv1.Status{
									Conditions: duckv1.Conditions{
										{
											Type:   apis.ConditionSucceeded,
											Status: corev1.ConditionTrue,
										},
									},
								},
								TaskRunStatusFields: v1.TaskRunStatusFields{
									Results: []v1.TaskRunResult{
										{
											Name: "r1",
											Type: v1.ResultsTypeString,
											Value: v1.ResultValue{
												StringVal: "step1",
											},
										},
										{
											Name: "r2",
											Type: v1.ResultsTypeString,
											Value: v1.ResultValue{
												StringVal: "echo",
											},
										},
									},
								},
							},
						},
					},
				}, {
					PipelineTask: &v1.PipelineTask{
						Name: "pt2",
					},
					TaskRuns: []*v1.TaskRun{
						{
							Status: v1.TaskRunStatus{
								Status: duckv1.Status{
									Conditions: duckv1.Conditions{
										{
											Type:   apis.ConditionSucceeded,
											Status: corev1.ConditionTrue,
										},
									},
								},
								TaskRunStatusFields: v1.TaskRunStatusFields{
									Results: []v1.TaskRunResult{
										{
											Name: "r1",
											Type: v1.ResultsTypeString,
											Value: v1.ResultValue{
												StringVal: "arg1",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedResolvedTask: &resources.ResolvedPipelineTask{
				ResolvedTask: &taskresources.ResolvedTask{
					TaskSpec: &v1.TaskSpec{
						Steps: []v1.Step{
							{
								Name:    "step1",
								Command: []string{"echo"},
								Args:    []string{"arg1"},
							},
						},
					},
				},
			},
		}, {
			name: "propagate array result",
			resolvedTask: &resources.ResolvedPipelineTask{
				ResolvedTask: &taskresources.ResolvedTask{
					TaskSpec: &v1.TaskSpec{
						Steps: []v1.Step{
							{
								Command: []string{"$(tasks.pt1.results.r1[*])"},
								Args:    []string{"$(tasks.pt2.results.r1[*])"},
							},
						},
					},
				},
			},
			runStates: resources.PipelineRunState{
				{
					PipelineTask: &v1.PipelineTask{
						Name: "pt1",
					},
					TaskRuns: []*v1.TaskRun{
						{
							Status: v1.TaskRunStatus{
								Status: duckv1.Status{
									Conditions: duckv1.Conditions{
										{
											Type:   apis.ConditionSucceeded,
											Status: corev1.ConditionTrue,
										},
									},
								},
								TaskRunStatusFields: v1.TaskRunStatusFields{
									Results: []v1.TaskRunResult{
										{
											Name: "r1",
											Type: v1.ResultsTypeArray,
											Value: v1.ResultValue{
												ArrayVal: []string{"bash", "-c"},
											},
										},
									},
								},
							},
						},
					},
				}, {
					PipelineTask: &v1.PipelineTask{
						Name: "pt2",
					},
					TaskRuns: []*v1.TaskRun{
						{
							Status: v1.TaskRunStatus{
								Status: duckv1.Status{
									Conditions: duckv1.Conditions{
										{
											Type:   apis.ConditionSucceeded,
											Status: corev1.ConditionTrue,
										},
									},
								},
								TaskRunStatusFields: v1.TaskRunStatusFields{
									Results: []v1.TaskRunResult{
										{
											Name: "r1",
											Type: v1.ResultsTypeArray,
											Value: v1.ResultValue{
												ArrayVal: []string{"echo", "arg1"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedResolvedTask: &resources.ResolvedPipelineTask{
				ResolvedTask: &taskresources.ResolvedTask{
					TaskSpec: &v1.TaskSpec{
						Steps: []v1.Step{
							{
								Command: []string{"bash", "-c"},
								Args:    []string{"echo", "arg1"},
							},
						},
					},
				},
			},
		}, {
			name: "propagate object result",
			resolvedTask: &resources.ResolvedPipelineTask{
				ResolvedTask: &taskresources.ResolvedTask{
					TaskSpec: &v1.TaskSpec{
						Steps: []v1.Step{
							{
								Command: []string{"$(tasks.pt1.results.r1.command1)", "$(tasks.pt1.results.r1.command2)"},
								Args:    []string{"$(tasks.pt2.results.r1.arg1)", "$(tasks.pt2.results.r1.arg2)"},
							},
						},
					},
				},
			},
			runStates: resources.PipelineRunState{
				{
					PipelineTask: &v1.PipelineTask{
						Name: "pt1",
					},
					TaskRuns: []*v1.TaskRun{
						{
							Status: v1.TaskRunStatus{
								Status: duckv1.Status{
									Conditions: duckv1.Conditions{
										{
											Type:   apis.ConditionSucceeded,
											Status: corev1.ConditionTrue,
										},
									},
								},
								TaskRunStatusFields: v1.TaskRunStatusFields{
									Results: []v1.TaskRunResult{
										{
											Name: "r1",
											Type: v1.ResultsTypeObject,
											Value: v1.ResultValue{
												ObjectVal: map[string]string{
													"command1": "bash",
													"command2": "-c",
												},
											},
										},
									},
								},
							},
						},
					},
				}, {
					PipelineTask: &v1.PipelineTask{
						Name: "pt2",
					},
					TaskRuns: []*v1.TaskRun{
						{
							Status: v1.TaskRunStatus{
								Status: duckv1.Status{
									Conditions: duckv1.Conditions{
										{
											Type:   apis.ConditionSucceeded,
											Status: corev1.ConditionTrue,
										},
									},
								},
								TaskRunStatusFields: v1.TaskRunStatusFields{
									Results: []v1.TaskRunResult{
										{
											Name: "r1",
											Type: v1.ResultsTypeObject,
											Value: v1.ResultValue{
												ObjectVal: map[string]string{
													"arg1": "echo",
													"arg2": "arg1",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedResolvedTask: &resources.ResolvedPipelineTask{
				ResolvedTask: &taskresources.ResolvedTask{
					TaskSpec: &v1.TaskSpec{
						Steps: []v1.Step{
							{
								Command: []string{"bash", "-c"},
								Args:    []string{"echo", "arg1"},
							},
						},
					},
				},
			},
		}, {
			name: "not propagate result when resolved task is nil",
			resolvedTask: &resources.ResolvedPipelineTask{
				ResolvedTask: nil,
			},
			runStates: resources.PipelineRunState{},
			expectedResolvedTask: &resources.ResolvedPipelineTask{
				ResolvedTask: nil,
			},
		}, {
			name: "not propagate result when taskSpec is nil",
			resolvedTask: &resources.ResolvedPipelineTask{
				ResolvedTask: &taskresources.ResolvedTask{
					TaskSpec: nil,
				},
			},
			runStates: resources.PipelineRunState{},
			expectedResolvedTask: &resources.ResolvedPipelineTask{
				ResolvedTask: &taskresources.ResolvedTask{
					TaskSpec: nil,
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resources.PropagateResults(tt.resolvedTask, tt.runStates)
			if d := cmp.Diff(tt.expectedResolvedTask, tt.resolvedTask); d != "" {
				t.Fatalf("PropagateResults() %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestPropagateArtifacts(t *testing.T) {
	for _, tt := range []struct {
		name                 string
		resolvedTask         *resources.ResolvedPipelineTask
		runStates            resources.PipelineRunState
		expectedResolvedTask *resources.ResolvedPipelineTask
		wantErr              bool
	}{
		{
			name: "not propagate artifact when resolved task is nil",
			resolvedTask: &resources.ResolvedPipelineTask{
				ResolvedTask: nil,
			},
			runStates: resources.PipelineRunState{},
			expectedResolvedTask: &resources.ResolvedPipelineTask{
				ResolvedTask: nil,
			},
		},
		{
			name: "not propagate artifact when taskSpec is nil",
			resolvedTask: &resources.ResolvedPipelineTask{
				ResolvedTask: &taskresources.ResolvedTask{
					TaskSpec: nil,
				},
			},
			runStates: resources.PipelineRunState{},
			expectedResolvedTask: &resources.ResolvedPipelineTask{
				ResolvedTask: &taskresources.ResolvedTask{
					TaskSpec: nil,
				},
			},
		},
		{
			name: "propagate artifacts inputs",
			resolvedTask: &resources.ResolvedPipelineTask{
				ResolvedTask: &taskresources.ResolvedTask{
					TaskSpec: &v1.TaskSpec{
						Steps: []v1.Step{
							{
								Name:    "get-artifacts-inputs-from-pt1",
								Command: []string{"$(tasks.pt1.inputs.source)"},
								Args:    []string{"$(tasks.pt1.inputs.source)"},
							},
						},
					},
				},
			},
			runStates: resources.PipelineRunState{
				{
					PipelineTask: &v1.PipelineTask{
						Name: "pt1",
					},
					TaskRuns: []*v1.TaskRun{
						{
							Status: v1.TaskRunStatus{
								Status: duckv1.Status{
									Conditions: duckv1.Conditions{
										{
											Type:   apis.ConditionSucceeded,
											Status: corev1.ConditionTrue,
										},
									},
								},
								TaskRunStatusFields: v1.TaskRunStatusFields{
									Artifacts: &v1.Artifacts{
										Inputs:  []v1.Artifact{{Name: "source", Values: []v1.ArtifactValue{{Digest: map[v1.Algorithm]string{"sha256": "b35cacccfdb1e24dc497d15d553891345fd155713ffe647c281c583269eaaae0"}, Uri: "pkg:example.github.com/inputs"}}}},
										Outputs: nil,
									},
								},
							},
						},
					},
				},
			},
			expectedResolvedTask: &resources.ResolvedPipelineTask{
				ResolvedTask: &taskresources.ResolvedTask{
					TaskSpec: &v1.TaskSpec{
						Steps: []v1.Step{
							{
								Name:    "get-artifacts-inputs-from-pt1",
								Command: []string{`[{"digest":{"sha256":"b35cacccfdb1e24dc497d15d553891345fd155713ffe647c281c583269eaaae0"},"uri":"pkg:example.github.com/inputs"}]`},
								Args:    []string{`[{"digest":{"sha256":"b35cacccfdb1e24dc497d15d553891345fd155713ffe647c281c583269eaaae0"},"uri":"pkg:example.github.com/inputs"}]`},
							},
						},
					},
				},
			},
		},
		{
			name: "propagate artifacts outputs",
			resolvedTask: &resources.ResolvedPipelineTask{
				ResolvedTask: &taskresources.ResolvedTask{
					TaskSpec: &v1.TaskSpec{
						Steps: []v1.Step{
							{
								Name:    "get-artifacts-outputs-from-pt1",
								Command: []string{"$(tasks.pt1.outputs.image)"},
								Args:    []string{"$(tasks.pt1.outputs.image)"},
							},
						},
					},
				},
			},
			runStates: resources.PipelineRunState{
				{
					PipelineTask: &v1.PipelineTask{
						Name: "pt1",
					},
					TaskRuns: []*v1.TaskRun{
						{
							Status: v1.TaskRunStatus{
								Status: duckv1.Status{
									Conditions: duckv1.Conditions{
										{
											Type:   apis.ConditionSucceeded,
											Status: corev1.ConditionTrue,
										},
									},
								},
								TaskRunStatusFields: v1.TaskRunStatusFields{
									Artifacts: &v1.Artifacts{
										Inputs:  []v1.Artifact{{Name: "source", Values: []v1.ArtifactValue{{Digest: map[v1.Algorithm]string{"sha256": "b35cacccfdb1e24dc497d15d553891345fd155713ffe647c281c583269eaaae0"}, Uri: "pkg:example.github.com/inputs"}}}},
										Outputs: []v1.Artifact{{Name: "image", Values: []v1.ArtifactValue{{Digest: map[v1.Algorithm]string{"sha1": "95588b8f34c31eb7d62c92aaa4e6506639b06ef2"}, Uri: "pkg:github/package-url/purl-spec@244fd47e07d1004f0aed9c"}}}},
									},
								},
							},
						},
					},
				},
			},
			expectedResolvedTask: &resources.ResolvedPipelineTask{
				ResolvedTask: &taskresources.ResolvedTask{
					TaskSpec: &v1.TaskSpec{
						Steps: []v1.Step{
							{
								Name:    "get-artifacts-outputs-from-pt1",
								Command: []string{`[{"digest":{"sha1":"95588b8f34c31eb7d62c92aaa4e6506639b06ef2"},"uri":"pkg:github/package-url/purl-spec@244fd47e07d1004f0aed9c"}]`},
								Args:    []string{`[{"digest":{"sha1":"95588b8f34c31eb7d62c92aaa4e6506639b06ef2"},"uri":"pkg:github/package-url/purl-spec@244fd47e07d1004f0aed9c"}]`},
							},
						},
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := resources.PropagateArtifacts(tt.resolvedTask, tt.runStates)
			if tt.wantErr != (err != nil) {
				t.Fatalf("Failed to check err want %t, got %v", tt.wantErr, err)
			}
			if d := cmp.Diff(tt.expectedResolvedTask, tt.resolvedTask); d != "" {
				t.Fatalf("TestPropagateArtifacts() %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestApplyParametersToWorkspaceBindings(t *testing.T) {
	testCases := []struct {
		name       string
		ps         *v1.PipelineSpec
		pr         *v1.PipelineRun
		expectedPr *v1.PipelineRun
	}{
		{
			name: "pvc",
			ps: &v1.PipelineSpec{
				Params: []v1.ParamSpec{
					{Name: "pvc-name", Type: v1.ParamTypeString},
				},
			},
			pr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Params: []v1.Param{
						{Name: "pvc-name", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "claim-value"}},
					},
					Workspaces: []v1.WorkspaceBinding{
						{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "$(params.pvc-name)",
							},
						},
					},
				},
			},
			expectedPr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Params: []v1.Param{
						{Name: "pvc-name", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "claim-value"}},
					},
					Workspaces: []v1.WorkspaceBinding{
						{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "claim-value",
							},
						},
					},
				},
			},
		},
		{
			name: "subpath",
			ps: &v1.PipelineSpec{
				Params: []v1.ParamSpec{
					{Name: "subpath-value", Type: v1.ParamTypeString},
				},
			},
			pr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Params: []v1.Param{
						{Name: "subpath-value", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "sub/path"}},
					},
					Workspaces: []v1.WorkspaceBinding{
						{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
							SubPath:  "$(params.subpath-value)",
						},
					},
				},
			},
			expectedPr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Params: []v1.Param{
						{Name: "subpath-value", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "sub/path"}},
					},
					Workspaces: []v1.WorkspaceBinding{
						{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
							SubPath:  "sub/path",
						},
					},
				},
			},
		},
		{
			name: "configmap",
			ps: &v1.PipelineSpec{
				Params: []v1.ParamSpec{
					{Name: "configmap-name", Type: v1.ParamTypeString},
				},
			},
			pr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Params: []v1.Param{
						{Name: "configmap-name", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "config-map-value"}},
					},
					Workspaces: []v1.WorkspaceBinding{
						{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "$(params.configmap-name)",
								},
							},
						},
					},
				},
			},
			expectedPr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Params: []v1.Param{
						{Name: "configmap-name", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "config-map-value"}},
					},
					Workspaces: []v1.WorkspaceBinding{
						{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "config-map-value",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "secret",
			ps: &v1.PipelineSpec{
				Params: []v1.ParamSpec{
					{Name: "secret-name", Type: v1.ParamTypeString},
				},
			},
			pr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Params: []v1.Param{
						{Name: "secret-name", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "secret-value"}},
					},
					Workspaces: []v1.WorkspaceBinding{
						{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "$(params.secret-name)",
							},
						},
					},
				},
			},
			expectedPr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Params: []v1.Param{
						{Name: "secret-name", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "secret-value"}},
					},
					Workspaces: []v1.WorkspaceBinding{
						{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "secret-value",
							},
						},
					},
				},
			},
		},
		{
			name: "projected-secret",
			ps: &v1.PipelineSpec{
				Params: []v1.ParamSpec{
					{Name: "projected-secret-name", Type: v1.ParamTypeString},
				},
			},
			pr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Params: []v1.Param{
						{Name: "projected-secret-name", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "projected-secret-value"}},
					},
					Workspaces: []v1.WorkspaceBinding{
						{
							Projected: &corev1.ProjectedVolumeSource{
								Sources: []corev1.VolumeProjection{
									{
										Secret: &corev1.SecretProjection{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "$(params.projected-secret-name)",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedPr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Params: []v1.Param{
						{Name: "projected-secret-name", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "projected-secret-value"}},
					},
					Workspaces: []v1.WorkspaceBinding{
						{
							Projected: &corev1.ProjectedVolumeSource{
								Sources: []corev1.VolumeProjection{
									{
										Secret: &corev1.SecretProjection{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "projected-secret-value",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "projected-configmap",
			ps: &v1.PipelineSpec{
				Params: []v1.ParamSpec{
					{Name: "projected-configmap-name", Type: v1.ParamTypeString},
				},
			},
			pr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Params: []v1.Param{
						{Name: "projected-configmap-name", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "projected-config-value"}},
					},
					Workspaces: []v1.WorkspaceBinding{
						{
							Projected: &corev1.ProjectedVolumeSource{
								Sources: []corev1.VolumeProjection{
									{
										ConfigMap: &corev1.ConfigMapProjection{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "$(params.projected-configmap-name)",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedPr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Params: []v1.Param{
						{Name: "projected-configmap-name", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "projected-config-value"}},
					},
					Workspaces: []v1.WorkspaceBinding{
						{
							Projected: &corev1.ProjectedVolumeSource{
								Sources: []corev1.VolumeProjection{
									{
										ConfigMap: &corev1.ConfigMapProjection{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "projected-config-value",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "csi-driver",
			ps: &v1.PipelineSpec{
				Params: []v1.ParamSpec{
					{Name: "csi-driver", Type: v1.ParamTypeString},
				},
			},
			pr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Params: []v1.Param{
						{Name: "csi-driver", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "csi-driver-value"}},
					},
					Workspaces: []v1.WorkspaceBinding{
						{
							CSI: &corev1.CSIVolumeSource{Driver: "$(params.csi-driver)"},
						},
					},
				},
			},
			expectedPr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Params: []v1.Param{
						{Name: "csi-driver", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "csi-driver-value"}},
					},
					Workspaces: []v1.WorkspaceBinding{
						{
							CSI: &corev1.CSIVolumeSource{Driver: "csi-driver-value"},
						},
					},
				},
			},
		},
		{
			name: "csi-nodePublishSecretRef-name",
			ps: &v1.PipelineSpec{
				Params: []v1.ParamSpec{
					{Name: "csi-nodePublishSecretRef-name", Type: v1.ParamTypeString},
				},
			},
			pr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Params: []v1.Param{
						{Name: "csi-nodePublishSecretRef-name", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "csi-nodePublishSecretRef-value"}},
					},
					Workspaces: []v1.WorkspaceBinding{
						{
							CSI: &corev1.CSIVolumeSource{NodePublishSecretRef: &corev1.LocalObjectReference{Name: "$(params.csi-nodePublishSecretRef-name)"}},
						},
					},
				},
			},
			expectedPr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Params: []v1.Param{
						{Name: "csi-nodePublishSecretRef-name", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "csi-nodePublishSecretRef-value"}},
					},
					Workspaces: []v1.WorkspaceBinding{
						{
							CSI: &corev1.CSIVolumeSource{NodePublishSecretRef: &corev1.LocalObjectReference{Name: "csi-nodePublishSecretRef-value"}},
						},
					},
				},
			},
		},
		{
			name: "pvc object params",
			ps: &v1.PipelineSpec{
				Params: []v1.ParamSpec{
					{Name: "pvc-object", Type: v1.ParamTypeObject, Properties: map[string]v1.PropertySpec{
						"name": {Type: v1.ParamTypeString},
					}},
				},
			},
			pr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Params: []v1.Param{
						{Name: "pvc-object", Value: v1.ParamValue{Type: v1.ParamTypeObject, ObjectVal: map[string]string{
							"name": "pvc-object-value",
						}}},
					},
					Workspaces: []v1.WorkspaceBinding{
						{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "$(params.pvc-object.name)",
							},
						},
					},
				},
			},
			expectedPr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Params: []v1.Param{
						{Name: "pvc-object", Value: v1.ParamValue{Type: v1.ParamTypeObject, ObjectVal: map[string]string{
							"name": "pvc-object-value",
						}}},
					},
					Workspaces: []v1.WorkspaceBinding{
						{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "pvc-object-value",
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			resources.ApplyParametersToWorkspaceBindings(context.TODO(), tt.pr)
			if d := cmp.Diff(tt.expectedPr, tt.pr); d != "" {
				t.Fatalf("TestApplyParametersToWorkspaceBindings() %s, got: %v", tt.name, diff.PrintWantGot(d))
			}
		})
	}
}

func TestApplyResultsToWorkspaceBindings(t *testing.T) {
	testCases := []struct {
		name       string
		trResults  map[string][]v1.TaskRunResult
		pr         *v1.PipelineRun
		expectedPr *v1.PipelineRun
	}{
		{
			name: "pvc",
			trResults: map[string][]v1.TaskRunResult{
				"task1": {
					{
						Name:  "pvc-name",
						Type:  v1.ResultsTypeString,
						Value: v1.ResultValue{StringVal: "claim-value"},
					},
				},
			},
			pr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Workspaces: []v1.WorkspaceBinding{
						{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "$(tasks.task1.results.pvc-name)",
							},
						},
					},
				},
			},
			expectedPr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Workspaces: []v1.WorkspaceBinding{
						{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "claim-value",
							},
						},
					},
				},
			},
		},
		{
			name: "subPath",
			trResults: map[string][]v1.TaskRunResult{
				"task2": {
					{
						Name:  "subpath-value",
						Type:  v1.ResultsTypeString,
						Value: v1.ResultValue{StringVal: "sub/path"},
					},
				},
			},
			pr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Workspaces: []v1.WorkspaceBinding{
						{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
							SubPath:  "$(tasks.task2.results.subpath-value)",
						},
					},
				},
			},
			expectedPr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Workspaces: []v1.WorkspaceBinding{
						{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
							SubPath:  "sub/path",
						},
					},
				},
			},
		},
		{
			name: "configmap name",
			trResults: map[string][]v1.TaskRunResult{
				"task3": {
					{
						Name:  "configmap-name",
						Type:  v1.ResultsTypeString,
						Value: v1.ResultValue{StringVal: "config-map-value"},
					},
				},
			},
			pr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Workspaces: []v1.WorkspaceBinding{
						{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: "$(tasks.task3.results.configmap-name)"},
							},
						},
					},
				},
			},
			expectedPr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Workspaces: []v1.WorkspaceBinding{
						{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: "config-map-value"},
							},
						},
					},
				},
			},
		},
		{
			name: "secret.secretName",
			trResults: map[string][]v1.TaskRunResult{
				"task4": {
					{
						Name:  "secret-name",
						Type:  v1.ResultsTypeString,
						Value: v1.ResultValue{StringVal: "secret-value"},
					},
				},
			},
			pr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Workspaces: []v1.WorkspaceBinding{
						{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "$(tasks.task4.results.secret-name)",
							},
						},
					},
				},
			},
			expectedPr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Workspaces: []v1.WorkspaceBinding{
						{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "secret-value",
							},
						},
					},
				},
			},
		},
		{
			name: "projected-configmap",
			trResults: map[string][]v1.TaskRunResult{
				"task5": {
					{
						Name:  "projected-configmap-name",
						Type:  v1.ResultsTypeString,
						Value: v1.ResultValue{StringVal: "projected-config-map-value"},
					},
				},
			},
			pr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Workspaces: []v1.WorkspaceBinding{
						{
							Projected: &corev1.ProjectedVolumeSource{
								Sources: []corev1.VolumeProjection{
									{
										ConfigMap: &corev1.ConfigMapProjection{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "$(tasks.task5.results.projected-configmap-name)",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedPr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Workspaces: []v1.WorkspaceBinding{
						{
							Projected: &corev1.ProjectedVolumeSource{
								Sources: []corev1.VolumeProjection{
									{
										ConfigMap: &corev1.ConfigMapProjection{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "projected-config-map-value",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "projected-secret",
			trResults: map[string][]v1.TaskRunResult{
				"task6": {
					{
						Name:  "projected-secret-name",
						Type:  v1.ResultsTypeString,
						Value: v1.ResultValue{StringVal: "projected-secret-value"},
					},
				},
			},
			pr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Workspaces: []v1.WorkspaceBinding{
						{
							Projected: &corev1.ProjectedVolumeSource{
								Sources: []corev1.VolumeProjection{
									{
										Secret: &corev1.SecretProjection{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "$(tasks.task6.results.projected-secret-name)",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedPr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Workspaces: []v1.WorkspaceBinding{
						{
							Projected: &corev1.ProjectedVolumeSource{
								Sources: []corev1.VolumeProjection{
									{
										Secret: &corev1.SecretProjection{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "projected-secret-value",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "csi-driver",
			trResults: map[string][]v1.TaskRunResult{
				"task6": {
					{
						Name:  "driver-name",
						Type:  v1.ResultsTypeString,
						Value: v1.ResultValue{StringVal: "driver-value"},
					},
				},
			},
			pr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Workspaces: []v1.WorkspaceBinding{
						{
							CSI: &corev1.CSIVolumeSource{
								Driver: "$(tasks.task6.results.driver-name)",
							},
						},
					},
				},
			},
			expectedPr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Workspaces: []v1.WorkspaceBinding{
						{
							CSI: &corev1.CSIVolumeSource{
								Driver: "driver-value",
							},
						},
					},
				},
			},
		},
		{
			name: "csi-NodePublishSecretRef-name",
			trResults: map[string][]v1.TaskRunResult{
				"task6": {
					{
						Name:  "ref-name",
						Type:  v1.ResultsTypeString,
						Value: v1.ResultValue{StringVal: "ref-value"},
					},
				},
			},
			pr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Workspaces: []v1.WorkspaceBinding{
						{
							CSI: &corev1.CSIVolumeSource{
								Driver:               "driver",
								NodePublishSecretRef: &corev1.LocalObjectReference{Name: "$(tasks.task6.results.ref-name)"},
							},
						},
					},
				},
			},
			expectedPr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Workspaces: []v1.WorkspaceBinding{
						{
							CSI: &corev1.CSIVolumeSource{
								Driver:               "driver",
								NodePublishSecretRef: &corev1.LocalObjectReference{Name: "ref-value"},
							},
						},
					},
				},
			},
		},
		{
			name: "pvc object-result",
			trResults: map[string][]v1.TaskRunResult{
				"task1": {
					{
						Name: "pvc-object",
						Type: v1.ResultsTypeObject,
						Value: v1.ResultValue{Type: v1.ParamTypeObject, ObjectVal: map[string]string{
							"name": "pvc-object-value",
						}},
					},
				},
			},
			pr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Workspaces: []v1.WorkspaceBinding{
						{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "$(tasks.task1.results.pvc-object.name)",
							},
						},
					},
				},
			},
			expectedPr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Workspaces: []v1.WorkspaceBinding{
						{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "pvc-object-value",
							},
						},
					},
				},
			},
		},
		{
			name: "pvc object-result - along with array-result (no effect)",
			trResults: map[string][]v1.TaskRunResult{
				"task1": {
					{
						Name: "pvc-object",
						Type: v1.ResultsTypeObject,
						Value: v1.ResultValue{Type: v1.ParamTypeObject, ObjectVal: map[string]string{
							"name": "pvc-object-value",
						}},
					},
					{
						Name:  "pvc-array",
						Type:  v1.ResultsTypeArray,
						Value: v1.ResultValue{Type: v1.ParamTypeArray, ArrayVal: []string{"name-1", "name-2"}},
					},
				},
			},
			pr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Workspaces: []v1.WorkspaceBinding{
						{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "$(tasks.task1.results.pvc-object.name)",
							},
						},
					},
				},
			},
			expectedPr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					Workspaces: []v1.WorkspaceBinding{
						{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "pvc-object-value",
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resources.ApplyResultsToWorkspaceBindings(tc.trResults, tc.pr)
			if d := cmp.Diff(tc.pr, tc.expectedPr); d != "" {
				t.Fatalf("TestApplyResultsToWorkspaceBindings() %s, %v", tc.name, diff.PrintWantGot(d))
			}
		})
	}
}
