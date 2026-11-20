/*
Copyright 2024 The Tekton Authors

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

package pod

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestMergeByName(t *testing.T) {
	type testCase struct {
		name      string
		base      []interface{}
		overrides []interface{}
		expected  []interface{}
	}

	testCases := []testCase{
		{
			name:      "empty overrides",
			base:      []interface{}{corev1.EnvVar{Name: "foo", Value: "bar"}},
			overrides: []interface{}{},
			expected:  []interface{}{corev1.EnvVar{Name: "foo", Value: "bar"}},
		},
		{
			name:      "empty base",
			base:      []interface{}{},
			overrides: []interface{}{corev1.EnvVar{Name: "foo", Value: "bar"}},
			expected:  []interface{}{corev1.EnvVar{Name: "foo", Value: "bar"}},
		},
		{
			name:      "same name",
			base:      []interface{}{corev1.EnvVar{Name: "foo", Value: "bar"}},
			overrides: []interface{}{corev1.EnvVar{Name: "foo", Value: "baz"}},
			expected:  []interface{}{corev1.EnvVar{Name: "foo", Value: "baz"}},
		},
		{
			name:      "different name",
			base:      []interface{}{corev1.EnvVar{Name: "foo", Value: "bar"}},
			overrides: []interface{}{corev1.EnvVar{Name: "bar", Value: "baz"}},
			expected:  []interface{}{corev1.EnvVar{Name: "bar", Value: "baz"}, corev1.EnvVar{Name: "foo", Value: "bar"}},
		},
		{
			name:      "different volume name",
			base:      []interface{}{corev1.Volume{Name: "foo", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}},
			overrides: []interface{}{corev1.Volume{Name: "bar", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}},
			expected: []interface{}{
				corev1.Volume{Name: "bar", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
				corev1.Volume{Name: "foo", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
			},
		},
		{
			name:      "unsupported type",
			base:      []interface{}{corev1.EnvVar{Name: "foo", Value: "bar"}},
			overrides: []interface{}{42},
			expected:  []interface{}{corev1.EnvVar{Name: "foo", Value: "bar"}},
		},
		{
			name:      "empty name",
			base:      []interface{}{corev1.EnvVar{Name: "", Value: "bar"}},
			overrides: []interface{}{corev1.EnvVar{Name: "", Value: "bar"}},
			expected:  []interface{}{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := mergeByName(tc.base, tc.overrides)
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("mergeByName(%v, %v) = %v, want %v", tc.base, tc.overrides, result, tc.expected)
			}
		})
	}
}

func TestMergePodTemplateWithDefault(t *testing.T) {
	type testCase struct {
		name       string
		tpl        *PodTemplate
		defaultTpl *PodTemplate
		expected   *PodTemplate
	}

	testCases := []testCase{
		{
			name: "defaultTpl is nil",
			tpl: &PodTemplate{
				NodeSelector: map[string]string{"foo": "bar"},
			},
			defaultTpl: nil,
			expected: &PodTemplate{
				NodeSelector: map[string]string{"foo": "bar"},
			},
		},
		{
			name: "tpl is nil",
			tpl:  nil,
			defaultTpl: &PodTemplate{
				NodeSelector: map[string]string{"foo": "bar"},
			},
			expected: &PodTemplate{
				NodeSelector: map[string]string{"foo": "bar"},
			},
		},
		{
			name: "override default env",
			tpl: &PodTemplate{
				Env: []corev1.EnvVar{{Name: "foo", Value: "bar"}},
			},
			defaultTpl: &PodTemplate{
				Env: []corev1.EnvVar{{Name: "foo", Value: "baz"}},
			},
			expected: &PodTemplate{
				Env: []corev1.EnvVar{{Name: "foo", Value: "bar"}},
			},
		},
		{
			name: "merge envs",
			tpl: &PodTemplate{
				Env: []corev1.EnvVar{{Name: "foo", Value: "bar"}},
			},
			defaultTpl: &PodTemplate{
				Env: []corev1.EnvVar{{Name: "bar", Value: "bar"}},
			},
			expected: &PodTemplate{
				Env: []corev1.EnvVar{
					{Name: "foo", Value: "bar"},
					{Name: "bar", Value: "bar"},
				},
			},
		},
		{
			name: "update host network",
			tpl: &PodTemplate{
				HostNetwork: false,
			},
			defaultTpl: &PodTemplate{
				HostNetwork: true,
			},
			expected: &PodTemplate{
				HostNetwork: true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := MergePodTemplateWithDefault(tc.tpl, tc.defaultTpl)
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("mergePodTemplateWithDefault%v, %v) = %v, want %v", tc.tpl, tc.defaultTpl, result, tc.expected)
			}
		})
	}
}

func TestMergeAAPodTemplateWithDefault(t *testing.T) {
	priority1 := "low-priority"
	priority2 := "high-priority"
	type testCase struct {
		name       string
		tpl        *AAPodTemplate
		defaultTpl *AAPodTemplate
		expected   *AAPodTemplate
	}

	specResources := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("200m"),
			corev1.ResourceMemory: resource.MustParse("512Mi"),
		},
	}
	defaultResources := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("50m"),
			corev1.ResourceMemory: resource.MustParse("100Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("50m"),
			corev1.ResourceMemory: resource.MustParse("100Mi"),
		},
	}

	testCases := []testCase{
		{
			name: "defaultTpl is nil",
			tpl: &AAPodTemplate{
				NodeSelector: map[string]string{"foo": "bar"},
			},
			defaultTpl: nil,
			expected: &AAPodTemplate{
				NodeSelector: map[string]string{"foo": "bar"},
			},
		},
		{
			name: "tpl is nil",
			tpl:  nil,
			defaultTpl: &AAPodTemplate{
				NodeSelector: map[string]string{"foo": "bar"},
			},
			expected: &AAPodTemplate{
				NodeSelector: map[string]string{"foo": "bar"},
			},
		},
		{
			name: "override default priorityClassName",
			tpl: &AAPodTemplate{
				PriorityClassName: &priority2,
			},
			defaultTpl: &AAPodTemplate{
				PriorityClassName: &priority1,
			},
			expected: &AAPodTemplate{
				PriorityClassName: &priority2,
			},
		},
		{
			name: "resources from spec override default",
			tpl: &AAPodTemplate{
				Resources: specResources,
			},
			defaultTpl: &AAPodTemplate{
				Resources: defaultResources,
			},
			expected: &AAPodTemplate{
				Resources: specResources,
			},
		},
		{
			name: "resources from default when spec is nil",
			tpl: &AAPodTemplate{
				NodeSelector: map[string]string{"foo": "bar"},
			},
			defaultTpl: &AAPodTemplate{
				Resources: defaultResources,
			},
			expected: &AAPodTemplate{
				NodeSelector: map[string]string{"foo": "bar"},
				Resources:    defaultResources,
			},
		},
		{
			name: "no resources when neither set",
			tpl: &AAPodTemplate{
				NodeSelector: map[string]string{"foo": "bar"},
			},
			defaultTpl: &AAPodTemplate{
				NodeSelector: map[string]string{"baz": "qux"},
			},
			expected: &AAPodTemplate{
				NodeSelector: map[string]string{"foo": "bar"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := MergeAAPodTemplateWithDefault(tc.tpl, tc.defaultTpl)
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("mergeAAPodTemplateWithDefault(%v, %v) = %v, want %v", tc.tpl, tc.defaultTpl, result, tc.expected)
			}
		})
	}
}

func TestToAffinityAssistantTemplate_PropagatesResources(t *testing.T) {
	resources := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("200m"),
			corev1.ResourceMemory: resource.MustParse("512Mi"),
		},
	}

	tpl := &Template{
		NodeSelector: map[string]string{"foo": "bar"},
		Resources:    resources,
	}

	aaTpl := tpl.ToAffinityAssistantTemplate()
	if aaTpl.Resources == nil {
		t.Fatal("expected Resources to be propagated, got nil")
	}
	if !reflect.DeepEqual(aaTpl.Resources, resources) {
		t.Errorf("ToAffinityAssistantTemplate() Resources = %v, want %v", aaTpl.Resources, resources)
	}
}

func TestToAffinityAssistantTemplate_NilResources(t *testing.T) {
	tpl := &Template{
		NodeSelector: map[string]string{"foo": "bar"},
	}

	aaTpl := tpl.ToAffinityAssistantTemplate()
	if aaTpl.Resources != nil {
		t.Errorf("expected Resources to be nil when not set on Template, got %v", aaTpl.Resources)
	}
}

func TestValidateResources(t *testing.T) {
	tests := []struct {
		name      string
		tpl       *AffinityAssistantTemplate
		expectErr bool
	}{
		{
			name:      "nil template",
			tpl:       nil,
			expectErr: false,
		},
		{
			name:      "nil resources",
			tpl:       &AffinityAssistantTemplate{},
			expectErr: false,
		},
		{
			name: "valid: requests equal limits",
			tpl: &AffinityAssistantTemplate{
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "valid: requests less than limits",
			tpl: &AffinityAssistantTemplate{
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "invalid: CPU request exceeds limit",
			tpl: &AffinityAssistantTemplate{
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("500m"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("100m"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "invalid: memory request exceeds limit",
			tpl: &AffinityAssistantTemplate{
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "valid: requests only, no limits",
			tpl: &AffinityAssistantTemplate{
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("100m"),
					},
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.tpl.ValidateResources()
			if tt.expectErr && len(errs) == 0 {
				t.Errorf("expected validation errors but got none")
			}
			if !tt.expectErr && len(errs) > 0 {
				t.Errorf("expected no validation errors but got: %v", errs)
			}
		})
	}
}
