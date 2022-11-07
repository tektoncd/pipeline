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

package testing

import (
	"context"

	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
)

// ContextWithGitResolverDisabled returns a context containing a Config with the enable-git-resolver feature flag disabled.
func ContextWithGitResolverDisabled(ctx context.Context) context.Context {
	return contextWithResolverDisabled(ctx, "enable-git-resolver")
}

// ContextWithHubResolverDisabled returns a context containing a Config with the enable-hub-resolver feature flag disabled.
func ContextWithHubResolverDisabled(ctx context.Context) context.Context {
	return contextWithResolverDisabled(ctx, "enable-hub-resolver")
}

// ContextWithBundlesResolverDisabled returns a context containing a Config with the enable-bundles-resolver feature flag disabled.
func ContextWithBundlesResolverDisabled(ctx context.Context) context.Context {
	return contextWithResolverDisabled(ctx, "enable-bundles-resolver")
}

// ContextWithClusterResolverDisabled returns a context containing a Config with the enable-cluster-resolver feature flag disabled.
func ContextWithClusterResolverDisabled(ctx context.Context) context.Context {
	return contextWithResolverDisabled(ctx, "enable-cluster-resolver")
}

func contextWithResolverDisabled(ctx context.Context, resolverFlag string) context.Context {
	featureFlags, _ := resolverconfig.NewFeatureFlagsFromMap(map[string]string{
		resolverFlag: "false",
	})
	cfg := &resolverconfig.Config{
		FeatureFlags: featureFlags,
	}
	return resolverconfig.ToContext(ctx, cfg)
}
