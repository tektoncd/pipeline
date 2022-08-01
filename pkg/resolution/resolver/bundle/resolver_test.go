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

package bundle

import (
	"context"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
)

func TestGetSelector(t *testing.T) {
	resolver := Resolver{}
	sel := resolver.GetSelector(context.Background())
	if typ, has := sel[resolutioncommon.LabelKeyResolverType]; !has {
		t.Fatalf("unexpected selector: %v", sel)
	} else if typ != LabelValueBundleResolverType {
		t.Fatalf("unexpected type: %q", typ)
	}
}

func TestValidateParams(t *testing.T) {
	resolver := Resolver{}

	paramsWithTask := map[string]v1beta1.ArrayOrString{
		ParamKind:           *v1beta1.NewArrayOrString("task"),
		ParamName:           *v1beta1.NewArrayOrString("foo"),
		ParamBundle:         *v1beta1.NewArrayOrString("bar"),
		ParamServiceAccount: *v1beta1.NewArrayOrString("baz"),
	}
	if err := resolver.ValidateParams(context.Background(), paramsWithTask); err != nil {
		t.Fatalf("unexpected error validating params: %v", err)
	}

	paramsWithPipeline := map[string]v1beta1.ArrayOrString{
		ParamKind:           *v1beta1.NewArrayOrString("pipeline"),
		ParamName:           *v1beta1.NewArrayOrString("foo"),
		ParamBundle:         *v1beta1.NewArrayOrString("bar"),
		ParamServiceAccount: *v1beta1.NewArrayOrString("baz"),
	}
	if err := resolver.ValidateParams(context.Background(), paramsWithPipeline); err != nil {
		t.Fatalf("unexpected error validating params: %v", err)
	}
}

func TestValidateParamsMissing(t *testing.T) {
	resolver := Resolver{}

	var err error

	paramsMissingBundle := map[string]v1beta1.ArrayOrString{
		ParamKind:           *v1beta1.NewArrayOrString("pipeline"),
		ParamName:           *v1beta1.NewArrayOrString("foo"),
		ParamServiceAccount: *v1beta1.NewArrayOrString("baz"),
	}
	err = resolver.ValidateParams(context.Background(), paramsMissingBundle)
	if err == nil {
		t.Fatalf("expected missing kind err")
	}

	paramsMissingName := map[string]v1beta1.ArrayOrString{
		ParamKind:           *v1beta1.NewArrayOrString("pipeline"),
		ParamBundle:         *v1beta1.NewArrayOrString("bar"),
		ParamServiceAccount: *v1beta1.NewArrayOrString("baz"),
	}
	err = resolver.ValidateParams(context.Background(), paramsMissingName)
	if err == nil {
		t.Fatalf("expected missing name err")
	}

}
