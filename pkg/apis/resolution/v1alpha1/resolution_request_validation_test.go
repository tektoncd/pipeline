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
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestResolutionRequest_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		rr      *ResolutionRequest
		wantErr *apis.FieldError
	}{{
		name:    "missing type label",
		rr:      &ResolutionRequest{},
		wantErr: apis.ErrMissingField(common.LabelKeyResolverType).ViaField("labels").ViaField("meta"),
	}, {
		name: "invalid params - exactly same names",
		rr: &ResolutionRequest{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{common.LabelKeyResolverType: "foo"},
			},
			Spec: ResolutionRequestSpec{
				Params: []v1beta1.Param{{
					Name:  "myname",
					Value: *v1beta1.NewArrayOrString("value"),
				}, {
					Name:  "myname",
					Value: *v1beta1.NewArrayOrString("value"),
				}},
			},
		},
		wantErr: apis.ErrMultipleOneOf("params[myname].name").ViaField("spec"),
	}, {
		name: "invalid params - same names but different case",
		rr: &ResolutionRequest{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{common.LabelKeyResolverType: "foo"},
			},
			Spec: ResolutionRequestSpec{
				Params: []v1beta1.Param{{
					Name:  "FOO",
					Value: *v1beta1.NewArrayOrString("value"),
				}, {
					Name:  "foo",
					Value: *v1beta1.NewArrayOrString("value"),
				}},
			},
		},
		wantErr: apis.ErrMultipleOneOf("params[foo].name").ViaField("spec"),
	}, {
		name: "invalid params (object type) - same names but different case",
		rr: &ResolutionRequest{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{common.LabelKeyResolverType: "foo"},
			},
			Spec: ResolutionRequestSpec{
				Params: []v1beta1.Param{{
					Name:  "MYOBJECTPARAM",
					Value: *v1beta1.NewObject(map[string]string{"key1": "val1", "key2": "val2"}),
				}, {
					Name:  "myobjectparam",
					Value: *v1beta1.NewObject(map[string]string{"key1": "val1", "key2": "val2"}),
				}},
			},
		},
		wantErr: apis.ErrMultipleOneOf("params[myobjectparam].name").ViaField("spec"),
	}}

	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			ctx := config.EnableAlphaAPIFields(context.Background())
			err := ts.rr.Validate(ctx)
			if d := cmp.Diff(ts.wantErr.Error(), err.Error()); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestResolutionRequest_Valid(t *testing.T) {
	tests := []struct {
		name string
		rr   *ResolutionRequest
	}{{
		name: "with label, no params",
		rr: &ResolutionRequest{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{common.LabelKeyResolverType: "foo"},
			},
			Spec: ResolutionRequestSpec{},
		},
	}, {
		name: "with label, valid params",
		rr: &ResolutionRequest{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{common.LabelKeyResolverType: "foo"},
			},
			Spec: ResolutionRequestSpec{
				Params: []v1beta1.Param{{
					Name:  "a",
					Value: *v1beta1.NewArrayOrString("value"),
				}, {
					Name:  "b",
					Value: *v1beta1.NewArrayOrString("value1", "value2"),
				}, {
					Name:  "c",
					Value: *v1beta1.NewObject(map[string]string{"foo": "bar"}),
				}},
			},
		},
	}}

	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			ctx := config.EnableAlphaAPIFields(context.Background())
			if err := ts.rr.Validate(ctx); err != nil {
				t.Error(err)
			}
		})
	}
}
