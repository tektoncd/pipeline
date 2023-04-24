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

package tasklevel_test

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/internal/computeresources/tasklevel"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestApplyTaskLevelResourceRequirements(t *testing.T) {
	testcases := []struct {
		desc                     string
		Steps                    []v1.Step
		ComputeResources         corev1.ResourceRequirements
		expectedComputeResources []corev1.ResourceRequirements
	}{{
		desc: "only with requests",
		Steps: []v1.Step{{
			Name:    "1st-step",
			Image:   "image",
			Command: []string{"cmd"},
		}, {
			Name:    "2nd-step",
			Image:   "image",
			Command: []string{"cmd"},
		}},
		ComputeResources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
		},
		expectedComputeResources: []corev1.ResourceRequirements{{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
		}, {
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
		}},
	}, {
		desc: "only with limits",
		Steps: []v1.Step{{
			Name:    "1st-step",
			Image:   "image",
			Command: []string{"cmd"},
		}, {
			Name:    "2nd-step",
			Image:   "image",
			Command: []string{"cmd"},
		}},
		ComputeResources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")},
		},
		expectedComputeResources: []corev1.ResourceRequirements{{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("250m")},
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")},
		}, {
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("250m")},
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")},
		}},
	}, {
		desc: "both with requests and limits",
		Steps: []v1.Step{{
			Name:    "1st-step",
			Image:   "image",
			Command: []string{"cmd"},
		}, {
			Name:    "2nd-step",
			Image:   "image",
			Command: []string{"cmd"},
		}},
		ComputeResources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
		},
		expectedComputeResources: []corev1.ResourceRequirements{{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")},
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
		}, {
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")},
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
		}},
	}, {
		desc: "steps with compute resources are overridden by task-level compute resources",
		Steps: []v1.Step{{
			Name:    "1st-step",
			Image:   "image",
			Command: []string{"cmd"},
			ComputeResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
			},
		}, {
			Name:    "2nd-step",
			Image:   "image",
			Command: []string{"cmd"},
			ComputeResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
			},
		}},
		ComputeResources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
		},
		expectedComputeResources: []corev1.ResourceRequirements{{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")},
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
		}, {
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")},
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
		}},
	}, {
		desc: "steps with partial compute resources are overridden by task-level compute resources",
		Steps: []v1.Step{{
			Name:    "1st-step",
			Image:   "image",
			Command: []string{"cmd"},
			ComputeResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
			},
		}, {
			Name:    "2nd-step",
			Image:   "image",
			Command: []string{"cmd"},
		}},
		ComputeResources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
		},
		expectedComputeResources: []corev1.ResourceRequirements{{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")},
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
		}, {
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")},
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
		}},
	}, {
		desc: "steps with compute resources are preserved, if there are no task-level compute resources",
		Steps: []v1.Step{{
			Name:    "1st-step",
			Image:   "image",
			Command: []string{"cmd"},
			ComputeResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
			},
		}, {
			Name:    "2nd-step",
			Image:   "image",
			Command: []string{"cmd"},
			ComputeResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
			},
		}},
		ComputeResources: corev1.ResourceRequirements{},
		expectedComputeResources: []corev1.ResourceRequirements{{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
		}, {
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")},
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
		}},
	}}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			tasklevel.ApplyTaskLevelComputeResources(tc.Steps, &tc.ComputeResources)

			if err := verifyTaskLevelComputeResources(tc.Steps, tc.expectedComputeResources); err != nil {
				t.Errorf("verifyTaskLevelComputeResources: %v", err)
			}
		})
	}
}

// verifyTaskLevelComputeResources verifies that the given TaskRun's containers have the expected compute resources.
func verifyTaskLevelComputeResources(steps []v1.Step, expectedComputeResources []corev1.ResourceRequirements) error {
	if len(expectedComputeResources) != len(steps) {
		return fmt.Errorf("expected %d compute resource requirements, got %d", len(expectedComputeResources), len(steps))
	}
	for id, step := range steps {
		if d := cmp.Diff(expectedComputeResources[id], step.ComputeResources); d != "" {
			return fmt.Errorf("container \"#%d\" resource requirements don't match %s", id, diff.PrintWantGot(d))
		}
	}
	return nil
}
