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

	"github.com/tektoncd/pipeline/pkg/apis/config"
)

// ContextWithGitResolverEnabled returns a context containing a Config with the enable-git-resolver feature flag enabled.
func ContextWithGitResolverEnabled(ctx context.Context) context.Context {
	return contextWithResolverEnabled(ctx, "enable-git-resolver")
}

// ContextWithHubResolverEnabled returns a context containing a Config with the enable-hub-resolver feature flag enabled.
func ContextWithHubResolverEnabled(ctx context.Context) context.Context {
	return contextWithResolverEnabled(ctx, "enable-hub-resolver")
}

// ContextWithBundlesResolverEnabled returns a context containing a Config with the enable-bundles-resolver feature flag enabled.
func ContextWithBundlesResolverEnabled(ctx context.Context) context.Context {
	return contextWithResolverEnabled(ctx, "enable-bundles-resolver")
}

func contextWithResolverEnabled(ctx context.Context, resolverFlag string) context.Context {
	featureFlags, _ := config.NewFeatureFlagsFromMap(map[string]string{
		resolverFlag: "true",
	})
	cfg := &config.Config{
		FeatureFlags: featureFlags,
	}
	return config.ToContext(ctx, cfg)
}
