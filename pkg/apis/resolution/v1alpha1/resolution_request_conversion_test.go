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

package v1alpha1_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	v1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resolution/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestResolutionRequestConversionBadType(t *testing.T) {
	good, bad := &v1alpha1.ResolutionRequest{}, &pipelinev1.Task{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

func TestResolutionRequestConvertTo(t *testing.T) {
	versions := []apis.Convertible{&v1beta1.ResolutionRequest{}}

	testCases := []struct {
		name string
		in   *v1alpha1.ResolutionRequest
		out  apis.Convertible
	}{
		{
			name: "no params",
			in: &v1alpha1.ResolutionRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
				Spec: v1alpha1.ResolutionRequestSpec{
					Parameters: nil,
				},
			},
			out: &v1beta1.ResolutionRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
				Spec: v1beta1.ResolutionRequestSpec{
					Params: nil,
				},
			},
		}, {
			name: "with params",
			in: &v1alpha1.ResolutionRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
				Spec: v1alpha1.ResolutionRequestSpec{
					Parameters: map[string]string{
						"some-param": "some-value",
					},
				},
			},
			out: &v1beta1.ResolutionRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
				Spec: v1beta1.ResolutionRequestSpec{
					Params: []pipelinev1.Param{{
						Name:  "some-param",
						Value: *pipelinev1.NewStructuredValues("some-value"),
					}},
				},
			},
		},
	}

	for _, tc := range testCases {
		for _, version := range versions {
			t.Run(tc.name, func(t *testing.T) {
				got := version
				if err := tc.in.ConvertTo(context.Background(), got); err != nil {
					t.Fatalf("ConvertTo() = %v", err)
				}
				t.Logf("ConvertTo() = %#v", got)
				if d := cmp.Diff(tc.out, got); d != "" {
					t.Errorf("converted ResolutionRequest did not match expected: %s", diff.PrintWantGot(d))
				}
			})
		}
	}
}

func TestResolutionRequestConvertFrom(t *testing.T) {
	versions := []apis.Convertible{&v1alpha1.ResolutionRequest{}}

	testCases := []struct {
		name        string
		in          apis.Convertible
		out         *v1alpha1.ResolutionRequest
		expectedErr error
	}{
		{
			name: "no params",
			in: &v1beta1.ResolutionRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
				Spec: v1beta1.ResolutionRequestSpec{
					Params: nil,
				},
			},
			out: &v1alpha1.ResolutionRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
				Spec: v1alpha1.ResolutionRequestSpec{
					Parameters: nil,
				},
			},
		}, {
			name: "with only string params",
			in: &v1beta1.ResolutionRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
				Spec: v1beta1.ResolutionRequestSpec{
					Params: []pipelinev1.Param{{
						Name:  "some-param",
						Value: *pipelinev1.NewStructuredValues("some-value"),
					}},
				},
			},
			out: &v1alpha1.ResolutionRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
				Spec: v1alpha1.ResolutionRequestSpec{
					Parameters: map[string]string{
						"some-param": "some-value",
					},
				},
			},
		}, {
			name: "with non-string params",
			in: &v1beta1.ResolutionRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
				Spec: v1beta1.ResolutionRequestSpec{
					Params: []pipelinev1.Param{
						{
							Name:  "array-val",
							Value: *pipelinev1.NewStructuredValues("one", "two"),
						}, {
							Name:  "string-val",
							Value: *pipelinev1.NewStructuredValues("a-string"),
						}, {
							Name: "object-val",
							Value: *pipelinev1.NewObject(map[string]string{
								"key-one": "value-one",
								"key-two": "value-two",
							}),
						},
					},
				},
			},
			out:         nil,
			expectedErr: errors.New("cannot convert v1beta1 to v1alpha, non-string type parameter(s) found: array-val, object-val"),
		},
	}

	for _, tc := range testCases {
		for _, version := range versions {
			t.Run(tc.name, func(t *testing.T) {
				got := version
				err := got.ConvertFrom(context.Background(), tc.in)
				if tc.expectedErr != nil {
					if err == nil {
						t.Fatalf("expected error '%s', but did not get an error", tc.expectedErr.Error())
					} else if d := cmp.Diff(tc.expectedErr.Error(), err.Error()); d != "" {
						t.Fatalf("error did not meet expected: %s", diff.PrintWantGot(d))
					}
					return
				} else if err != nil {
					t.Fatalf("ConvertFrom() = %v", err)
				}
				t.Logf("ConvertFrom() = %#v", got)
				if d := cmp.Diff(tc.out, got); d != "" {
					t.Errorf("converted ResolutionRequest did not match expected: %s", diff.PrintWantGot(d))
				}
			})
		}
	}
}
