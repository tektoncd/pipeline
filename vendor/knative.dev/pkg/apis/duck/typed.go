/*
Copyright 2018 The Knative Authors

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

package duck

import (
	"context"
	"fmt"
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"

	"knative.dev/pkg/apis"
)

// TypedInformerFactory implements InformerFactory such that the elements
// tracked by the informer/lister have the type of the canonical "obj".
type TypedInformerFactory struct {
	Client       dynamic.Interface
	Type         apis.Listable
	ResyncPeriod time.Duration
	StopChannel  <-chan struct{}
}

// Check that TypedInformerFactory implements InformerFactory.
var _ InformerFactory = (*TypedInformerFactory)(nil)

// Get implements InformerFactory.
func (dif *TypedInformerFactory) Get(ctx context.Context, gvr schema.GroupVersionResource) (cache.SharedIndexInformer, cache.GenericLister, error) {
	// Avoid error cases, like the GVR does not exist.
	// It is not a full check. Some RBACs might sneak by, but the window is very small.
	if _, err := dif.Client.Resource(gvr).List(ctx, metav1.ListOptions{}); err != nil {
		return nil, nil, err
	}

	listObj := dif.Type.GetListType()
	lw := &cache.ListWatch{
		ListWithContextFunc:  asStructuredLister(dif.Client.Resource(gvr).List, listObj),
		WatchFuncWithContext: AsStructuredWatcher(dif.Client.Resource(gvr).Watch, dif.Type),
	}
	inf := cache.NewSharedIndexInformer(lw, dif.Type, dif.ResyncPeriod, cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
	})

	lister := cache.NewGenericLister(inf.GetIndexer(), gvr.GroupResource())

	go inf.Run(dif.StopChannel)

	if ok := cache.WaitForCacheSync(dif.StopChannel, inf.HasSynced); !ok {
		return nil, nil, fmt.Errorf("failed starting shared index informer for %v with type %T", gvr, dif.Type)
	}

	return inf, lister, nil
}

type unstructuredLister func(context.Context, metav1.ListOptions) (*unstructured.UnstructuredList, error)

func asStructuredLister(ulist unstructuredLister, listObj runtime.Object) cache.ListWithContextFunc {
	return func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
		ul, err := ulist(ctx, opts)
		if err != nil {
			return nil, err
		}
		res := listObj.DeepCopyObject()
		if err := FromUnstructured(ul, res); err != nil {
			return nil, err
		}
		return res, nil
	}
}

type structuredWatcher func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)

// AsStructuredWatcher is public for testing only.
// TODO(mattmoor): Move tests for this to `package duck` and make private.
func AsStructuredWatcher(wf structuredWatcher, obj runtime.Object) cache.WatchFuncWithContext {
	return func(ctx context.Context, lo metav1.ListOptions) (watch.Interface, error) {
		uw, err := wf(ctx, lo)
		if err != nil {
			return nil, err
		}
		structuredCh := make(chan watch.Event)
		go func() {
			defer close(structuredCh)
			unstructuredCh := uw.ResultChan()
			for ue := range unstructuredCh {
				unstructuredObj, ok := ue.Object.(*unstructured.Unstructured)
				if !ok {
					// If it isn't an unstructured object, then forward the
					// event as-is.  This is likely to happen when the event's
					// Type is an Error.
					structuredCh <- ue
					continue
				}
				structuredObj := obj.DeepCopyObject()

				err := FromUnstructured(unstructuredObj, structuredObj)
				if err != nil {
					// Pass back an error indicating that the object we got
					// was invalid.
					structuredCh <- watch.Event{
						Type: watch.Error,
						Object: &metav1.Status{
							Status:  metav1.StatusFailure,
							Code:    http.StatusUnprocessableEntity,
							Reason:  metav1.StatusReasonInvalid,
							Message: err.Error(),
						},
					}
					continue
				}
				// Send the structured event.
				structuredCh <- watch.Event{
					Type:   ue.Type,
					Object: structuredObj,
				}
			}
		}()

		return watch.NewProxyWatcher(structuredCh), nil
	}
}
