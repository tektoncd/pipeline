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

// Package nop provides a no-op builder implementation.
package nop

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	buildercommon "github.com/knative/build/pkg/builder"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
)

const operationName = "nop"

var (
	startTime      = metav1.NewTime(time.Unix(0, 0))
	completionTime = metav1.NewTime(time.Unix(30, 0))
)

type operation struct {
	builder *Builder
}

func (nb *operation) Name() string { return operationName }

func (nb *operation) Checkpoint(_ *v1alpha1.Build, status *v1alpha1.BuildStatus) error {
	// Masquerade as the Google builder.
	status.Builder = v1alpha1.GoogleBuildProvider
	if status.Google == nil {
		status.Google = &v1alpha1.GoogleSpec{}
	}
	status.Google.Operation = nb.Name()
	status.CreationTime = startTime
	status.StartTime = startTime
	status.SetCondition(&duckv1alpha1.Condition{
		Type:   v1alpha1.BuildSucceeded,
		Status: corev1.ConditionUnknown,
		Reason: "Building",
	})
	return nil
}

func (nb *operation) Terminate() error {
	return nil
}

func (nb *operation) Wait() (*v1alpha1.BuildStatus, error) {
	bs := &v1alpha1.BuildStatus{
		// Masquerade as the Google builder.
		Builder: v1alpha1.GoogleBuildProvider,
		Google: &v1alpha1.GoogleSpec{
			Operation: nb.Name(),
		},
		CreationTime:   startTime,
		StartTime:      startTime,
		CompletionTime: completionTime,
	}

	if nb.builder.ErrorMessage != "" {
		bs.SetCondition(&duckv1alpha1.Condition{
			Type:    v1alpha1.BuildSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  "NopFailed",
			Message: nb.builder.ErrorMessage,
		})
	} else {
		bs.SetCondition(&duckv1alpha1.Condition{
			Type:   v1alpha1.BuildSucceeded,
			Status: corev1.ConditionTrue,
		})
	}

	return bs, nil
}

type build struct {
	builder *Builder
	err     error
}

func (nb *build) Execute() (buildercommon.Operation, error) {
	if nb.err != nil {
		return nil, nb.err
	}
	return &operation{builder: nb.builder}, nil
}

// Builder is a no-op Builder implementation.
type Builder struct {
	// ErrorMessage is the error message that should be returned by builds
	// executed by this builder.
	ErrorMessage string

	// Err is the error that should be returned from calls to this builder.
	Err error
}

func (nb *Builder) Builder() v1alpha1.BuildProvider {
	// Masquerade as the Google builder.
	return v1alpha1.GoogleBuildProvider
}

// Validate does nothing.
func (nb *Builder) Validate(u *v1alpha1.Build) error { return nil }

// BuildFromSpec returns the converted build, or the builder's predefined error.
func (nb *Builder) BuildFromSpec(*v1alpha1.Build) (buildercommon.Build, error) {
	b := &build{builder: nb}
	if nb.Err != nil {
		b.err = nb.Err
	}
	return b, nil
}

// OperationFromStatus returns the no-op operation.
func (nb *Builder) OperationFromStatus(*v1alpha1.BuildStatus) (buildercommon.Operation, error) {
	return &operation{builder: nb}, nil
}
