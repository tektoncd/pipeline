package test

import (
	"context"

	resolution "github.com/tektoncd/resolution/pkg/resource"
)

var _ resolution.Requester = &Requester{}
var _ resolution.ResolvedResource = &ResolvedResource{}

// NewRequester creates a mock requester that resolves to the given
// resource or returns the given error on Submit().
func NewRequester(resource resolution.ResolvedResource, err error) *Requester {
	return &Requester{
		ResolvedResource: resource,
		SubmitErr:        err,
	}
}

// NewResolvedResource creates a mock resolved resource that is
// populated with the given data and annotations or returns the given
// error from its Data() method.
func NewResolvedResource(data []byte, annotations map[string]string, dataErr error) *ResolvedResource {
	return &ResolvedResource{
		ResolvedData:        data,
		ResolvedAnnotations: annotations,
		DataErr:             dataErr,
	}
}

// Requester implements resolution.Requester and makes it easier
// to mock the outcome of a remote pipelineRef or taskRef resolution.
type Requester struct {
	// The resolved resource object to return when a request is
	// submitted.
	ResolvedResource resolution.ResolvedResource
	// An error to return when a request is submitted.
	SubmitErr error
}

// Submit implements resolution.Requester, accepting the name of a
// resolver and a request for a specific remote file, and then returns
// whatever mock data was provided on initialization.
func (r *Requester) Submit(ctx context.Context, resolverName resolution.ResolverName, req resolution.Request) (resolution.ResolvedResource, error) {
	return r.ResolvedResource, r.SubmitErr
}

// ResolvedResource implements resolution.ResolvedResource and makes
// it easier to mock the resolved content of a fetched pipeline or task.
type ResolvedResource struct {
	// The resolved bytes to return when resolution is complete.
	ResolvedData []byte
	// An error to return instead of the resolved bytes after
	// resolution completes.
	DataErr error
	// Annotations to return when resolution is complete.
	ResolvedAnnotations map[string]string
}

// Data implements resolution.ResolvedResource and returns the mock
// data and/or error given to it on initialization.
func (r *ResolvedResource) Data() ([]byte, error) {
	return r.ResolvedData, r.DataErr
}

// Annotations implements resolution.ResolvedResource and returns
// the mock annotations given to it on initialization.
func (r *ResolvedResource) Annotations() map[string]string {
	return r.ResolvedAnnotations
}
