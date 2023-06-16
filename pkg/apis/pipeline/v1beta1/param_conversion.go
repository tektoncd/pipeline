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

func (p ParamSpec) convertTo(ctx context.Context, sink *v1.ParamSpec) {
	sink.Name = p.Name
	if p.Type != "" {
		sink.Type = v1.ParamType(p.Type)
	} else {
		sink.Type = v1.ParamType(ParamTypeString)
	}
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
		sink.Default = &v1.ParamValue{
			Type: v1.ParamType(p.Default.Type), StringVal: p.Default.StringVal,
			ArrayVal: p.Default.ArrayVal, ObjectVal: p.Default.ObjectVal,
		}
	}
}

func (p *ParamSpec) convertFrom(ctx context.Context, source v1.ParamSpec) {
	p.Name = source.Name
	if source.Type != "" {
		p.Type = ParamType(source.Type)
	} else {
		p.Type = ParamTypeString
	}
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
		p.Default = &ParamValue{
			Type: ParamType(source.Default.Type), StringVal: source.Default.StringVal,
			ArrayVal: source.Default.ArrayVal, ObjectVal: source.Default.ObjectVal,
		}
	}
}

func (p Param) convertTo(ctx context.Context, sink *v1.Param) {
	sink.Name = p.Name
	newValue := v1.ParamValue{}
	p.Value.convertTo(ctx, &newValue)
	sink.Value = newValue
}

// ConvertFrom converts v1beta1 Param from v1 Param
func (p *Param) ConvertFrom(ctx context.Context, source v1.Param) {
	p.Name = source.Name
	newValue := ParamValue{}
	newValue.convertFrom(ctx, source.Value)
	p.Value = newValue
}

func (v ParamValue) convertTo(ctx context.Context, sink *v1.ParamValue) {
	if v.Type != "" {
		sink.Type = v1.ParamType(v.Type)
	} else {
		sink.Type = v1.ParamType(ParamTypeString)
	}
	sink.StringVal = v.StringVal
	sink.ArrayVal = v.ArrayVal
	sink.ObjectVal = v.ObjectVal
}

func (v *ParamValue) convertFrom(ctx context.Context, source v1.ParamValue) {
	if source.Type != "" {
		v.Type = ParamType(source.Type)
	} else {
		v.Type = ParamTypeString
	}
	v.StringVal = source.StringVal
	v.ArrayVal = source.ArrayVal
	v.ObjectVal = source.ObjectVal
}
