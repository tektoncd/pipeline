/*
Copyright 2022 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resource

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	rrclient "github.com/tektoncd/pipeline/pkg/client/resolution/clientset/versioned"
	rrlisters "github.com/tektoncd/pipeline/pkg/client/resolution/listers/resolution/v1beta1"
	common "github.com/tektoncd/pipeline/pkg/resolution/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

// CRDRequester implements the Requester interface using
// ResolutionRequest CRDs.
//
// Deprecated: Use [github.com/tektoncd/pipeline/pkg/remoteresolution/resource.CRDRequester] instead.
type CRDRequester struct {
	clientset rrclient.Interface
	lister    rrlisters.ResolutionRequestLister
}

// NewCRDRequester returns an implementation of Requester that uses
// ResolutionRequest CRD objects to mediate between the caller who wants a
// resource (e.g. Tekton Pipelines) and the responder who can fetch
// it (e.g. the gitresolver)
//
// Deprecated: Use [github.com/tektoncd/pipeline/pkg/remoteresolution/resource.NewCRDRequester] instead.
func NewCRDRequester(clientset rrclient.Interface, lister rrlisters.ResolutionRequestLister) *CRDRequester {
	return &CRDRequester{clientset, lister}
}

var _ Requester = &CRDRequester{}

// Submit constructs a ResolutionRequest object and submits it to the
// kubernetes cluster, returning any errors experienced while doing so.
// If ResolutionRequest is succeeded then it returns the resolved data.
func (r *CRDRequester) Submit(ctx context.Context, resolver ResolverName, req Request) (ResolvedResource, error) {
	rr, _ := r.lister.ResolutionRequests(req.Namespace()).Get(req.Name())
	if rr == nil {
		if err := r.createResolutionRequest(ctx, resolver, req); err != nil &&
			// When the request reconciles frequently, the creation may fail
			// because the list informer cache is not updated.
			// If the request already exists then we can assume that is in progress.
			// The next reconcile will handle it based on the actual situation.
			!apierrors.IsAlreadyExists(err) {
			return nil, err
		}
		return nil, common.ErrRequestInProgress
	}

	if rr.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
		// TODO(sbwsg): This should be where an existing
		// resource is given an additional owner reference so
		// that it doesn't get deleted until the caller is done
		// with it. Use appendOwnerReference and then submit
		// update to ResolutionRequest.
		return nil, common.ErrRequestInProgress
	}

	if rr.Status.GetCondition(apis.ConditionSucceeded).IsTrue() {
		return CrdIntoResource(rr), nil
	}

	message := rr.Status.GetCondition(apis.ConditionSucceeded).GetMessage()
	err := common.NewError(common.ReasonResolutionFailed, errors.New(message))
	return nil, err
}

func (r *CRDRequester) createResolutionRequest(ctx context.Context, resolver ResolverName, req Request) error {
	var owner metav1.OwnerReference
	if ownedReq, ok := req.(OwnedRequest); ok {
		owner = ownedReq.OwnerRef()
	}
	rr := CreateResolutionRequest(ctx, resolver, req.Name(), req.Namespace(), req.Params(), owner)
	_, err := r.clientset.ResolutionV1beta1().ResolutionRequests(rr.Namespace).Create(ctx, rr, metav1.CreateOptions{})
	return err
}

func CreateResolutionRequest(ctx context.Context, resolver common.ResolverName, name, namespace string, params []v1.Param, ownerRef metav1.OwnerReference) *v1beta1.ResolutionRequest {
	rr := &v1beta1.ResolutionRequest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "resolution.tekton.dev/v1beta1",
			Kind:       "ResolutionRequest",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				common.LabelKeyResolverType: string(resolver),
			},
		},
		Spec: v1beta1.ResolutionRequestSpec{
			Params: params,
		},
	}
	appendOwnerReference(rr, ownerRef)
	return rr
}

func appendOwnerReference(rr *v1beta1.ResolutionRequest, ownerRef metav1.OwnerReference) {
	isOwner := false
	for _, ref := range rr.ObjectMeta.OwnerReferences {
		if ownerRefsAreEqual(ref, ownerRef) {
			isOwner = true
		}
	}
	if !isOwner {
		rr.ObjectMeta.OwnerReferences = append(rr.ObjectMeta.OwnerReferences, ownerRef)
	}
}

func ownerRefsAreEqual(a, b metav1.OwnerReference) bool {
	// pointers values cannot be directly compared.
	if (a.Controller == nil && b.Controller != nil) ||
		(a.Controller != nil && b.Controller == nil) ||
		(*a.Controller != *b.Controller) {
		return false
	}
	return a.APIVersion == b.APIVersion && a.Kind == b.Kind && a.Name == b.Name && a.UID == b.UID
}

// ReadOnlyResolutionRequest is an opaque wrapper around ResolutionRequest
// that provides the methods needed to read data from it using the
// Resource interface without exposing the underlying API
// object.
type ReadOnlyResolutionRequest struct {
	req *v1beta1.ResolutionRequest
}

var _ common.ResolvedResource = ReadOnlyResolutionRequest{}

func CrdIntoResource(rr *v1beta1.ResolutionRequest) ReadOnlyResolutionRequest {
	return ReadOnlyResolutionRequest{req: rr}
}

func (r ReadOnlyResolutionRequest) Annotations() map[string]string {
	status := r.req.GetStatus()
	if status != nil && status.Annotations != nil {
		annotationsCopy := map[string]string{}
		for key, val := range status.Annotations {
			annotationsCopy[key] = val
		}
		return annotationsCopy
	}
	return nil
}

func (r ReadOnlyResolutionRequest) Data() ([]byte, error) {
	encodedData := r.req.Status.ResolutionRequestStatusFields.Data
	decodedBytes, err := base64.StdEncoding.Strict().DecodeString(encodedData)
	if err != nil {
		return nil, fmt.Errorf("error decoding data from base64: %w", err)
	}
	return decodedBytes, nil
}

func (r ReadOnlyResolutionRequest) RefSource() *v1.RefSource {
	return r.req.Status.RefSource
}
