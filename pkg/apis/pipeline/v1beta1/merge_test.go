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

package v1beta1_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
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
		template *v1beta1.StepTemplate
		steps    []v1beta1.Step
		expected []v1beta1.Step
	}{{
		name:     "nil-template",
		template: nil,
		steps: []v1beta1.Step{{
			Image:   "some-image",
			OnError: "foo",
		}},
		expected: []v1beta1.Step{{
			Image:   "some-image",
			OnError: "foo",
		}},
	}, {
		name: "not-overlapping",
		template: &v1beta1.StepTemplate{
			Command: []string{"/somecmd"},
		},
		steps: []v1beta1.Step{{
			Image:   "some-image",
			OnError: "foo",
		}},
		expected: []v1beta1.Step{{
			Command: []string{"/somecmd"}, Image: "some-image",
			OnError: "foo",
		}},
	}, {
		name: "overwriting-one-field",
		template: &v1beta1.StepTemplate{
			Image:   "some-image",
			Command: []string{"/somecmd"},
		},
		steps: []v1beta1.Step{{
			Image: "some-other-image",
		}},
		expected: []v1beta1.Step{{
			Command: []string{"/somecmd"},
			Image:   "some-other-image",
		}},
	}, {
		name: "merge-and-overwrite-slice",
		template: &v1beta1.StepTemplate{
			Env: []corev1.EnvVar{{
				Name:  "KEEP_THIS",
				Value: "A_VALUE",
			}, {
				Name:  "SOME_KEY",
				Value: "ORIGINAL_VALUE",
			}},
		},
		steps: []v1beta1.Step{{
			Env: []corev1.EnvVar{{
				Name:  "NEW_KEY",
				Value: "A_VALUE",
			}, {
				Name:  "SOME_KEY",
				Value: "NEW_VALUE",
			}},
		}},
		expected: []v1beta1.Step{{
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
		}},
	}, {
		name: "workspace-and-output-config",
		template: &v1beta1.StepTemplate{
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "data",
				MountPath: "/workspace/data",
			}},
		},
		steps: []v1beta1.Step{{
			Image:        "some-image",
			StdoutConfig: &v1beta1.StepOutputConfig{Path: "stdout.txt"},
			StderrConfig: &v1beta1.StepOutputConfig{Path: "stderr.txt"},
		}},
		expected: []v1beta1.Step{{
			Image:        "some-image",
			StdoutConfig: &v1beta1.StepOutputConfig{Path: "stdout.txt"},
			StderrConfig: &v1beta1.StepOutputConfig{Path: "stderr.txt"},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "data",
				MountPath: "/workspace/data",
			}},
		}},
	}, {
		name: "merge-env-by-step",
		template: &v1beta1.StepTemplate{
			Env: []corev1.EnvVar{{
				Name:  "KEEP_THIS",
				Value: "A_VALUE",
			}, {
				Name: "SOME_KEY_1",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key:                  "A_KEY",
						LocalObjectReference: corev1.LocalObjectReference{Name: "A_NAME"},
					},
				},
			}, {
				Name:  "SOME_KEY_2",
				Value: "VALUE_2",
			}},
		},
		steps: []v1beta1.Step{{
			Env: []corev1.EnvVar{{
				Name:  "NEW_KEY",
				Value: "A_VALUE",
			}, {
				Name:  "SOME_KEY_1",
				Value: "VALUE_1",
			}, {
				Name: "SOME_KEY_2",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key:                  "A_KEY",
						LocalObjectReference: corev1.LocalObjectReference{Name: "A_NAME"},
					},
				},
			}},
		}},
		expected: []v1beta1.Step{{
			Env: []corev1.EnvVar{{
				Name:  "NEW_KEY",
				Value: "A_VALUE",
			}, {
				Name:  "KEEP_THIS",
				Value: "A_VALUE",
			}, {
				Name:  "SOME_KEY_1",
				Value: "VALUE_1",
			}, {
				Name: "SOME_KEY_2",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key:                  "A_KEY",
						LocalObjectReference: corev1.LocalObjectReference{Name: "A_NAME"},
					},
				},
			}},
		}},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := v1beta1.MergeStepsWithStepTemplate(tc.template, tc.steps)
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
		steps         []v1beta1.Step
		stepOverrides []v1beta1.TaskRunStepOverride
		want          []v1beta1.Step
	}{{
		name: "no overrides",
		steps: []v1beta1.Step{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
			},
		}},
		want: []v1beta1.Step{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
			},
		}},
	}, {
		name: "not all steps overridden",
		steps: []v1beta1.Step{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
			},
		}, {
			Name: "bar",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
			},
		}},
		stepOverrides: []v1beta1.TaskRunStepOverride{{
			Name: "bar",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
			},
		}},
		want: []v1beta1.Step{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
			},
		}, {
			Name: "bar",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
			},
		}},
	}, {
		name: "override memory but not CPU",
		steps: []v1beta1.Step{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("1Gi"),
					corev1.ResourceCPU:    resource.MustParse("100m"),
				},
			},
		}},
		stepOverrides: []v1beta1.TaskRunStepOverride{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
			},
		}},
		want: []v1beta1.Step{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
			},
		}},
	}, {
		name: "override request but not limit",
		steps: []v1beta1.Step{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
				Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
			},
		}},
		stepOverrides: []v1beta1.TaskRunStepOverride{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1.5Gi")},
			},
		}},
		want: []v1beta1.Step{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1.5Gi")},
				Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
			},
		}},
	}, {
		name: "override request and limit",
		steps: []v1beta1.Step{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
				Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
			},
		}},
		stepOverrides: []v1beta1.TaskRunStepOverride{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1.5Gi")},
				Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("3Gi")},
			},
		}},
		want: []v1beta1.Step{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1.5Gi")},
				Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("3Gi")},
			},
		}},
	}, {
		// We don't make any effort to reject overrides that would result in invalid pods;
		// instead, we let k8s reject the resulting pod.
		name: "new request > old limit",
		steps: []v1beta1.Step{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
				Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
			},
		}},
		stepOverrides: []v1beta1.TaskRunStepOverride{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("3Gi")},
			},
		}},
		want: []v1beta1.Step{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("3Gi")},
				Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
			},
		}},
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			steps, err := v1beta1.MergeStepsWithOverrides(tc.steps, tc.stepOverrides)
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
		sidecars         []v1beta1.Sidecar
		sidecarOverrides []v1beta1.TaskRunSidecarOverride
		want             []v1beta1.Sidecar
	}{{
		name: "no overrides",
		sidecars: []v1beta1.Sidecar{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
			},
		}},
		want: []v1beta1.Sidecar{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
			},
		}},
	}, {
		name: "not all sidecars overridden",
		sidecars: []v1beta1.Sidecar{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
			},
		}, {
			Name: "bar",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
			},
		}},
		sidecarOverrides: []v1beta1.TaskRunSidecarOverride{{
			Name: "bar",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
			},
		}},
		want: []v1beta1.Sidecar{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
			},
		}, {
			Name: "bar",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
			},
		}},
	}, {
		name: "override memory but not CPU",
		sidecars: []v1beta1.Sidecar{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("1Gi"),
					corev1.ResourceCPU:    resource.MustParse("100m"),
				},
			},
		}},
		sidecarOverrides: []v1beta1.TaskRunSidecarOverride{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
			},
		}},
		want: []v1beta1.Sidecar{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
			},
		}},
	}, {
		name: "override request but not limit",
		sidecars: []v1beta1.Sidecar{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
				Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
			},
		}},
		sidecarOverrides: []v1beta1.TaskRunSidecarOverride{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1.5Gi")},
			},
		}},
		want: []v1beta1.Sidecar{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1.5Gi")},
				Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
			},
		}},
	}, {
		name: "override request and limit",
		sidecars: []v1beta1.Sidecar{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
				Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
			},
		}},
		sidecarOverrides: []v1beta1.TaskRunSidecarOverride{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1.5Gi")},
				Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("3Gi")},
			},
		}},
		want: []v1beta1.Sidecar{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1.5Gi")},
				Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("3Gi")},
			},
		}},
	}, {
		// We don't make any effort to reject overrides that would result in invalid pods;
		// instead, we let k8s reject the resulting pod.
		name: "new request > old limit",
		sidecars: []v1beta1.Sidecar{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
				Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
			},
		}},
		sidecarOverrides: []v1beta1.TaskRunSidecarOverride{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("3Gi")},
			},
		}},
		want: []v1beta1.Sidecar{{
			Name: "foo",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("3Gi")},
				Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
			},
		}},
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			sidecars, err := v1beta1.MergeSidecarsWithOverrides(tc.sidecars, tc.sidecarOverrides)
			if err != nil {
				t.Errorf("unexpected error merging sidecars with overrides: %s", err)
			}
			if d := cmp.Diff(tc.want, sidecars); d != "" {
				t.Errorf("merged sidecars don't match, diff: %s", diff.PrintWantGot(d))
			}
		})
	}
}
