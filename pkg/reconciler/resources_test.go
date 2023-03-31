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

package reconciler_test

import (
	"testing"
	"time"

	reconciler "github.com/tektoncd/pipeline/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"
)

const (
	// minimumResourceAge is the age at which resources stop being IsYoungResource.
	minimumResourceAge = 5 * time.Second
)

func TestIsYoungResource(t *testing.T) {
	tests := []struct {
		name     string
		resource kmeta.Accessor
		want     bool
	}{{
		name: "young",
		resource: &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: metav1.Time{
					// The age is Min-1 seconds
					Time: time.Now().Add(-minimumResourceAge + 1*time.Second),
				},
			},
		},
		want: true,
	}, {
		name: "old",
		resource: &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: metav1.Time{
					// The age is Min+1 seconds
					Time: time.Now().Add(-minimumResourceAge - 1*time.Second),
				},
			},
		},
		want: false,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := reconciler.IsYoungResource(test.resource)
			if got != test.want {
				t.Errorf("IsYoungResource() = %v, wanted %v", got, test.want)
			}
		})
	}
}
