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

package v1alpha1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	pipelinev1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestResolutionRequestConversionBadType(t *testing.T) {
	good, bad := &ResolutionRequest{}, &pipelinev1beta1.Task{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

func TestResolutionRequestConvertRoundTrip(t *testing.T) {
	testCases := []struct {
		name string
		in   *ResolutionRequest
		out  apis.Convertible
	}{{
		name: "no params",
		in: &ResolutionRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: ResolutionRequestSpec{
				Parameters: nil,
			},
		},
	}, {
		name: "with params",
		in: &ResolutionRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: ResolutionRequestSpec{
				Parameters: map[string]string{
					"some-param": "some-value",
				},
			},
		},
	}, {
		name: "with status refsource",
		in: &ResolutionRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: ResolutionRequestSpec{
				Parameters: map[string]string{
					"some-param": "some-value",
				},
			},
			Status: ResolutionRequestStatus{
				ResolutionRequestStatusFields: ResolutionRequestStatusFields{
					Data: "foobar",
					RefSource: &pipelinev1beta1.RefSource{
						URI:        "abcd",
						Digest:     map[string]string{"123": "456"},
						EntryPoint: "baz",
					},
				},
			},
		},
	}, {
		name: "with status, no refsource",
		in: &ResolutionRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: ResolutionRequestSpec{
				Parameters: map[string]string{
					"some-param": "some-value",
				},
			},
			Status: ResolutionRequestStatus{
				ResolutionRequestStatusFields: ResolutionRequestStatusFields{
					Data: "foobar",
				},
			},
		},
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := &v1beta1.ResolutionRequest{}
			if err := tc.in.ConvertTo(context.Background(), got); err != nil {
				t.Fatalf("ConvertTo() = %v", err)
			}

			t.Logf("ConvertTo() = %#v", got)

			roundTrip := &ResolutionRequest{}
			if err := roundTrip.ConvertFrom(context.Background(), got); err != nil {
				t.Errorf("ConvertFrom() = %v", err)
			}

			if d := cmp.Diff(tc.in, roundTrip); d != "" {
				t.Errorf("converted ResolutionRequest did not match expected: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestResolutionRequestConvertFromDeprecated(t *testing.T) {
	testCases := []struct {
		name string
		in   *v1beta1.ResolutionRequest
		want apis.Convertible
	}{{
		name: "with status.source",
		in: &v1beta1.ResolutionRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.ResolutionRequestSpec{
				Params: nil,
			},
			Status: v1beta1.ResolutionRequestStatus{
				ResolutionRequestStatusFields: v1beta1.ResolutionRequestStatusFields{
					Source: &pipelinev1beta1.ConfigSource{
						URI:        "abcd",
						Digest:     map[string]string{"123": "456"},
						EntryPoint: "baz",
					},
				},
			},
		},
		want: &ResolutionRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: ResolutionRequestSpec{
				Parameters: nil,
			},
			Status: ResolutionRequestStatus{
				ResolutionRequestStatusFields: ResolutionRequestStatusFields{
					RefSource: &pipelinev1beta1.RefSource{
						URI:        "abcd",
						Digest:     map[string]string{"123": "456"},
						EntryPoint: "baz",
					},
				},
			},
		},
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := &ResolutionRequest{}
			if err := got.ConvertFrom(context.Background(), tc.in); err != nil {
				t.Errorf("ConvertFrom() = %v", err)
			}

			if d := cmp.Diff(tc.want, got); d != "" {
				t.Errorf("converted ResolutionRequest did not match expected: %s", diff.PrintWantGot(d))
			}
		})
	}
}
