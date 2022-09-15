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

package v1beta1

import (
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
)

// ResolutionRequests only have apis.ConditionSucceeded for now.
var resolutionRequestCondSet = apis.NewBatchConditionSet()

// GetGroupVersionKind implements kmeta.OwnerRefable.
func (*ResolutionRequest) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("ResolutionRequest")
}

// GetConditionSet implements KRShaped.
func (*ResolutionRequest) GetConditionSet() apis.ConditionSet {
	return resolutionRequestCondSet
}

// HasStarted returns whether a ResolutionRequests Status is considered to
// be in-progress.
func (rr *ResolutionRequest) HasStarted() bool {
	return rr.Status.GetCondition(apis.ConditionSucceeded).IsUnknown()
}

// IsDone returns whether a ResolutionRequests Status is considered to be
// in a completed state, independent of success/failure.
func (rr *ResolutionRequest) IsDone() bool {
	finalStateIsUnknown := rr.Status.GetCondition(apis.ConditionSucceeded).IsUnknown()
	return !finalStateIsUnknown
}

// InitializeConditions set ths initial values of the conditions.
func (s *ResolutionRequestStatus) InitializeConditions() {
	resolutionRequestCondSet.Manage(s).InitializeConditions()
}

// MarkFailed sets the Succeeded condition to False with an accompanying
// error message.
func (s *ResolutionRequestStatus) MarkFailed(reason, message string) {
	resolutionRequestCondSet.Manage(s).MarkFalse(apis.ConditionSucceeded, reason, message)
}

// MarkSucceeded sets the Succeeded condition to True.
func (s *ResolutionRequestStatus) MarkSucceeded() {
	resolutionRequestCondSet.Manage(s).MarkTrue(apis.ConditionSucceeded)
}

// MarkInProgress updates the Succeeded condition to Unknown with an
// accompanying message.
func (s *ResolutionRequestStatus) MarkInProgress(message string) {
	resolutionRequestCondSet.Manage(s).MarkUnknown(apis.ConditionSucceeded, resolutioncommon.ReasonResolutionInProgress, message)
}
