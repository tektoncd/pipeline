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

package main

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	frtesting "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework/testing"
	"github.com/tektoncd/pipeline/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "knative.dev/pkg/system/testing"
)

func TestResolver(t *testing.T) {
	ctx, _ := ttesting.SetupFakeContext(t)

	r := &resolver{}

	request := &v1beta1.ResolutionRequest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "resolution.tekton.dev/v1beta1",
			Kind:       "ResolutionRequest",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "rr",
			Namespace:         "foo",
			CreationTimestamp: metav1.Time{Time: time.Now()},
			Labels: map[string]string{
				resolutioncommon.LabelKeyResolverType: "demo",
			},
		},
		Spec: v1beta1.ResolutionRequestSpec{},
	}
	d := test.Data{
		ResolutionRequests: []*v1beta1.ResolutionRequest{request},
	}

	expectedStatus := &v1beta1.ResolutionRequestStatus{
		ResolutionRequestStatusFields: v1beta1.ResolutionRequestStatusFields{
			Data: base64.StdEncoding.Strict().EncodeToString([]byte(pipeline)),
		},
	}

	// If you want to test scenarios where an error should occur, pass a non-nil error to RunResolverReconcileTest
	var expectedErr error

	frtesting.RunResolverReconcileTest(ctx, t, d, r, request, expectedStatus, expectedErr)
}
