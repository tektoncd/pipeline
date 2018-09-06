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
package google

import (
	"fmt"
	"time"

	cloudbuild "google.golang.org/api/cloudbuild/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	buildercommon "github.com/knative/build/pkg/builder"
	"github.com/knative/build/pkg/builder/google/convert"
)

type operation struct {
	cloudbuild *cloudbuild.Service
	name       string
	startTime  metav1.Time
}

func (op *operation) Name() string {
	return op.name
}

func (op *operation) Checkpoint(status *v1alpha1.BuildStatus) error {
	status.Builder = v1alpha1.GoogleBuildProvider
	if status.Google == nil {
		status.Google = &v1alpha1.GoogleSpec{}
	}
	status.Google.Operation = op.Name()
	status.StartTime = op.startTime
	status.SetCondition(&v1alpha1.BuildCondition{
		Type:   v1alpha1.BuildSucceeded,
		Status: corev1.ConditionUnknown,
		Reason: "Building",
	})
	return nil
}

func (op *operation) Wait() (*v1alpha1.BuildStatus, error) {
	for {
		cbOp, err := op.cloudbuild.Operations.Get(op.Name()).Do()
		if err != nil {
			return nil, err
		}
		if cbOp.Done {
			bs := &v1alpha1.BuildStatus{
				Builder: v1alpha1.GoogleBuildProvider,
				Google: &v1alpha1.GoogleSpec{
					Operation: op.Name(),
				},
				StartTime:      op.startTime,
				CompletionTime: metav1.Now(),
			}
			if cbOp.Error != nil {
				bs.SetCondition(&v1alpha1.BuildCondition{
					Type:   v1alpha1.BuildSucceeded,
					Status: corev1.ConditionFalse,
					// TODO(mattmoor): Considering inlining the final "Status"
					// string from GCB's Build (camelcased) as "Reason" here:
					// https://godoc.org/google.golang.org/api/cloudbuild/v1#Build
					Message: cbOp.Error.Message,
				})
			} else {
				bs.SetCondition(&v1alpha1.BuildCondition{
					Type:   v1alpha1.BuildSucceeded,
					Status: corev1.ConditionTrue,
				})
			}
			return bs, nil
		}
		time.Sleep(1 * time.Second)
	}
}

type build struct {
	cloudbuild *cloudbuild.Service
	project    string
	body       cloudbuild.Build
}

func (b *build) Execute() (buildercommon.Operation, error) {
	op, err := b.cloudbuild.Projects.Builds.Create(b.project, &b.body).Do()
	if err != nil {
		return nil, err
	}
	return &operation{
		cloudbuild: b.cloudbuild,
		name:       op.Name,
		startTime:  metav1.Now(),
	}, nil
}

// NewBuilder constructs a Google-hosted builder.Interface for executing Build custom resources.
func NewBuilder(cloudbuild *cloudbuild.Service, project string) buildercommon.Interface {
	return &builder{
		cloudbuild: cloudbuild,
		project:    project,
	}
}

type builder struct {
	cloudbuild *cloudbuild.Service
	project    string
}

func (b *builder) Builder() v1alpha1.BuildProvider {
	return v1alpha1.GoogleBuildProvider
}

func (b *builder) Validate(u *v1alpha1.Build, tmpl *v1alpha1.BuildTemplate) error {
	_, err := convert.FromCRD(&u.Spec)
	return err
}

func (b *builder) BuildFromSpec(u *v1alpha1.Build) (buildercommon.Build, error) {
	bld, err := convert.FromCRD(&u.Spec)
	if err != nil {
		return nil, err
	}
	return &build{
		cloudbuild: b.cloudbuild,
		project:    b.project,
		body:       *bld,
	}, nil
}

func (b *builder) OperationFromStatus(status *v1alpha1.BuildStatus) (buildercommon.Operation, error) {
	if status.Builder != v1alpha1.GoogleBuildProvider {
		return nil, fmt.Errorf("not a 'Google' builder: %v", status.Builder)
	}
	if status.Google == nil {
		return nil, fmt.Errorf("status.google cannot be empty: %v", status)
	}
	return &operation{
		cloudbuild: b.cloudbuild,
		name:       status.Google.Operation,
		startTime:  status.StartTime,
	}, nil
}
