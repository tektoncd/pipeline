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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
)

// Operation defines the interface for interacting with an Operation of a particular BuildProvider.
type Operation interface {
	// Name provides the unique name for this operation, see OperationFromStatus.
	Name() string

	// Checkpoint augments the provided BuildStatus with sufficient state to be
	// restored by OperationFromStatus on an appropriate BuildProvider.
	//
	// This takes into account necessary information about the provided Build.
	Checkpoint(*v1alpha1.Build, *v1alpha1.BuildStatus) error

	// Wait blocks until the Operation completes, returning either a status for the build or an error.
	// TODO(mattmoor): This probably shouldn't be BuildStatus, but some sort of smaller-scope thing.
	Wait() (*v1alpha1.BuildStatus, error)

	// Terminate cleans up this particular operation and returns an error if it fails
	Terminate() error
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
	Validate(*v1alpha1.Build) error

	// Construct a Build for this flavor of builder from our CRD specification.
	BuildFromSpec(*v1alpha1.Build) (Build, error)

	// Construct an Operation for this flavor of builder from a BuildStatus.
	OperationFromStatus(*v1alpha1.BuildStatus) (Operation, error)
}

// IsDone returns true if the build's status indicates the build is done.
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

// IsTimeout returns true if the build's execution time is greater than
// specified build spec timeout.
func IsTimeout(status *v1alpha1.BuildStatus, buildTimeout *metav1.Duration) bool {
	var timeout time.Duration
	var defaultTimeout = 10 * time.Minute

	if status == nil {
		return false
	}

	if buildTimeout == nil {
		// Set default timeout to 10 minute if build timeout is not set
		timeout = defaultTimeout
	} else {
		timeout = buildTimeout.Duration
	}

	// If build has not started timeout, startTime should be zero.
	if status.StartTime.Time.IsZero() {
		return false
	}
	return time.Since(status.StartTime.Time).Seconds() > timeout.Seconds()
}

// ErrorMessage returns the error message from the status.
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
