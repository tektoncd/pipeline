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

package v1beta1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestMergeStepsWithStepTemplate(t *testing.T) {
	resourceQuantityCmp := cmp.Comparer(func(x, y resource.Quantity) bool {
		return x.Cmp(y) == 0
	})

	for _, tc := range []struct {
		name     string
		template *corev1.Container
		steps    []Step
		expected []Step
	}{{
		name:     "nil-template",
		template: nil,
		steps: []Step{{
			Container: corev1.Container{Image: "some-image"},
			OnError:   "foo",
		}},
		expected: []Step{{
			Container: corev1.Container{Image: "some-image"},
			OnError:   "foo",
		}},
	}, {
		name: "not-overlapping",
		template: &corev1.Container{
			Command: []string{"/somecmd"},
		},
		steps: []Step{{
			Container: corev1.Container{Image: "some-image"},
			OnError:   "foo",
		}},
		expected: []Step{{
			Container: corev1.Container{Command: []string{"/somecmd"}, Image: "some-image"},
			OnError:   "foo",
		}},
	}, {
		name: "overwriting-one-field",
		template: &corev1.Container{
			Image:   "some-image",
			Command: []string{"/somecmd"},
		},
		steps: []Step{{Container: corev1.Container{
			Image: "some-other-image",
		}}},
		expected: []Step{{Container: corev1.Container{
			Command: []string{"/somecmd"},
			Image:   "some-other-image",
		}}},
	}, {
		name: "merge-and-overwrite-slice",
		template: &corev1.Container{
			Env: []corev1.EnvVar{{
				Name:  "KEEP_THIS",
				Value: "A_VALUE",
			}, {
				Name:  "SOME_KEY",
				Value: "ORIGINAL_VALUE",
			}},
		},
		steps: []Step{{Container: corev1.Container{
			Env: []corev1.EnvVar{{
				Name:  "NEW_KEY",
				Value: "A_VALUE",
			}, {
				Name:  "SOME_KEY",
				Value: "NEW_VALUE",
			}},
		}}},
		expected: []Step{{Container: corev1.Container{
			Env: []corev1.EnvVar{{
				Name:  "NEW_KEY",
				Value: "A_VALUE",
			}, {
				Name:  "KEEP_THIS",
				Value: "A_VALUE",
			}, {
				Name:  "SOME_KEY",
				Value: "NEW_VALUE",
			}},
		}}},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := MergeStepsWithStepTemplate(tc.template, tc.steps)
			if err != nil {
				t.Errorf("expected no error. Got error %v", err)
			}

			if d := cmp.Diff(tc.expected, result, resourceQuantityCmp); d != "" {
				t.Errorf("merged steps don't match, diff: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestMergeStepOverrides(t *testing.T) {
	tcs := []struct {
		name          string
		steps         []Step
		stepOverrides []TaskRunStepOverride
		want          []Step
	}{{
		name: "no overrides",
		steps: []Step{{
			Container: corev1.Container{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
				},
			},
		}},
		want: []Step{{
			Container: corev1.Container{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
				},
			},
		}},
	}, {
		name: "not all steps overridden",
		steps: []Step{{
			Container: corev1.Container{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
				},
			},
		}, {
			Container: corev1.Container{
				Name: "bar",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
				},
			},
		}},
		stepOverrides: []TaskRunStepOverride{{
			Name: "bar",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
			},
		}},
		want: []Step{{
			Container: corev1.Container{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
				},
			},
		}, {
			Container: corev1.Container{
				Name: "bar",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
				},
			},
		}},
	}, {
		name: "override memory but not CPU",
		steps: []Step{{
			Container: corev1.Container{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("1Gi"),
						corev1.ResourceCPU:    resource.MustParse("100m"),
					},
				},
			},
		}},
		stepOverrides: []TaskRunStepOverride{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
			},
		}},
		want: []Step{{
			Container: corev1.Container{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					},
				},
			},
		}},
	}, {
		name: "override request but not limit",
		steps: []Step{{
			Container: corev1.Container{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
					Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
				},
			},
		}},
		stepOverrides: []TaskRunStepOverride{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1.5Gi")},
			},
		}},
		want: []Step{{
			Container: corev1.Container{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1.5Gi")},
					Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
				},
			},
		}},
	}, {
		name: "override request and limit",
		steps: []Step{{
			Container: corev1.Container{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
					Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
				},
			},
		}},
		stepOverrides: []TaskRunStepOverride{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1.5Gi")},
				Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("3Gi")},
			},
		}},
		want: []Step{{
			Container: corev1.Container{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1.5Gi")},
					Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("3Gi")},
				},
			},
		}},
	}, {
		// We don't make any effort to reject overrides that would result in invalid pods;
		// instead, we let k8s reject the resulting pod.
		name: "new request > old limit",
		steps: []Step{{
			Container: corev1.Container{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
					Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
				},
			},
		}},
		stepOverrides: []TaskRunStepOverride{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("3Gi")},
			},
		}},
		want: []Step{{
			Container: corev1.Container{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("3Gi")},
					Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
				},
			},
		}},
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			steps, err := MergeStepsWithOverrides(tc.steps, tc.stepOverrides)
			if err != nil {
				t.Errorf("unexpected error merging steps with overrides: %s", err)
			}
			if d := cmp.Diff(tc.want, steps); d != "" {
				t.Errorf("merged steps don't match, diff: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestMergeSidecarOverrides(t *testing.T) {
	tcs := []struct {
		name             string
		sidecars         []Sidecar
		sidecarOverrides []TaskRunSidecarOverride
		want             []Sidecar
	}{{
		name: "no overrides",
		sidecars: []Sidecar{{
			Container: corev1.Container{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
				},
			},
		}},
		want: []Sidecar{{
			Container: corev1.Container{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
				},
			},
		}},
	}, {
		name: "not all sidecars overridden",
		sidecars: []Sidecar{{
			Container: corev1.Container{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
				},
			},
		}, {
			Container: corev1.Container{
				Name: "bar",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
				},
			},
		}},
		sidecarOverrides: []TaskRunSidecarOverride{{
			Name: "bar",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
			},
		}},
		want: []Sidecar{{
			Container: corev1.Container{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
				},
			},
		}, {
			Container: corev1.Container{
				Name: "bar",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
				},
			},
		}},
	}, {
		name: "override memory but not CPU",
		sidecars: []Sidecar{{
			Container: corev1.Container{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("1Gi"),
						corev1.ResourceCPU:    resource.MustParse("100m"),
					},
				},
			},
		}},
		sidecarOverrides: []TaskRunSidecarOverride{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
			},
		}},
		want: []Sidecar{{
			Container: corev1.Container{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					},
				},
			},
		}},
	}, {
		name: "override request but not limit",
		sidecars: []Sidecar{{
			Container: corev1.Container{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
					Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
				},
			},
		}},
		sidecarOverrides: []TaskRunSidecarOverride{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1.5Gi")},
			},
		}},
		want: []Sidecar{{
			Container: corev1.Container{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1.5Gi")},
					Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
				},
			},
		}},
	}, {
		name: "override request and limit",
		sidecars: []Sidecar{{
			Container: corev1.Container{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
					Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
				},
			},
		}},
		sidecarOverrides: []TaskRunSidecarOverride{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1.5Gi")},
				Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("3Gi")},
			},
		}},
		want: []Sidecar{{
			Container: corev1.Container{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1.5Gi")},
					Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("3Gi")},
				},
			},
		}},
	}, {
		// We don't make any effort to reject overrides that would result in invalid pods;
		// instead, we let k8s reject the resulting pod.
		name: "new request > old limit",
		sidecars: []Sidecar{{
			Container: corev1.Container{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
					Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
				},
			},
		}},
		sidecarOverrides: []TaskRunSidecarOverride{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("3Gi")},
			},
		}},
		want: []Sidecar{{
			Container: corev1.Container{
				Name: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("3Gi")},
					Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
				},
			},
		}},
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			sidecars, err := MergeSidecarsWithOverrides(tc.sidecars, tc.sidecarOverrides)
			if err != nil {
				t.Errorf("unexpected error merging sidecars with overrides: %s", err)
			}
			if d := cmp.Diff(tc.want, sidecars); d != "" {
				t.Errorf("merged sidecars don't match, diff: %s", diff.PrintWantGot(d))
			}
		})
	}
}
