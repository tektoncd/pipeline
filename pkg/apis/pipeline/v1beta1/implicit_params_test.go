package v1beta1_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

var (
	cmpOptions = []cmp.Option{
		cmpopts.SortSlices(func(x, y v1beta1.ParamSpec) bool {
			return x.Name < y.Name
		}),
		cmpopts.SortSlices(func(x, y v1beta1.Param) bool {
			return x.Name < y.Name
		}),
		cmpopts.SortSlices(func(x, y v1beta1.PipelineTask) bool {
			return x.Name < y.Name
		}),
	}
)

func TestImplicitParams(t *testing.T) {
	// Configure alpha API config.
	ctx := context.Background()
	cfg := config.FromContextOrDefaults(ctx)
	cfg.FeatureFlags = &config.FeatureFlags{EnableAPIFields: "alpha"}
	ctx = config.ToContext(ctx, cfg)

	for _, tc := range []struct {
		// Expect in and want to be the same type.
		in   apis.Defaultable
		want interface{}
	}{
		// This data needs to be inlined to ensure unique copies on each test.
		{
			in: &v1beta1.PipelineRunSpec{
				Params: []v1beta1.Param{
					{
						Name: "foo",
						Value: v1beta1.ArrayOrString{
							StringVal: "a",
						},
					},
					{
						Name: "bar",
						Value: v1beta1.ArrayOrString{
							ArrayVal: []string{"b"},
						},
					},
				},
				PipelineSpec: &v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{
						{
							TaskSpec: &v1beta1.EmbeddedTask{
								TaskSpec: v1beta1.TaskSpec{},
							},
						},
						{
							TaskRef: &v1beta1.TaskRef{
								Name: "baz",
							},
						},
					},
					Finally: []v1beta1.PipelineTask{
						{
							TaskSpec: &v1beta1.EmbeddedTask{
								TaskSpec: v1beta1.TaskSpec{},
							},
						},
						{
							TaskRef: &v1beta1.TaskRef{
								Name: "baz",
							},
						},
					},
				},
			},
			want: &v1beta1.PipelineRunSpec{
				ServiceAccountName: config.DefaultServiceAccountValue,
				Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
				Params: []v1beta1.Param{
					{
						Name: "foo",
						Value: v1beta1.ArrayOrString{
							StringVal: "a",
						},
					},
					{
						Name: "bar",
						Value: v1beta1.ArrayOrString{
							ArrayVal: []string{"b"},
						},
					},
				},
				PipelineSpec: &v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{
						{
							TaskSpec: &v1beta1.EmbeddedTask{
								TaskSpec: v1beta1.TaskSpec{
									Params: []v1beta1.ParamSpec{
										{
											Name: "foo",
											Type: v1beta1.ParamTypeString,
										},
										{
											Name: "bar",
											Type: v1beta1.ParamTypeArray,
										},
									},
								},
							},
							Params: []v1beta1.Param{
								{
									Name: "foo",
									Value: v1beta1.ArrayOrString{
										Type:      v1beta1.ParamTypeString,
										StringVal: "$(params.foo)",
									},
								},
								{
									Name: "bar",
									Value: v1beta1.ArrayOrString{
										Type:     v1beta1.ParamTypeArray,
										ArrayVal: []string{"$(params.bar[*])"},
									},
								},
							},
						},
						{
							TaskRef: &v1beta1.TaskRef{
								Name: "baz",
								Kind: v1beta1.NamespacedTaskKind,
							},
						},
					},
					Finally: []v1beta1.PipelineTask{
						{
							TaskSpec: &v1beta1.EmbeddedTask{
								TaskSpec: v1beta1.TaskSpec{
									Params: []v1beta1.ParamSpec{
										{
											Name: "foo",
											Type: v1beta1.ParamTypeString,
										},
										{
											Name: "bar",
											Type: v1beta1.ParamTypeArray,
										},
									},
								},
							},
							Params: []v1beta1.Param{
								{
									Name: "foo",
									Value: v1beta1.ArrayOrString{
										Type:      v1beta1.ParamTypeString,
										StringVal: "$(params.foo)",
									},
								},
								{
									Name: "bar",
									Value: v1beta1.ArrayOrString{
										Type:     v1beta1.ParamTypeArray,
										ArrayVal: []string{"$(params.bar[*])"},
									},
								},
							},
						},
						{
							TaskRef: &v1beta1.TaskRef{
								Name: "baz",
								Kind: v1beta1.NamespacedTaskKind,
							},
						},
					},
					Params: []v1beta1.ParamSpec{
						{
							Name: "foo",
							Type: v1beta1.ParamTypeString,
						},
						{
							Name: "bar",
							Type: v1beta1.ParamTypeArray,
						},
					},
				},
			},
		},
		{
			in: &v1beta1.TaskRunSpec{
				TaskSpec: &v1beta1.TaskSpec{},
				Params: []v1beta1.Param{
					{
						Name: "string-param",
						Value: v1beta1.ArrayOrString{
							StringVal: "a",
						},
					},
					{
						Name: "array-param",
						Value: v1beta1.ArrayOrString{
							ArrayVal: []string{"b"},
						},
					},
				},
			},
			want: &v1beta1.TaskRunSpec{
				TaskSpec: &v1beta1.TaskSpec{
					Params: []v1beta1.ParamSpec{
						{
							Name: "string-param",
							Type: v1beta1.ParamTypeString,
						},
						{
							Name: "array-param",
							Type: v1beta1.ParamTypeArray,
						},
					},
				},
				Params: []v1beta1.Param{
					{
						Name: "string-param",
						Value: v1beta1.ArrayOrString{
							StringVal: "a",
						},
					},
					{
						Name: "array-param",
						Value: v1beta1.ArrayOrString{
							ArrayVal: []string{"b"},
						},
					},
				},
				ServiceAccountName: config.DefaultServiceAccountValue,
				Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			},
		},
	} {
		t.Run(fmt.Sprintf("%T", tc.in), func(t *testing.T) {
			tc.in.SetDefaults(ctx)

			if d := cmp.Diff(tc.want, tc.in, cmpOptions...); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

// Pipelines have different behavior than PipelineRuns/TaskRuns - break this into its own test.
func TestImplicitParams_Pipeline(t *testing.T) {
	pipeline := &v1beta1.PipelineSpec{
		Params: []v1beta1.ParamSpec{
			{
				Name: "foo",
				Type: v1beta1.ParamTypeString,
			},
			{
				Name: "bar",
				Type: v1beta1.ParamTypeArray,
			},
		},
		Tasks: []v1beta1.PipelineTask{
			{
				TaskSpec: &v1beta1.EmbeddedTask{
					TaskSpec: v1beta1.TaskSpec{},
				},
			},
			{
				TaskRef: &v1beta1.TaskRef{
					Name: "baz",
				},
			},
		},
		Finally: []v1beta1.PipelineTask{
			{
				TaskSpec: &v1beta1.EmbeddedTask{
					TaskSpec: v1beta1.TaskSpec{},
				},
			},
			{
				TaskRef: &v1beta1.TaskRef{
					Name: "baz",
				},
			},
		},
	}

	ctx := context.Background()

	cfg := config.FromContextOrDefaults(ctx)
	cfg.FeatureFlags = &config.FeatureFlags{EnableAPIFields: "alpha"}
	config.ToContext(ctx, cfg)

	p := pipeline.DeepCopy()
	p.SetDefaults(ctx)

	want := &v1beta1.PipelineSpec{
		Params: []v1beta1.ParamSpec{
			{
				Name: "foo",
				Type: v1beta1.ParamTypeString,
			},
			{
				Name: "bar",
				Type: v1beta1.ParamTypeArray,
			},
		},
		Tasks: []v1beta1.PipelineTask{
			{
				TaskSpec: &v1beta1.EmbeddedTask{
					TaskSpec: v1beta1.TaskSpec{},
				},
			},
			{
				TaskRef: &v1beta1.TaskRef{
					Name: "baz",
					Kind: v1beta1.NamespacedTaskKind,
				},
			},
		},
		Finally: []v1beta1.PipelineTask{
			{
				TaskSpec: &v1beta1.EmbeddedTask{
					TaskSpec: v1beta1.TaskSpec{},
				},
			},
			{
				TaskRef: &v1beta1.TaskRef{
					Name: "baz",
					Kind: v1beta1.NamespacedTaskKind,
				},
			},
		},
	}

	if d := cmp.Diff(want, p, cmpOptions...); d != "" {
		t.Error(diff.PrintWantGot(d))
	}
}
