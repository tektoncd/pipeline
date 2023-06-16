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

package framework

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	pipelinev1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	rrclient "github.com/tektoncd/pipeline/pkg/client/resolution/clientset/versioned"
	rrv1beta1 "github.com/tektoncd/pipeline/pkg/client/resolution/listers/resolution/v1beta1"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/clock"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
)

// Reconciler handles ResolutionRequest objects, performs functionality
// common to all resolvers and delegates resolver-specific actions
// to its embedded type-specific Resolver object.
type Reconciler struct {
	// Implements reconciler.LeaderAware
	reconciler.LeaderAwareFuncs

	// Clock is used by the reconciler to track the passage of time
	// and can be overridden for tests.
	Clock clock.PassiveClock

	resolver                   Resolver
	kubeClientSet              kubernetes.Interface
	resolutionRequestLister    rrv1beta1.ResolutionRequestLister
	resolutionRequestClientSet rrclient.Interface

	configStore *ConfigStore
}

var _ reconciler.LeaderAware = &Reconciler{}

// defaultMaximumResolutionDuration is the maximum amount of time
// resolution may take.

// defaultMaximumResolutionDuration is the max time that a call to
// Resolve() may take. It can be overridden by a resolver implementing
// the framework.TimedResolution interface.
const defaultMaximumResolutionDuration = time.Minute

// Reconcile receives the string key of a ResolutionRequest object, looks
// it up, checks it for common errors, and then delegates
// resolver-specific functionality to the reconciler's embedded
// type-specific resolver. Any errors that occur during validation or
// resolution are handled by updating or failing the ResolutionRequest.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		err = &resolutioncommon.InvalidResourceKeyError{Key: key, Original: err}
		return controller.NewPermanentError(err)
	}

	rr, err := r.resolutionRequestLister.ResolutionRequests(namespace).Get(name)
	if err != nil {
		err := &resolutioncommon.GetResourceError{ResolverName: "resolutionrequest", Key: key, Original: err}
		return controller.NewPermanentError(err)
	}

	if rr.IsDone() {
		return nil
	}

	// Inject request-scoped information into the context, such as
	// the namespace that the request originates from and the
	// configuration from the configmap this resolver is watching.
	ctx = resolutioncommon.InjectRequestNamespace(ctx, namespace)
	ctx = resolutioncommon.InjectRequestName(ctx, name)
	if r.configStore != nil {
		ctx = r.configStore.ToContext(ctx)
	}

	return r.resolve(ctx, key, rr)
}

func (r *Reconciler) resolve(ctx context.Context, key string, rr *v1beta1.ResolutionRequest) error {
	errChan := make(chan error)
	resourceChan := make(chan ResolvedResource)

	timeoutDuration := defaultMaximumResolutionDuration
	if timed, ok := r.resolver.(TimedResolution); ok {
		timeoutDuration = timed.GetResolutionTimeout(ctx, defaultMaximumResolutionDuration)
	}

	// A new context is created for resolution so that timeouts can
	// be enforced without affecting other uses of ctx (e.g. sending
	// Updates to ResolutionRequest objects).
	resolutionCtx, cancelFn := context.WithTimeout(ctx, timeoutDuration)
	defer cancelFn()

	go func() {
		validationError := r.resolver.ValidateParams(resolutionCtx, rr.Spec.Params)
		if validationError != nil {
			errChan <- &resolutioncommon.InvalidRequestError{
				ResolutionRequestKey: key,
				Message:              validationError.Error(),
			}
			return
		}
		resource, resolveErr := r.resolver.Resolve(resolutionCtx, rr.Spec.Params)
		if resolveErr != nil {
			errChan <- &resolutioncommon.GetResourceError{
				ResolverName: r.resolver.GetName(resolutionCtx),
				Key:          key,
				Original:     resolveErr,
			}
			return
		}
		resourceChan <- resource
	}()

	select {
	case err := <-errChan:
		if err != nil {
			return r.OnError(ctx, rr, err)
		}
	case <-resolutionCtx.Done():
		if err := resolutionCtx.Err(); err != nil {
			return r.OnError(ctx, rr, err)
		}
	case resource := <-resourceChan:
		return r.writeResolvedData(ctx, rr, resource)
	}

	return errors.New("unknown error")
}

// OnError is used to handle any situation where a ResolutionRequest has
// reached a terminal situation that cannot be recovered from.
func (r *Reconciler) OnError(ctx context.Context, rr *v1beta1.ResolutionRequest, err error) error {
	if rr == nil {
		return controller.NewPermanentError(err)
	}
	if err != nil {
		_ = r.MarkFailed(ctx, rr, err)
		return controller.NewPermanentError(err)
	}
	return nil
}

// MarkFailed updates a ResolutionRequest as having failed. It returns
// errors that occur during the update process or nil if the update
// appeared to succeed.
func (r *Reconciler) MarkFailed(ctx context.Context, rr *v1beta1.ResolutionRequest, resolutionErr error) error {
	key := fmt.Sprintf("%s/%s", rr.Namespace, rr.Name)
	reason, resolutionErr := resolutioncommon.ReasonError(resolutionErr)
	latestGeneration, err := r.resolutionRequestClientSet.ResolutionV1beta1().ResolutionRequests(rr.Namespace).Get(ctx, rr.Name, metav1.GetOptions{})
	if err != nil {
		logging.FromContext(ctx).Warnf("error getting latest generation of resolutionrequest %q: %v", key, err)
		return err
	}
	if latestGeneration.IsDone() {
		return nil
	}
	latestGeneration.Status.MarkFailed(reason, resolutionErr.Error())
	_, err = r.resolutionRequestClientSet.ResolutionV1beta1().ResolutionRequests(rr.Namespace).UpdateStatus(ctx, latestGeneration, metav1.UpdateOptions{})
	if err != nil {
		logging.FromContext(ctx).Warnf("error marking resolutionrequest %q as failed: %v", key, err)
		return err
	}
	return nil
}

// statusDataPatch is the json structure that will be PATCHed into
// a ResolutionRequest with its data and annotations once successfully
// resolved.
type statusDataPatch struct {
	Annotations map[string]string             `json:"annotations"`
	Data        string                        `json:"data"`
	Source      *pipelinev1beta1.ConfigSource `json:"source"`
	RefSource   *pipelinev1.RefSource         `json:"refSource"`
}

func (r *Reconciler) writeResolvedData(ctx context.Context, rr *v1beta1.ResolutionRequest, resource ResolvedResource) error {
	encodedData := base64.StdEncoding.Strict().EncodeToString(resource.Data())
	patchBytes, err := json.Marshal(map[string]statusDataPatch{
		"status": {
			Data:        encodedData,
			Annotations: resource.Annotations(),
			RefSource:   resource.RefSource(),
			Source:      (*pipelinev1beta1.ConfigSource)(resource.RefSource()),
		},
	})
	if err != nil {
		return r.OnError(ctx, rr, &resolutioncommon.UpdatingRequestError{
			ResolutionRequestKey: fmt.Sprintf("%s/%s", rr.Namespace, rr.Name),
			Original:             fmt.Errorf("error serializing resource request patch: %w", err),
		})
	}
	_, err = r.resolutionRequestClientSet.ResolutionV1beta1().ResolutionRequests(rr.Namespace).Patch(ctx, rr.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		return r.OnError(ctx, rr, &resolutioncommon.UpdatingRequestError{
			ResolutionRequestKey: fmt.Sprintf("%s/%s", rr.Namespace, rr.Name),
			Original:             err,
		})
	}

	return nil
}
