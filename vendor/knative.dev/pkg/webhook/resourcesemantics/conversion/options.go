/*
Copyright 2023 The Knative Authors

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

package conversion

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

type options struct {
	path  string
	wc    func(context.Context) context.Context
	kinds map[schema.GroupKind]GroupKindConversion
}

type OptionFunc func(*options)

func WithKinds(kinds map[schema.GroupKind]GroupKindConversion) OptionFunc {
	return func(o *options) {
		o.kinds = kinds
	}
}

func WithPath(path string) OptionFunc {
	return func(o *options) {
		o.path = path
	}
}

func WithWrapContext(f func(context.Context) context.Context) OptionFunc {
	return func(o *options) {
		o.wc = f
	}
}
