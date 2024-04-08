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

package framework

import (
	"context"
	"fmt"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
)

const FakeUrl string = "fake://url"

var _ Resolver = &FakeResolver{}

// FakeResolver implements a framework.Resolver that can fetch pre-configured strings based on a parameter value, or return
// resolution attempts with a configured error.
type FakeResolver framework.FakeResolver

// Initialize performs any setup required by the fake resolver.
func (r *FakeResolver) Initialize(ctx context.Context) error {
	if r.ForParam == nil {
		r.ForParam = make(map[string]*framework.FakeResolvedResource)
	}
	return nil
}

// GetName returns the string name that the fake resolver should be
// associated with.
func (r *FakeResolver) GetName(_ context.Context) string {
	return framework.FakeResolverName
}

// GetSelector returns the labels that resource requests are required to have for
// the fake resolver to process them.
func (r *FakeResolver) GetSelector(_ context.Context) map[string]string {
	return map[string]string{
		resolutioncommon.LabelKeyResolverType: framework.LabelValueFakeResolverType,
	}
}

// Validate returns an error if the given parameter map is not
// valid for a resource request targeting the fake resolver.
func (r *FakeResolver) Validate(_ context.Context, req *v1beta1.ResolutionRequestSpec) error {
	if len(req.Params) > 0 {
		return framework.ValidateParams(req.Params)
	}
	if req.URL != FakeUrl {
		return fmt.Errorf("Wrong url. Expected: %s,  Got: %s", FakeUrl, req.URL)
	}
	return nil
}

// Resolve performs the work of fetching a file from the fake resolver given a map of
// parameters.
func (r *FakeResolver) Resolve(_ context.Context, req *v1beta1.ResolutionRequestSpec) (framework.ResolvedResource, error) {
	if len(req.Params) > 0 {
		return framework.Resolve(req.Params, r.ForParam)
	}
	frr, ok := r.ForParam[req.URL]
	if !ok {
		return nil, fmt.Errorf("couldn't find resource for url %s", req.URL)
	}
	return frr, nil
}

var _ framework.TimedResolution = &FakeResolver{}

// GetResolutionTimeout returns the configured timeout for the reconciler, or the default time.Duration if not configured.
func (r *FakeResolver) GetResolutionTimeout(ctx context.Context, defaultTimeout time.Duration) time.Duration {
	return framework.GetResolutionTimeout(r.Timeout, defaultTimeout)
}
