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

func (tr TaskRef) convertTo(ctx context.Context, sink *v1.TaskRef) {
	sink.Name = tr.Name
	sink.Kind = v1.TaskKind(tr.Kind)
	sink.APIVersion = tr.APIVersion
	new := v1.ResolverRef{}
	tr.ResolverRef.convertTo(ctx, &new)
	sink.ResolverRef = new
}

// ConvertFrom converts v1beta1 TaskRef from v1 TaskRef
func (tr *TaskRef) ConvertFrom(ctx context.Context, source v1.TaskRef) {
	tr.Name = source.Name
	tr.Kind = TaskKind(source.Kind)
	tr.APIVersion = source.APIVersion
	new := ResolverRef{}
	new.convertFrom(ctx, source.ResolverRef)
	tr.ResolverRef = new
}
