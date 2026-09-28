/*
Copyright 2026 The Tekton Authors

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
	rrcache "github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework/cache"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
)

// defaultMaximumResolutionDuration is the maximum time that a call to
// Resolve may take. A resolver implementing framework.TimedResolution can
// override it.
const defaultMaximumResolutionDuration = time.Minute

// prepareResolution validates framework-wide parameters and determines how long
// the resolver invocation may run.
func prepareResolution(ctx context.Context, resolver Resolver, key string, spec *v1beta1.ResolutionRequestSpec) (time.Duration, error) {
	params := make(map[string]string)
	for _, p := range spec.Params {
		params[p.Name] = p.Value.StringVal
	}

	if cacheMode, exists := params[rrcache.CacheParam]; exists && cacheMode != "" {
		if err := rrcache.Validate(cacheMode); err != nil {
			return 0, &resolutioncommon.InvalidRequestError{
				ResolutionRequestKey: key,
				Message:              err.Error(),
			}
		}
	}

	if timed, ok := resolver.(framework.TimedResolution); ok {
		return timed.GetResolutionTimeout(ctx, defaultMaximumResolutionDuration, params)
	}
	return defaultMaximumResolutionDuration, nil
}

// executeResolution invokes and validates a resolver without persisting its
// result. The caller is responsible for adding request-scoped values to ctx.
func executeResolution(ctx context.Context, resolver Resolver, key string, spec *v1beta1.ResolutionRequestSpec, timeout time.Duration) (framework.ResolvedResource, error) {
	resolutionCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	errChan := make(chan error, 1)
	resourceChan := make(chan framework.ResolvedResource, 1)
	go func() {
		if err := resolver.Validate(resolutionCtx, spec); err != nil {
			errChan <- &resolutioncommon.InvalidRequestError{
				ResolutionRequestKey: key,
				Message:              err.Error(),
			}
			return
		}

		resource, err := resolver.Resolve(resolutionCtx, spec)
		if err != nil {
			errChan <- &resolutioncommon.GetResourceError{
				ResolverName: resolver.GetName(resolutionCtx),
				Key:          key,
				Original:     err,
			}
			return
		}
		if err := framework.ValidateResolvedResource(resource); err != nil {
			errChan <- &resolutioncommon.GetResourceError{
				ResolverName: resolver.GetName(resolutionCtx),
				Key:          key,
				Original:     fmt.Errorf("resolved resource validation error: %w", err),
			}
			return
		}
		resourceChan <- resource
	}()

	select {
	case err := <-errChan:
		return nil, err
	case <-resolutionCtx.Done():
		return nil, resolutionCtx.Err()
	case resource := <-resourceChan:
		return resource, nil
	}
}
