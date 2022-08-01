package v1beta1

import (
	"context"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func (p ParamSpec) convertTo(ctx context.Context, sink *v1.ParamSpec) {
	sink.Name = p.Name
	sink.Type = v1.ParamType(p.Type)
	sink.Description = p.Description
	var properties map[string]v1.PropertySpec
	if p.Properties != nil {
		properties = make(map[string]v1.PropertySpec)
	}
	for k, v := range p.Properties {
		properties[k] = v1.PropertySpec{Type: v1.ParamType(v.Type)}
	}
	sink.Properties = properties
	if p.Default != nil {
		sink.Default = &v1.ArrayOrString{
			Type: v1.ParamType(p.Default.Type), StringVal: p.Default.StringVal,
			ArrayVal: p.Default.ArrayVal, ObjectVal: p.Default.ObjectVal,
		}
	}
}

func (p *ParamSpec) convertFrom(ctx context.Context, source v1.ParamSpec) {
	p.Name = source.Name
	p.Type = ParamType(source.Type)
	p.Description = source.Description
	var properties map[string]PropertySpec
	if source.Properties != nil {
		properties = make(map[string]PropertySpec)
	}
	for k, v := range source.Properties {
		properties[k] = PropertySpec{Type: ParamType(v.Type)}
	}
	p.Properties = properties
	if source.Default != nil {
		p.Default = &ArrayOrString{
			Type: ParamType(source.Default.Type), StringVal: source.Default.StringVal,
			ArrayVal: source.Default.ArrayVal, ObjectVal: source.Default.ObjectVal,
		}
	}
}
