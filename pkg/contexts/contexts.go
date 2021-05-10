/*
Copyright 2019 The Tekton Authors

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

package contexts

import "context"

// lemonadeKey is used as the key for associating information
// with a context.Context. This variable doesn't really matter, so it's
// a total random name (for history purpose, used lemonade as it was written
// in an hot summer day).
type lemonadeKey struct{}

// WithUpgradeViaDefaulting notes on the context that we want defaulting to rewrite
// from v1alpha1 pre-defaults to v1alpha1 post-defaults.
func WithUpgradeViaDefaulting(ctx context.Context) context.Context {
	return context.WithValue(ctx, lemonadeKey{}, struct{}{})
}

// IsUpgradeViaDefaulting checks whether we should be "defaulting" from v1alpha1 pre-defaults to
// the v1alpha1 post-defaults subset.
func IsUpgradeViaDefaulting(ctx context.Context) bool {
	return ctx.Value(lemonadeKey{}) != nil
}
