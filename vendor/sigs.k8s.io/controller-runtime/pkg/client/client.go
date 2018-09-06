/*
Copyright 2018 The Kubernetes Authors.

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

package client

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// Options are creation options for a Client
type Options struct {
	// Scheme, if provided, will be used to map go structs to GroupVersionKinds
	Scheme *runtime.Scheme

	// Mapper, if provided, will be used to map GroupVersionKinds to Resources
	Mapper meta.RESTMapper
}

// New returns a new Client using the provided config and Options.
func New(config *rest.Config, options Options) (Client, error) {
	if config == nil {
		return nil, fmt.Errorf("must provide non-nil rest.Config to client.New")
	}

	// Init a scheme if none provided
	if options.Scheme == nil {
		options.Scheme = scheme.Scheme
	}

	// Init a Mapper if none provided
	if options.Mapper == nil {
		var err error
		options.Mapper, err = apiutil.NewDiscoveryRESTMapper(config)
		if err != nil {
			return nil, err
		}
	}

	c := &client{
		cache: clientCache{
			config:         config,
			scheme:         options.Scheme,
			mapper:         options.Mapper,
			codecs:         serializer.NewCodecFactory(options.Scheme),
			resourceByType: make(map[reflect.Type]*resourceMeta),
		},
		paramCodec: runtime.NewParameterCodec(options.Scheme),
	}

	return c, nil
}

var _ Client = &client{}

// client is a client.Client that reads and writes directly from/to an API server.  It lazily initializes
// new clients at the time they are used, and caches the client.
type client struct {
	cache      clientCache
	paramCodec runtime.ParameterCodec
}

// Create implements client.Client
func (c *client) Create(ctx context.Context, obj runtime.Object) error {
	o, err := c.cache.getObjMeta(obj)
	if err != nil {
		return err
	}
	return o.Post().
		NamespaceIfScoped(o.GetNamespace(), o.isNamespaced()).
		Resource(o.resource()).
		Body(obj).
		Do().
		Into(obj)
}

// Update implements client.Client
func (c *client) Update(ctx context.Context, obj runtime.Object) error {
	o, err := c.cache.getObjMeta(obj)
	if err != nil {
		return err
	}
	return o.Put().
		NamespaceIfScoped(o.GetNamespace(), o.isNamespaced()).
		Resource(o.resource()).
		Name(o.GetName()).
		Body(obj).
		Do().
		Into(obj)
}

// Delete implements client.Client
func (c *client) Delete(ctx context.Context, obj runtime.Object) error {
	o, err := c.cache.getObjMeta(obj)
	if err != nil {
		return err
	}
	return o.Delete().
		NamespaceIfScoped(o.GetNamespace(), o.isNamespaced()).
		Resource(o.resource()).
		Name(o.GetName()).
		Do().
		Error()
}

// Get implements client.Client
func (c *client) Get(ctx context.Context, key ObjectKey, obj runtime.Object) error {
	r, err := c.cache.getResource(obj)
	if err != nil {
		return err
	}
	return r.Get().
		NamespaceIfScoped(key.Namespace, r.isNamespaced()).
		Resource(r.resource()).
		Name(key.Name).Do().Into(obj)
}

// List implements client.Client
func (c *client) List(ctx context.Context, opts *ListOptions, obj runtime.Object) error {
	r, err := c.cache.getResource(obj)
	if err != nil {
		return err
	}
	namespace := ""
	if opts != nil {
		namespace = opts.Namespace
	}
	return r.Get().
		NamespaceIfScoped(namespace, r.isNamespaced()).
		Resource(r.resource()).
		Body(obj).
		VersionedParams(opts.AsListOptions(), c.paramCodec).
		Do().
		Into(obj)
}

// Status implements client.StatusClient
func (c *client) Status() StatusWriter {
	return &statusWriter{client: c}
}

// statusWriter is client.StatusWriter that writes status subresource
type statusWriter struct {
	client *client
}

// ensure statusWriter implements client.StatusWriter
var _ StatusWriter = &statusWriter{}

// Update implements client.StatusWriter
func (sw *statusWriter) Update(_ context.Context, obj runtime.Object) error {
	o, err := sw.client.cache.getObjMeta(obj)
	if err != nil {
		return err
	}
	// TODO(droot): examine the returned error and check if it error needs to be
	// wrapped to improve the UX ?
	// It will be nice to receive an error saying the object doesn't implement
	// status subresource and check CRD definition
	return o.Put().
		NamespaceIfScoped(o.GetNamespace(), o.isNamespaced()).
		Resource(o.resource()).
		Name(o.GetName()).
		SubResource("status").
		Body(obj).
		Do().
		Into(obj)
}
