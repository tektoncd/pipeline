/*
Copyright 2025 The Tekton Authors

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

package reconciler

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StripManagedFields removes metadata.managedFields from objects on cache insertion.
// ManagedFields can be 30-70% of an object's serialized size but are never read
// by the controller, so stripping them reduces informer cache memory and DeepCopy cost.
func StripManagedFields(obj interface{}) (interface{}, error) {
	if accessor, ok := obj.(metav1.ObjectMetaAccessor); ok {
		accessor.GetObjectMeta().SetManagedFields(nil)
	}
	return obj, nil
}
