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
	pr.convertBundleToResolver(sink)
}

func (pr *PipelineRef) convertFrom(ctx context.Context, source v1.PipelineRef) {
	pr.Name = source.Name
	pr.APIVersion = source.APIVersion
	new := ResolverRef{}
	new.convertFrom(ctx, source.ResolverRef)
	pr.ResolverRef = new
}

// convertBundleToResolver converts v1beta1 bundle string to a remote reference with the bundle resolver in v1.
// The conversion from Resolver to Bundle is not being supported since remote resolution would be turned on by
// default and it will be in beta before the stored version of CRD getting swapped to v1.
func (pr PipelineRef) convertBundleToResolver(sink *v1.PipelineRef) {
	if pr.Bundle != "" {
		sink.ResolverRef = v1.ResolverRef{
			Resolver: "bundles",
			Params: []v1.Param{{
				Name:  "bundle",
				Value: v1.ParamValue{StringVal: pr.Bundle},
			}, {
				Name:  "name",
				Value: v1.ParamValue{StringVal: pr.Name},
			}, {
				Name:  "kind",
				Value: v1.ParamValue{StringVal: "Task"},
			}},
		}
	}
}
