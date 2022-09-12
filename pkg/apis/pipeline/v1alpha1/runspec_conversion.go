package v1alpha1

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

func (pr EmbeddedRunSpec) convertTo(ctx context.Context, sink *v1beta1.EmbeddedCustomRunSpec) {
	sink.APIVersion = pr.APIVersion
	sink.Metadata = pr.Metadata
	sink.Spec = pr.Spec
}

func (pr *EmbeddedRunSpec) convertFrom(ctx context.Context, source v1beta1.EmbeddedCustomRunSpec) {
	pr.APIVersion = source.APIVersion
	pr.Metadata = source.Metadata
	pr.Spec = source.Spec
}
