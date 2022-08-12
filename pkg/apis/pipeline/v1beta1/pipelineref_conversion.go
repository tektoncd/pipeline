package v1beta1

import (
	"context"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func (pr PipelineRef) convertTo(ctx context.Context, sink *v1.PipelineRef) {
	sink.Name = pr.Name
	sink.APIVersion = pr.APIVersion
	// TODO: handle bundle in #4546
	new := v1.ResolverRef{}
	pr.ResolverRef.convertTo(ctx, &new)
	sink.ResolverRef = new
}

func (pr *PipelineRef) convertFrom(ctx context.Context, source v1.PipelineRef) {
	pr.Name = source.Name
	pr.APIVersion = source.APIVersion
	// TODO: handle bundle in #4546
	new := ResolverRef{}
	new.convertFrom(ctx, source.ResolverRef)
	pr.ResolverRef = new
}
