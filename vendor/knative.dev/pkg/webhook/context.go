/*
Copyright 2019 The Knative Authors

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

package webhook

import "context"

// optionsKey is used as the key for associating information
// with a context.Context.
type optionsKey struct{}

// WithOptions associates a set of webhook.Options with
// the returned context.
func WithOptions(ctx context.Context, opt Options) context.Context {
	return context.WithValue(ctx, optionsKey{}, &opt)
}

// GetOptions retrieves webhook.Options associated with the
// given context via WithOptions (above).
func GetOptions(ctx context.Context) *Options {
	v := ctx.Value(optionsKey{})
	if v == nil {
		return nil
	}
	return v.(*Options)
}
