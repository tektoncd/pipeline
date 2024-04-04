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
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
)

// ParamImagePullSecret is the parameter defining what secret
// name to use for bundle requests.
const ParamImagePullSecret = "secret"

// ParamBundle is the parameter defining what the bundle image url is.
const ParamBundle = "bundle"

// ParamName is the parameter defining what the layer name in the bundle
// image is.
const ParamName = "name"

// ParamKind is the parameter defining what the layer kind in the bundle
// image is.
const ParamKind = "kind"

// OptionsFromParams parses the params from a resolution request and
// converts them into options to pass as part of a bundle request.
func OptionsFromParams(ctx context.Context, params []pipelinev1.Param) (RequestOptions, error) {
	opts := RequestOptions{}
	conf := framework.GetResolverConfigFromContext(ctx)

	paramsMap := make(map[string]pipelinev1.ParamValue)
	for _, p := range params {
		paramsMap[p.Name] = p.Value
	}

	bundleVal, ok := paramsMap[ParamBundle]
	if !ok || bundleVal.StringVal == "" {
		return opts, fmt.Errorf("parameter %q required", ParamBundle)
	}
	if _, err := name.ParseReference(bundleVal.StringVal); err != nil {
		return opts, fmt.Errorf("invalid bundle reference: %w", err)
	}

	nameVal, ok := paramsMap[ParamName]
	if !ok || nameVal.StringVal == "" {
		return opts, fmt.Errorf("parameter %q required", ParamName)
	}

	kindVal, ok := paramsMap[ParamKind]
	kind := ""
	if !ok || kindVal.StringVal == "" {
		if kindString, ok := conf[ConfigKind]; ok {
			kind = kindString
		} else {
			return opts, errors.New("default resource Kind  was not set during installation of the bundle resolver")
		}
	} else {
		kind = kindVal.StringVal
	}

	opts.ImagePullSecret = paramsMap[ParamImagePullSecret].StringVal
	opts.Bundle = bundleVal.StringVal
	opts.EntryName = nameVal.StringVal
	opts.Kind = kind

	return opts, nil
}
