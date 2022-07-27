package v1beta1

import (
	"context"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func (rr ResolverRef) convertTo(ctx context.Context, sink *v1.ResolverRef) {
	sink.Resolver = v1.ResolverName(rr.Resolver)
	sink.Resource = nil
	for _, r := range rr.Resource {
		new := v1.ResolverParam{}
		r.convertTo(ctx, &new)
		sink.Resource = append(sink.Resource, new)
	}
}

func (rr *ResolverRef) convertFrom(ctx context.Context, source v1.ResolverRef) {
	rr.Resolver = ResolverName(source.Resolver)
	rr.Resource = nil
	for _, r := range source.Resource {
		new := ResolverParam{}
		new.convertFrom(ctx, r)
		rr.Resource = append(rr.Resource, new)
	}
}

func (rp ResolverParam) convertTo(ctx context.Context, sink *v1.ResolverParam) {
	sink.Name = rp.Name
	sink.Value = rp.Value
}

func (rp *ResolverParam) convertFrom(ctx context.Context, source v1.ResolverParam) {
	rp.Name = source.Name
	rp.Value = source.Value
}
