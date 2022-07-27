package v1beta1

import (
	"context"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func (tr TaskRef) convertTo(ctx context.Context, sink *v1.TaskRef) {
	sink.Name = tr.Name
	sink.Kind = v1.TaskKind(tr.Kind)
	sink.APIVersion = tr.APIVersion
	// TODO: handle bundle in #4546
	new := v1.ResolverRef{}
	tr.ResolverRef.convertTo(ctx, &new)
	sink.ResolverRef = new
}

func (tr *TaskRef) convertFrom(ctx context.Context, source v1.TaskRef) {
	tr.Name = source.Name
	tr.Kind = TaskKind(source.Kind)
	tr.APIVersion = source.APIVersion
	// TODO: handle bundle in #4546
	new := ResolverRef{}
	new.convertFrom(ctx, source.ResolverRef)
	tr.ResolverRef = new
}
