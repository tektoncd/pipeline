/*
Copyright 2020 The Knative Authors

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

package reconciler

import "context"

// ConfigStore is used to attach the frozen configuration to the context.
type ConfigStore interface {
	// ToContext is used to attach the frozen configuration to the context.
	ToContext(ctx context.Context) context.Context
}

// ConfigStores is used to combine multiple ConfigStore and attach multiple frozen configurations
// to the context.
type ConfigStores []ConfigStore

// ConfigStores implements ConfigStore interface.
var _ ConfigStore = ConfigStores{}

func (stores ConfigStores) ToContext(ctx context.Context) context.Context {
	for _, s := range stores {
		ctx = s.ToContext(ctx) //nolint:fatcontext
	}
	return ctx
}
