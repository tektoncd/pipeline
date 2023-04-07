/*
Copyright 2020 The Tekton Authors

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

// Package controller provides helper methods for external controllers for
// Custom Task types.
package controller

import (
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	listersbeta "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FilterCustomRunRef returns a filter that can be passed to a CustomRun Informer, which
// filters out CustomRuns for apiVersion and kinds that a controller doesn't care
// about.
//
// For example, a controller impl that wants to be notified of updates to CustomRuns
// which reference a Task with apiVersion "example.dev/v0" and kind "Example":
//
//	customruninformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
//	  FilterFunc: FilterCustomRunRef("example.dev/v0", "Example"),
//	  Handler:    controller.HandleAll(impl.Enqueue),
//	})
func FilterCustomRunRef(apiVersion, kind string) func(interface{}) bool {
	return func(obj interface{}) bool {
		r, ok := obj.(*v1beta1.CustomRun)
		if !ok {
			// Somehow got informed of a non-CustomRun object.
			// Ignore.
			return false
		}
		if r == nil || (r.Spec.CustomRef == nil && r.Spec.CustomSpec == nil) {
			// These are invalid, but just in case they get
			// created somehow, don't panic.
			return false
		}
		result := false
		if r.Spec.CustomRef != nil {
			result = r.Spec.CustomRef.APIVersion == apiVersion && r.Spec.CustomRef.Kind == v1beta1.TaskKind(kind)
		} else if r.Spec.CustomSpec != nil {
			result = r.Spec.CustomSpec.APIVersion == apiVersion && r.Spec.CustomSpec.Kind == kind
		}
		return result
	}
}

// FilterOwnerCustomRunRef returns a filter that can be passed to an Informer for any runtime object, which
// filters out objects that aren't controlled by a CustomRun that references a particular apiVersion and kind.
//
// For example, a controller impl that wants to be notified of updates to TaskRuns that are controlled by
// a CustomRun which references a custom task with apiVersion "example.dev/v0" and kind "Example":
//
//	taskruninformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
//	  FilterFunc: FilterOwnerCustomRunRef("example.dev/v0", "Example"),
//	  Handler:    controller.HandleAll(impl.Enqueue),
//	})
func FilterOwnerCustomRunRef(customRunLister listersbeta.CustomRunLister, apiVersion, kind string) func(interface{}) bool {
	return func(obj interface{}) bool {
		object, ok := obj.(metav1.Object)
		if !ok {
			return false
		}
		owner := metav1.GetControllerOf(object)
		if owner == nil {
			return false
		}
		if owner.APIVersion != v1beta1.SchemeGroupVersion.String() || owner.Kind != pipeline.CustomRunControllerName {
			// Not owned by a CustomRun
			return false
		}
		run, err := customRunLister.CustomRuns(object.GetNamespace()).Get(owner.Name)
		if err != nil {
			return false
		}
		if run.Spec.CustomRef == nil && run.Spec.CustomSpec == nil {
			// These are invalid, but just in case they get created somehow, don't panic.
			return false
		}
		if run.Spec.CustomRef != nil && run.Spec.CustomSpec != nil {
			// These are invalid.
			return false
		}
		result := false
		if run.Spec.CustomRef != nil {
			result = run.Spec.CustomRef.APIVersion == apiVersion && run.Spec.CustomRef.Kind == v1beta1.TaskKind(kind)
		} else if run.Spec.CustomSpec != nil {
			result = run.Spec.CustomSpec.APIVersion == apiVersion && run.Spec.CustomSpec.Kind == kind
		}
		return result
	}
}
