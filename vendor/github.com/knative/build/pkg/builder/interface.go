/*
Copyright 2018 The Knative Authors

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

package builder

import (
	corev1 "k8s.io/api/core/v1"

	v1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
)

// Operation defines the interface for interacting with an Operation of a particular BuildProvider.
type Operation interface {
	// Name provides the unique name for this operation, see OperationFromStatus.
	Name() string

	// Checkpoint augments the provided BuildStatus with sufficient state to be restored
	// by OperationFromStatus on an appropriate BuildProvider.
	Checkpoint(*v1alpha1.BuildStatus) error

	// Wait blocks until the Operation completes, returning either a status for the build or an error.
	// TODO(mattmoor): This probably shouldn't be BuildStatus, but some sort of smaller-scope thing.
	Wait() (*v1alpha1.BuildStatus, error)
}

// Build defines the interface for launching a build and getting an Operation by which to track it to completion.
type Build interface {
	// Execute launches this particular build and returns an Operation to track it's progress.
	Execute() (Operation, error)
}

// Interface defines the set of operations that all builders must implement.
type Interface interface {
	// Which builder are we?
	Builder() v1alpha1.BuildProvider

	// Validate a Build for this flavor of builder.
	Validate(*v1alpha1.Build, *v1alpha1.BuildTemplate) error

	// Construct a Build for this flavor of builder from our CRD specification.
	BuildFromSpec(*v1alpha1.Build) (Build, error)

	// Construct an Operation for this flavor of builder from a BuildStatus.
	OperationFromStatus(*v1alpha1.BuildStatus) (Operation, error)
}

func IsDone(status *v1alpha1.BuildStatus) bool {
	if status == nil || len(status.Conditions) == 0 {
		return false
	}
	for _, cond := range status.Conditions {
		if cond.Type == v1alpha1.BuildSucceeded {
			return cond.Status != corev1.ConditionUnknown
		}
	}
	return false
}

func ErrorMessage(status *v1alpha1.BuildStatus) (string, bool) {
	if status == nil || len(status.Conditions) == 0 {
		return "", false
	}
	for _, cond := range status.Conditions {
		if cond.Type == v1alpha1.BuildSucceeded && cond.Status == corev1.ConditionFalse {
			return cond.Message, true
		}
	}
	return "", false
}
