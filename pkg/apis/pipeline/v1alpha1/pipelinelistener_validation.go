package v1alpha1

import (
	"context"

	"github.com/knative/pkg/apis"
)

func (pr *PipelineListener) Validate(ctx context.Context) *apis.FieldError {
	return nil
}

func (pr *PipelineListenerSpec) Validate(ctx context.Context) *apis.FieldError {
	return nil
}
