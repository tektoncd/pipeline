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

func (r TaskResult) convertTo(ctx context.Context, sink *v1.TaskResult) {
	sink.Name = r.Name
	sink.Type = v1.ResultsType(r.Type)
	sink.Description = r.Description
	if r.Properties != nil {
		properties := make(map[string]v1.PropertySpec)
		for k, v := range r.Properties {
			properties[k] = v1.PropertySpec{Type: v1.ParamType(v.Type)}
		}
		sink.Properties = properties
	}
}

func (r *TaskResult) convertFrom(ctx context.Context, source v1.TaskResult) {
	r.Name = source.Name
	r.Type = ResultsType(source.Type)
	r.Description = source.Description
	if source.Properties != nil {
		properties := make(map[string]PropertySpec)
		for k, v := range source.Properties {
			properties[k] = PropertySpec{Type: ParamType(v.Type)}
		}
		r.Properties = properties
	}
}
