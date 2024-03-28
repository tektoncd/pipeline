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

func (pr PipelineRef) convertTo(ctx context.Context, sink *v1.PipelineRef) {
	sink.Name = pr.Name
	sink.APIVersion = pr.APIVersion
	new := v1.ResolverRef{}
	pr.ResolverRef.convertTo(ctx, &new)
	sink.ResolverRef = new
}

func (pr *PipelineRef) convertFrom(ctx context.Context, source v1.PipelineRef) {
	pr.Name = source.Name
	pr.APIVersion = source.APIVersion
	new := ResolverRef{}
	new.convertFrom(ctx, source.ResolverRef)
	pr.ResolverRef = new
}
