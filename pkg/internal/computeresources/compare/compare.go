/*
Copyright 2022 The Tekton Authors

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
package compare

import (
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// IsZero returns true if the resource quantity has a zero value
func IsZero(q resource.Quantity) bool {
	return (&q).IsZero()
}

// MaxRequest returns the largest resource request
// A zero request is considered the smallest request
func MaxRequest(quantities ...resource.Quantity) resource.Quantity {
	max := resource.Quantity{}
	for _, q := range quantities {
		if q.Cmp(max) > 0 {
			max = q
		}
	}
	return max
}

// MinLimit returns the smallest resource limit
// A zero limit is considered higher than any other resource limit.
func MinLimit(quantities ...resource.Quantity) resource.Quantity {
	min := resource.Quantity{}
	for _, q := range quantities {
		if min.IsZero() {
			min = q
		} else if q.Cmp(min) < 0 {
			min = q
		}
	}
	return min
}

// ResourceQuantityCmp allows resource quantities to be compared in tests
var ResourceQuantityCmp = cmp.Comparer(func(x, y resource.Quantity) bool {
	return x.Cmp(y) == 0
})

func equateAlways(_, _ interface{}) bool { return true }

// EquateEmptyResourceList returns a comparison option that will equate resource lists
// if neither contains non-empty resource quantities.
func EquateEmptyResourceList() cmp.Option {
	return cmp.FilterValues(func(x, y corev1.ResourceList) bool { return IsEmpty(x) && IsEmpty(y) }, cmp.Comparer(equateAlways))
}

// IsEmpty returns false if the ResourceList contains non-empty resource quantities.
func IsEmpty(x corev1.ResourceList) bool {
	if len(x) == 0 {
		return true
	}
	for _, q := range x {
		if !q.IsZero() {
			return false
		}
	}
	return true
}
