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

package internal

import (
	"encoding/base64"

	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// CreateResolutionRequestStatusWithData returns a ResolutionRequestStatus with the resolved content.
func CreateResolutionRequestStatusWithData(content []byte) *v1beta1.ResolutionRequestStatus {
	return &v1beta1.ResolutionRequestStatus{
		Status: duckv1.Status{},
		ResolutionRequestStatusFields: v1beta1.ResolutionRequestStatusFields{
			Data: base64.StdEncoding.Strict().EncodeToString(content),
		},
	}
}

// CreateResolutionRequestFailureStatus returns a ResolutionRequestStatus with failure.
func CreateResolutionRequestFailureStatus() *v1beta1.ResolutionRequestStatus {
	return &v1beta1.ResolutionRequestStatus{
		Status: duckv1.Status{
			Conditions: duckv1.Conditions{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionFalse,
				Reason: resolutioncommon.ReasonResolutionFailed,
			}},
		},
	}
}
