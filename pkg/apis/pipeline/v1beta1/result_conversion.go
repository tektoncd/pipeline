package v1beta1

import (
	"context"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func (r TaskResult) convertTo(ctx context.Context, sink *v1.TaskResult) {
	sink.Name = r.Name
	sink.Type = v1.ResultsType(r.Type)
	sink.Description = r.Description
	properties := make(map[string]v1.PropertySpec)
	for k, v := range r.Properties {
		properties[k] = v1.PropertySpec{Type: v1.ParamType(v.Type)}
	}
	sink.Properties = properties
}

func (r *TaskResult) convertFrom(ctx context.Context, source v1.TaskResult) {
	r.Name = source.Name
	r.Type = ResultsType(source.Type)
	r.Description = source.Description
	properties := make(map[string]PropertySpec)
	for k, v := range source.Properties {
		properties[k] = PropertySpec{Type: ParamType(v.Type)}
	}
	r.Properties = properties
}
