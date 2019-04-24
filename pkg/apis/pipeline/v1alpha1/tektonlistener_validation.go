package v1alpha1

import (
	"context"

	"github.com/knative/pkg/apis"
)

func (ps *TektonListenerSpec) Validate(ctx context.Context) *apis.FieldError {
	if ps.PipelineRunSpec == nil {
		return apis.ErrMissingField("tektonlistener.spec.PipelineRunSpec")
	}

	return nil
}

func (pr *TektonListener) Validate(ctx context.Context) *apis.FieldError {
	if err := validateObjectMetadata(pr.GetObjectMeta()).ViaField("metadata"); err != nil {
		return err
	}

	return nil
}
