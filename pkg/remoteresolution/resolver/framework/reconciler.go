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
	rrcache "github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework/cache"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/clock"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
)

// defaultMaximumResolutionDuration is the maximum amount of time
// resolution may take.

// defaultMaximumResolutionDuration is the max time that a call to
// Resolve() may take. It can be overridden by a resolver implementing
// the framework.TimedResolution interface.
const defaultMaximumResolutionDuration = time.Minute

// TracerName is the name of the tracer used by the resolver framework reconciler.
const TracerName = "ResolverFrameworkReconciler"

// statusDataPatch is the json structure that will be PATCHed into
// a ResolutionRequest with its data and annotations once successfully
// resolved.
type statusDataPatch struct {
	Annotations map[string]string             `json:"annotations"`
	Data        string                        `json:"data"`
	Source      *pipelinev1beta1.ConfigSource `json:"source"`
	RefSource   *pipelinev1.RefSource         `json:"refSource"`
}

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

	configStore    *framework.ConfigStore
	tracerProvider trace.TracerProvider
}

var _ reconciler.LeaderAware = &Reconciler{}

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

	ctx, span := r.tracerProvider.Tracer(TracerName).Start(ctx, "Reconcile")
	defer span.End()
	span.SetAttributes(
		attribute.String("resolver.type", r.resolver.GetName(ctx)),
		attribute.String("resolutionrequest.name", name),
		attribute.String("resolutionrequest.namespace", namespace),
	)

	// Inject request-scoped information into the context, such as
	// the namespace that the request originates from and the
	// configuration from the configmap this resolver is watching.
	ctx = resolutioncommon.InjectRequestNamespace(ctx, namespace)
	ctx = resolutioncommon.InjectRequestName(ctx, name)
	if r.configStore != nil {
		ctx = r.configStore.ToContext(ctx)
	}

	if resolveErr := r.resolve(ctx, key, rr); resolveErr != nil {
		span.RecordError(resolveErr)
		span.SetStatus(codes.Error, resolveErr.Error())
		return resolveErr
	}
	return nil
}

func (r *Reconciler) resolve(ctx context.Context, key string, rr *v1beta1.ResolutionRequest) error {
	ctx, span := r.tracerProvider.Tracer(TracerName).Start(ctx, "resolve")
	defer span.End()

	errChan := make(chan error)
	resourceChan := make(chan framework.ResolvedResource)

	paramsMap := make(map[string]string)
	for _, p := range rr.Spec.Params {
		paramsMap[p.Name] = p.Value.StringVal
	}

	// Centralized cache parameter validation for all resolvers
	if cacheMode, exists := paramsMap[rrcache.CacheParam]; exists && cacheMode != "" {
		if err := rrcache.Validate(cacheMode); err != nil {
			return &resolutioncommon.InvalidRequestError{
				ResolutionRequestKey: key,
				Message:              err.Error(),
			}
		}
	}

	timeoutDuration := defaultMaximumResolutionDuration
	if timed, ok := r.resolver.(framework.TimedResolution); ok {
		var err error
		timeoutDuration, err = timed.GetResolutionTimeout(ctx, defaultMaximumResolutionDuration, paramsMap)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err
		}
	}

	// A new context is created for resolution so that timeouts can
	// be enforced without affecting other uses of ctx (e.g. sending
	// Updates to ResolutionRequest objects).
	resolutionCtx, cancelFn := context.WithTimeout(ctx, timeoutDuration)
	defer cancelFn()

	resolverName := r.resolver.GetName(resolutionCtx)
	tracer := r.tracerProvider.Tracer(TracerName)

	go func() {
		validateCtx, validateSpan := tracer.Start(resolutionCtx, "validate")
		validateSpan.SetAttributes(attribute.String("resolver.type", resolverName))
		validationError := r.resolver.Validate(validateCtx, &rr.Spec)
		if validationError != nil {
			validateSpan.RecordError(validationError)
			validateSpan.SetStatus(codes.Error, validationError.Error())
			validateSpan.End()
			errChan <- &resolutioncommon.InvalidRequestError{
				ResolutionRequestKey: key,
				Message:              validationError.Error(),
			}
			return
		}
		validateSpan.End()

		resolveCtx, resolveSpan := tracer.Start(resolutionCtx, "resolverResolve")
		resolveSpan.SetAttributes(attribute.String("resolver.type", resolverName))
		resource, resolveErr := r.resolver.Resolve(resolveCtx, &rr.Spec)
		if resolveErr != nil {
			resolveSpan.RecordError(resolveErr)
			resolveSpan.SetStatus(codes.Error, resolveErr.Error())
			resolveSpan.End()
			errChan <- &resolutioncommon.GetResourceError{
				ResolverName: resolverName,
				Key:          key,
				Original:     resolveErr,
			}
			return
		}
		if err := framework.ValidateResolvedResource(resource); err != nil {
			wrappedErr := fmt.Errorf("resolved resource validation error: %w", err)
			resolveSpan.RecordError(wrappedErr)
			resolveSpan.SetStatus(codes.Error, wrappedErr.Error())
			resolveSpan.End()
			errChan <- &resolutioncommon.GetResourceError{
				ResolverName: resolverName,
				Key:          key,
				Original:     wrappedErr,
			}
			return
		}
		resolveSpan.End()
		resourceChan <- resource
	}()

	select {
	case err := <-errChan:
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return r.OnError(ctx, rr, err)
		}
	case <-resolutionCtx.Done():
		if err := resolutionCtx.Err(); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
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
	if resolutioncommon.IsErrTransient(err) {
		return err
	}
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
	ctx, span := r.tracerProvider.Tracer(TracerName).Start(ctx, "MarkFailed")
	defer span.End()

	key := fmt.Sprintf("%s/%s", rr.Namespace, rr.Name)
	reason, resolutionErr := resolutioncommon.ReasonError(resolutionErr)
	latestGeneration, err := r.resolutionRequestClientSet.ResolutionV1beta1().ResolutionRequests(rr.Namespace).Get(ctx, rr.Name, metav1.GetOptions{})
	if err != nil {
		logging.FromContext(ctx).Warnf("error getting latest generation of resolutionrequest %q: %v", key, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	if latestGeneration.IsDone() {
		return nil
	}
	latestGeneration.Status.MarkFailed(reason, resolutionErr.Error())
	_, err = r.resolutionRequestClientSet.ResolutionV1beta1().ResolutionRequests(rr.Namespace).UpdateStatus(ctx, latestGeneration, metav1.UpdateOptions{})
	if err != nil {
		logging.FromContext(ctx).Warnf("error marking resolutionrequest %q as failed: %v", key, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (r *Reconciler) writeResolvedData(ctx context.Context, rr *v1beta1.ResolutionRequest, resource framework.ResolvedResource) error {
	ctx, span := r.tracerProvider.Tracer(TracerName).Start(ctx, "writeResolvedData")
	defer span.End()

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
		logging.FromContext(ctx).Warnf("writeResolvedData error serializing resource request patch for resolution request %s:%s: %s", rr.Namespace, rr.Name, err.Error())
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return r.OnError(ctx, rr, &resolutioncommon.UpdatingRequestError{
			ResolutionRequestKey: fmt.Sprintf("%s/%s", rr.Namespace, rr.Name),
			Original:             fmt.Errorf("error serializing resource request patch: %w", err),
		})
	}
	_, err = r.resolutionRequestClientSet.ResolutionV1beta1().ResolutionRequests(rr.Namespace).Patch(ctx, rr.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		logging.FromContext(ctx).Warnf("writeResolvedData error patching resolution request %s:%s: %s", rr.Namespace, rr.Name, err.Error())
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return r.OnError(ctx, rr, &resolutioncommon.UpdatingRequestError{
			ResolutionRequestKey: fmt.Sprintf("%s/%s", rr.Namespace, rr.Name),
			Original:             err,
		})
	}

	return nil
}
