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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	listersalpha "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FilterRunRef returns a filter that can be passed to a Run Informer, which
// filters out Runs for apiVersion and kinds that a controller doesn't care
// about.
//
// For example, a controller impl that wants to be notified of updates to Runs
// which reference a Task with apiVersion "example.dev/v0" and kind "Example":
//
//     runinformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
//       FilterFunc: FilterRunRef("example.dev/v0", "Example"),
//       Handler:    controller.HandleAll(impl.Enqueue),
//     })
func FilterRunRef(apiVersion, kind string) func(interface{}) bool {
	return func(obj interface{}) bool {
		r, ok := obj.(*v1alpha1.Run)
		if !ok {
			// Somehow got informed of a non-Run object.
			// Ignore.
			return false
		}
		if r == nil || r.Spec.Ref == nil {
			// These are invalid, but just in case they get
			// created somehow, don't panic.
			return false
		}

		return r.Spec.Ref.APIVersion == apiVersion && r.Spec.Ref.Kind == v1alpha1.TaskKind(kind)
	}
}

// FilterOwnerRunRef returns a filter that can be passed to an Informer for any runtime object, which
// filters out objects that aren't controlled by a Run that references a particular apiVersion and kind.
//
// For example, a controller impl that wants to be notified of updates to TaskRuns that are controlled by
// a Run which references a custom task with apiVersion "example.dev/v0" and kind "Example":
//
//     taskruninformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
//       FilterFunc: FilterOwnerRunRef("example.dev/v0", "Example"),
//       Handler:    controller.HandleAll(impl.Enqueue),
//     })
func FilterOwnerRunRef(runLister listersalpha.RunLister, apiVersion, kind string) func(interface{}) bool {
	return func(obj interface{}) bool {
		object, ok := obj.(metav1.Object)
		if !ok {
			return false
		}
		owner := metav1.GetControllerOf(object)
		if owner == nil {
			return false
		}
		if owner.APIVersion != v1alpha1.SchemeGroupVersion.String() || owner.Kind != pipeline.RunControllerName {
			// Not owned by a Run
			return false
		}
		run, err := runLister.Runs(object.GetNamespace()).Get(owner.Name)
		if err != nil {
			return false
		}
		if run.Spec.Ref == nil {
			// These are invalid, but just in case they get created somehow, don't panic.
			return false
		}
		return run.Spec.Ref.APIVersion == apiVersion && run.Spec.Ref.Kind == v1alpha1.TaskKind(kind)
	}
}
