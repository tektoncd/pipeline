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

package defaultresourcerequirements

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewTransformer(t *testing.T) {
	testPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "custom-ns"},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{Name: "place-scripts"},
				{Name: "prepare"},
				{Name: "working-dir-initializer"},
				{Name: "test-01"},
				{Name: "foo"},
			},
			Containers: []corev1.Container{
				{Name: "scripts-01"},
				{Name: "scripts-02"},
				{Name: "sidecar-scripts-01"},
				{Name: "sidecar-scripts-02"},
				{Name: "test-01"},
				{Name: "foo"},
			},
		},
	}

	tcs := []struct {
		name                 string
		targetPod            *corev1.Pod
		resourceRequirements map[string]corev1.ResourceRequirements
		getExpectedPod       func() *corev1.Pod
	}{
		// verifies with no resource requirements data from a config map
		{
			name:                 "test-with-no-data",
			targetPod:            testPod.DeepCopy(),
			resourceRequirements: map[string]corev1.ResourceRequirements{},
			getExpectedPod: func() *corev1.Pod {
				return testPod.DeepCopy()
			},
		},

		// verifies with empty resource requirements data from a config map
		{
			name:      "test-with-empty-resource-requirements",
			targetPod: testPod.DeepCopy(),
			resourceRequirements: map[string]corev1.ResourceRequirements{
				"default":        {},
				"place-scripts":  {},
				"prefix-scripts": {},
			},
			getExpectedPod: func() *corev1.Pod {
				return testPod.DeepCopy()
			},
		},

		// verifies only with 'default' resource requirements data from a config map
		{
			name:      "test-with-default-set",
			targetPod: testPod.DeepCopy(),
			resourceRequirements: map[string]corev1.ResourceRequirements{
				"default": {
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
			getExpectedPod: func() *corev1.Pod {
				expectedPod := testPod.DeepCopy()
				defaultResource := corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				}
				expectedPod.Spec = corev1.PodSpec{
					InitContainers: []corev1.Container{
						{Name: "place-scripts", Resources: defaultResource},
						{Name: "prepare", Resources: defaultResource},
						{Name: "working-dir-initializer", Resources: defaultResource},
						{Name: "test-01", Resources: defaultResource},
						{Name: "foo", Resources: defaultResource},
					},
					Containers: []corev1.Container{
						{Name: "scripts-01", Resources: defaultResource},
						{Name: "scripts-02", Resources: defaultResource},
						{Name: "sidecar-scripts-01", Resources: defaultResource},
						{Name: "sidecar-scripts-02", Resources: defaultResource},
						{Name: "test-01", Resources: defaultResource},
						{Name: "foo", Resources: defaultResource},
					},
				}
				return expectedPod
			},
		},

		// verifies only with 'place-scripts' resource requirements data from a config map
		{
			name:      "test-with-place-scripts-set",
			targetPod: testPod.DeepCopy(),
			resourceRequirements: map[string]corev1.ResourceRequirements{
				"place-scripts": {
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("128Mi"),
						corev1.ResourceCPU:    resource.MustParse("200m"),
					},
				},
			},
			getExpectedPod: func() *corev1.Pod {
				expectedPod := testPod.DeepCopy()
				expectedPod.Spec.InitContainers = []corev1.Container{
					{
						Name: "place-scripts",
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("128Mi"),
								corev1.ResourceCPU:    resource.MustParse("200m"),
							},
						},
					},
					{Name: "prepare"},
					{Name: "working-dir-initializer"},
					{Name: "test-01"},
					{Name: "foo"},
				}
				return expectedPod
			},
		},

		// verifies only with 'prefix-scripts' resource requirements data from a config map
		{
			name:      "test-with-prefix-scripts-set",
			targetPod: testPod.DeepCopy(),
			resourceRequirements: map[string]corev1.ResourceRequirements{
				"prefix-scripts": {
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("128Mi"),
						corev1.ResourceCPU:    resource.MustParse("200m"),
					},
				},
			},
			getExpectedPod: func() *corev1.Pod {
				expectedPod := testPod.DeepCopy()
				prefixScripts := corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("128Mi"),
						corev1.ResourceCPU:    resource.MustParse("200m"),
					},
				}
				expectedPod.Spec.Containers = []corev1.Container{
					{Name: "scripts-01", Resources: prefixScripts},
					{Name: "scripts-02", Resources: prefixScripts},
					{Name: "sidecar-scripts-01"},
					{Name: "sidecar-scripts-02"},
					{Name: "test-01"},
					{Name: "foo"},
				}
				return expectedPod
			},
		},

		// verifies with 'working-dir-initializer', 'prefix-sidecar-scripts', and 'default' resource requirements data from a config map
		{
			name:      "test-with_name_prefix_and_default-set",
			targetPod: testPod.DeepCopy(),
			resourceRequirements: map[string]corev1.ResourceRequirements{
				"working-dir-initializer": {
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("400m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("256Mi"),
						corev1.ResourceCPU:    resource.MustParse("250m"),
					},
				},
				"prefix-sidecar-scripts": {
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("512Mi"),
						corev1.ResourceCPU:    resource.MustParse("500m"),
					},
				},
				"default": {
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
				"prefix-test": {
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("32Mi"),
					},
				},
				"foo": {
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
				},
			},
			getExpectedPod: func() *corev1.Pod {
				expectedPod := testPod.DeepCopy()
				workDirResourceReqs := corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("400m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("256Mi"),
						corev1.ResourceCPU:    resource.MustParse("250m"),
					},
				}
				sideCarResourceReqs := corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("512Mi"),
						corev1.ResourceCPU:    resource.MustParse("500m"),
					},
				}
				defaultResourceReqs := corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				}

				testResourceReqs := corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("32Mi"),
					},
				}
				fooResourceReqs := corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
				}

				expectedPod.Spec = corev1.PodSpec{
					InitContainers: []corev1.Container{
						{Name: "place-scripts", Resources: defaultResourceReqs},
						{Name: "prepare", Resources: defaultResourceReqs},
						{Name: "working-dir-initializer", Resources: workDirResourceReqs},
						{Name: "test-01", Resources: testResourceReqs},
						{Name: "foo", Resources: fooResourceReqs},
					},
					Containers: []corev1.Container{
						{Name: "scripts-01", Resources: defaultResourceReqs},
						{Name: "scripts-02", Resources: defaultResourceReqs},
						{Name: "sidecar-scripts-01", Resources: sideCarResourceReqs},
						{Name: "sidecar-scripts-02", Resources: sideCarResourceReqs},
						{Name: "test-01", Resources: testResourceReqs},
						{Name: "foo", Resources: fooResourceReqs},
					},
				}
				return expectedPod
			},
		},

		// verifies with existing data
		{
			name: "test-with-existing-data",
			targetPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "custom-ns"},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{Name: "place-scripts"},
						{Name: "prepare", Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
						}},
						{Name: "working-dir-initializer"},
					},
					Containers: []corev1.Container{
						{Name: "scripts-01"},
						{Name: "scripts-02"},
						{Name: "sidecar-scripts-01"},
						{Name: "sidecar-scripts-02"},
					},
				},
			},
			resourceRequirements: map[string]corev1.ResourceRequirements{
				"prepare": {
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
				},
			},
			getExpectedPod: func() *corev1.Pod {
				expectedPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "custom-ns"},
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{
							{Name: "place-scripts"},
							{Name: "prepare", Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							}},
							{Name: "working-dir-initializer"},
						},
						Containers: []corev1.Container{
							{Name: "scripts-01"},
							{Name: "scripts-02"},
							{Name: "sidecar-scripts-01"},
							{Name: "sidecar-scripts-02"},
						},
					},
				}
				return expectedPod
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			// add default container resource requirements on the context
			ctx = config.ToContext(ctx, &config.Config{
				Defaults: &config.Defaults{
					DefaultContainerResourceRequirements: tc.resourceRequirements,
				},
			})

			// get the transformer and call the transformer
			transformer := NewTransformer(ctx)
			transformedPod, err := transformer(tc.targetPod)
			if err != nil {
				t.Errorf("unexpected error %s", err)
			}

			expectedPod := tc.getExpectedPod()
			if d := cmp.Diff(expectedPod, transformedPod); d != "" {
				t.Errorf("Diff %s", diff.PrintWantGot(d))
			}
		})
	}
}
