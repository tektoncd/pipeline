package v1beta1

import (
	"context"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func (rr ResolverRef) convertTo(ctx context.Context, sink *v1.ResolverRef) {
	sink.Resolver = v1.ResolverName(rr.Resolver)
	sink.Params = nil
	for _, r := range rr.Params {
		new := v1.Param{}
		r.convertTo(ctx, &new)
		sink.Params = append(sink.Params, new)
	}
}

func (rr *ResolverRef) convertFrom(ctx context.Context, source v1.ResolverRef) {
	rr.Resolver = ResolverName(source.Resolver)
	rr.Params = nil
	for _, r := range source.Params {
		new := Param{}
		new.convertFrom(ctx, r)
		rr.Params = append(rr.Params, new)
	}
}
