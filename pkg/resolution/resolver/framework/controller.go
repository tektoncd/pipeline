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
	"fmt"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	rrclient "github.com/tektoncd/pipeline/pkg/client/resolution/injection/client"
	rrinformer "github.com/tektoncd/pipeline/pkg/client/resolution/injection/informers/resolution/v1beta1/resolutionrequest"
	rrlister "github.com/tektoncd/pipeline/pkg/client/resolution/listers/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/resolution/common"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/clock"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
)

// ReconcilerModifier is a func that can access and modify a reconciler
// in the moments before a resolver is started. It allows for
// things like injecting a test clock.
type ReconcilerModifier = func(reconciler *Reconciler)

// NewController returns a knative controller for a Tekton Resolver.
// This sets up a lot of the boilerplate that individual resolvers
// shouldn't need to be concerned with since it's common to all of them.
func NewController(ctx context.Context, resolver Resolver, modifiers ...ReconcilerModifier) func(context.Context, configmap.Watcher) *controller.Impl {
	if err := validateResolver(ctx, resolver); err != nil {
		panic(err.Error())
	}
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		logger := logging.FromContext(ctx)
		kubeclientset := kubeclient.Get(ctx)
		rrclientset := rrclient.Get(ctx)
		rrInformer := rrinformer.Get(ctx)

		if err := resolver.Initialize(ctx); err != nil {
			panic(err.Error())
		}

		r := &Reconciler{
			LeaderAwareFuncs:           leaderAwareFuncs(rrInformer.Lister()),
			kubeClientSet:              kubeclientset,
			resolutionRequestLister:    rrInformer.Lister(),
			resolutionRequestClientSet: rrclientset,
			resolver:                   resolver,
		}

		watchConfigChanges(ctx, r, cmw)

		// TODO(sbwsg): Do better sanitize.
		resolverName := resolver.GetName(ctx)
		resolverName = strings.ReplaceAll(resolverName, "/", "")
		resolverName = strings.ReplaceAll(resolverName, " ", "")

		applyModifiersAndDefaults(ctx, r, modifiers)

		impl := controller.NewContext(ctx, r, controller.ControllerOptions{
			WorkQueueName: "TektonResolverFramework." + resolverName,
			Logger:        logger,
		})

		rrInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: filterResolutionRequestsBySelector(resolver.GetSelector(ctx)),
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc: impl.Enqueue,
				UpdateFunc: func(oldObj, newObj interface{}) {
					impl.Enqueue(newObj)
				},
				// TODO(sbwsg): should we deliver delete events
				// to the resolver?
				// DeleteFunc: impl.Enqueue,
			},
		})

		return impl
	}
}

func filterResolutionRequestsBySelector(selector map[string]string) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		rr, ok := obj.(*v1beta1.ResolutionRequest)
		if !ok {
			return false
		}
		if len(rr.ObjectMeta.Labels) == 0 {
			return false
		}
		for key, val := range selector {
			lookup, has := rr.ObjectMeta.Labels[key]
			if !has {
				return false
			}
			if lookup != val {
				return false
			}
		}
		return true
	}
}

// TODO(sbwsg): I don't really understand the LeaderAwareness types beyond the
// fact that the controller crashes if they're missing. It looks
// like this is bucketing based on labels. Should we use the filter
// selector from above in the call to lister.List here?
func leaderAwareFuncs(lister rrlister.ResolutionRequestLister) reconciler.LeaderAwareFuncs {
	return reconciler.LeaderAwareFuncs{
		PromoteFunc: func(bkt reconciler.Bucket, enq func(reconciler.Bucket, types.NamespacedName)) error {
			all, err := lister.List(labels.Everything())
			if err != nil {
				return err
			}
			for _, elt := range all {
				enq(bkt, types.NamespacedName{
					Namespace: elt.GetNamespace(),
					Name:      elt.GetName(),
				})
			}
			return nil
		},
	}
}

var (
	// ErrMissingTypeSelector is returned when a resolver does not return
	// a selector with a type label from its GetSelector method.
	ErrMissingTypeSelector = fmt.Errorf("invalid resolver: minimum selector must include %q", common.LabelKeyResolverType)

	// ErrorMissingTypeSelector is an alias to ErrMissingTypeSelector
	//
	// Deprecated: use ErrMissingTypeSelector instead.
	ErrorMissingTypeSelector = ErrMissingTypeSelector
)

func validateResolver(ctx context.Context, r Resolver) error {
	sel := r.GetSelector(ctx)
	if sel == nil {
		return ErrMissingTypeSelector
	}
	if sel[common.LabelKeyResolverType] == "" {
		return ErrMissingTypeSelector
	}
	return nil
}

// watchConfigChanges binds a framework.Resolver to updates on its
// configmap, using knative's configmap helpers. This is only done if
// the resolver implements the framework.ConfigWatcher interface.
func watchConfigChanges(ctx context.Context, reconciler *Reconciler, cmw configmap.Watcher) {
	if configWatcher, ok := reconciler.resolver.(ConfigWatcher); ok {
		logger := logging.FromContext(ctx)
		resolverConfigName := configWatcher.GetConfigName(ctx)
		if resolverConfigName == "" {
			panic("resolver returned empty config name")
		}
		reconciler.configStore = NewConfigStore(resolverConfigName, logger)
		reconciler.configStore.WatchConfigs(cmw)
	}
}

// applyModifiersAndDefaults applies the given modifiers to
// a reconciler and, after doing so, sets any default values for things
// that weren't set by a modifier.
func applyModifiersAndDefaults(ctx context.Context, r *Reconciler, modifiers []ReconcilerModifier) {
	for _, mod := range modifiers {
		mod(r)
	}

	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
}
