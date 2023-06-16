/*
Copyright 2023 The Tekton Authors

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

package v1beta1

import (
	"context"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func (rr ResolverRef) convertTo(ctx context.Context, sink *v1.ResolverRef) {
	sink.Resolver = v1.ResolverName(rr.Resolver)
	sink.Params = nil
	for _, r := range rr.Params {
		new := v1.Param{}
		r.convertTo(ctx, &new)
		sink.Params = append(sink.Params, new)
	}
}

func (rr *ResolverRef) convertFrom(ctx context.Context, source v1.ResolverRef) {
	rr.Resolver = ResolverName(source.Resolver)
	rr.Params = nil
	for _, r := range source.Params {
		new := Param{}
		new.ConvertFrom(ctx, r)
		rr.Params = append(rr.Params, new)
	}
}
