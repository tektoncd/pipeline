package v1beta1

import (
	"context"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func (tr TaskRef) convertTo(ctx context.Context, sink *v1.TaskRef) {
	if tr.Bundle == "" {
		sink.Name = tr.Name
	}
	sink.Kind = v1.TaskKind(tr.Kind)
	sink.APIVersion = tr.APIVersion
	new := v1.ResolverRef{}
	tr.ResolverRef.convertTo(ctx, &new)
	sink.ResolverRef = new
	tr.convertBundleToResolver(sink)
}

func (tr *TaskRef) convertFrom(ctx context.Context, source v1.TaskRef) {
	tr.Name = source.Name
	tr.Kind = TaskKind(source.Kind)
	tr.APIVersion = source.APIVersion
	new := ResolverRef{}
	new.convertFrom(ctx, source.ResolverRef)
	tr.ResolverRef = new
}

// convertBundleToResolver converts v1beta1 bundle string to a remote reference with the bundle resolver in v1.
// The conversion from Resolver to Bundle is not being supported since remote resolution would be turned on by
// default and it will be in beta before the stored version of CRD getting swapped to v1.
func (tr TaskRef) convertBundleToResolver(sink *v1.TaskRef) {
	if tr.Bundle != "" {
		sink.ResolverRef = v1.ResolverRef{
			Resolver: "bundles",
			Params: []v1.Param{{
				Name:  "bundle",
				Value: v1.ParamValue{StringVal: tr.Bundle, Type: v1.ParamTypeString},
			}, {
				Name:  "name",
				Value: v1.ParamValue{StringVal: tr.Name, Type: v1.ParamTypeString},
			}, {
				Name:  "kind",
				Value: v1.ParamValue{StringVal: "Task", Type: v1.ParamTypeString},
			}},
		}
	}
}
