package v1alpha1

import "context"

// SetDefaults walks a ResolutionRequest object and sets any default
// values that are required to be set before a reconciler sees it.
func (rr *ResolutionRequest) SetDefaults(ctx context.Context) {
	if rr.TypeMeta.Kind == "" {
		rr.TypeMeta.Kind = "ResolutionRequest"
	}
	if rr.TypeMeta.APIVersion == "" {
		rr.TypeMeta.APIVersion = "resolution.tekton.dev/v1alpha1"
	}
}
