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
	"errors"
	"fmt"
	"strings"
	"time"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
)

const (
	// LabelValueFakeResolverType is the value to use for the
	// resolution.tekton.dev/type label on resource requests
	LabelValueFakeResolverType string = "fake"

	// FakeResolverName is the name that the fake resolver should be
	// associated with
	FakeResolverName string = "Fake"

	// FakeParamName is the name used for the fake resolver's single parameter.
	FakeParamName string = "fake-key"
)

var _ Resolver = &FakeResolver{}

// FakeResolvedResource is a framework.ResolvedResource implementation for use with the fake resolver.
// If it's the value in the FakeResolver's ForParam map for the key given as the fake param value, the FakeResolver will
// first check if it's got a value for ErrorWith. If so, that string will be returned as an error. Then, if WaitFor is
// greater than zero, the FakeResolver will wait that long before returning. And finally, the FakeResolvedResource will
// be returned.
type FakeResolvedResource struct {
	Content       string
	AnnotationMap map[string]string
	ContentSource *pipelinev1.RefSource
	ErrorWith     string
	WaitFor       time.Duration
}

// Data returns the FakeResolvedResource's Content field as bytes.
func (f *FakeResolvedResource) Data() []byte {
	return []byte(f.Content)
}

// Annotations returns the FakeResolvedResource's AnnotationMap field.
func (f *FakeResolvedResource) Annotations() map[string]string {
	return f.AnnotationMap
}

// RefSource is the source reference of the remote data that records where the remote
// file came from including the url, digest and the entrypoint.
func (f *FakeResolvedResource) RefSource() *pipelinev1.RefSource {
	return f.ContentSource
}

// FakeResolver implements a framework.Resolver that can fetch pre-configured strings based on a parameter value, or return
// resolution attempts with a configured error.
type FakeResolver struct {
	ForParam map[string]*FakeResolvedResource
	Timeout  time.Duration
}

// Initialize performs any setup required by the fake resolver.
func (r *FakeResolver) Initialize(ctx context.Context) error {
	if r.ForParam == nil {
		r.ForParam = make(map[string]*FakeResolvedResource)
	}
	return nil
}

// GetName returns the string name that the fake resolver should be
// associated with.
func (r *FakeResolver) GetName(_ context.Context) string {
	return FakeResolverName
}

// GetSelector returns the labels that resource requests are required to have for
// the fake resolver to process them.
func (r *FakeResolver) GetSelector(_ context.Context) map[string]string {
	return map[string]string{
		resolutioncommon.LabelKeyResolverType: LabelValueFakeResolverType,
	}
}

// ValidateParams returns an error if the given parameter map is not
// valid for a resource request targeting the fake resolver.
func (r *FakeResolver) ValidateParams(_ context.Context, params []pipelinev1.Param) error {
	paramsMap := make(map[string]pipelinev1.ParamValue)
	for _, p := range params {
		paramsMap[p.Name] = p.Value
	}

	required := []string{
		FakeParamName,
	}
	missing := []string{}
	if params == nil {
		missing = required
	} else {
		for _, p := range required {
			v, has := paramsMap[p]
			if !has || v.StringVal == "" {
				missing = append(missing, p)
			}
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing %v", strings.Join(missing, ", "))
	}

	return nil
}

// Resolve performs the work of fetching a file from the fake resolver given a map of
// parameters.
func (r *FakeResolver) Resolve(_ context.Context, params []pipelinev1.Param) (ResolvedResource, error) {
	paramsMap := make(map[string]pipelinev1.ParamValue)
	for _, p := range params {
		paramsMap[p.Name] = p.Value
	}

	paramValue := paramsMap[FakeParamName].StringVal

	frr, ok := r.ForParam[paramValue]
	if !ok {
		return nil, fmt.Errorf("couldn't find resource for param value %s", paramValue)
	}

	if frr.ErrorWith != "" {
		return nil, errors.New(frr.ErrorWith)
	}

	if frr.WaitFor.Seconds() > 0 {
		time.Sleep(frr.WaitFor)
	}

	return frr, nil
}

var _ TimedResolution = &FakeResolver{}

// GetResolutionTimeout returns the configured timeout for the reconciler, or the default time.Duration if not configured.
func (r *FakeResolver) GetResolutionTimeout(ctx context.Context, defaultTimeout time.Duration) time.Duration {
	if r.Timeout > 0 {
		return r.Timeout
	}
	return defaultTimeout
}
