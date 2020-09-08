/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"knative.dev/pkg/kmeta"
)

// Callback is a function that is passed to an informer's event handler.
type Callback func(interface{})

// EnsureTypeMeta augments the passed-in callback, ensuring that all objects that pass
// through this callback have their TypeMeta set according to the provided GVK.
func EnsureTypeMeta(f Callback, gvk schema.GroupVersionKind) Callback {
	apiVersion, kind := gvk.ToAPIVersionAndKind()

	return func(untyped interface{}) {
		typed, err := kmeta.DeletionHandlingAccessor(untyped)
		if err != nil {
			// TODO: We should consider logging here.
			return
		}

		accessor, err := meta.TypeAccessor(typed)
		if err != nil {
			return
		}

		// If TypeMeta is already what we want, exit early.
		if accessor.GetAPIVersion() == apiVersion && accessor.GetKind() == kind {
			f(typed)
			return
		}

		// We need to populate TypeMeta, but cannot trample the
		// informer's copy.
		copy := typed.DeepCopyObject()

		accessor, err = meta.TypeAccessor(copy)
		if err != nil {
			return
		}
		accessor.SetAPIVersion(apiVersion)
		accessor.SetKind(kind)

		// Pass in the mutated copy (accessor is not just a type cast)
		f(copy)
	}
}
