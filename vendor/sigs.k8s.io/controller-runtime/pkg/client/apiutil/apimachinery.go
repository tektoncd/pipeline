package apiutil

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// NewDiscoveryRESTMapper constructs a new RESTMapper based on discovery
// information fetched by a new client with the given config.
func NewDiscoveryRESTMapper(c *rest.Config) (meta.RESTMapper, error) {
	// Get a mapper
	dc := discovery.NewDiscoveryClientForConfigOrDie(c)
	gr, err := discovery.GetAPIGroupResources(dc)
	if err != nil {
		return nil, err
	}
	return discovery.NewRESTMapper(gr, dynamic.VersionInterfaces), nil
}

// GVKForObject finds the GroupVersionKind associated with the given object, if there is only a single such GVK.
func GVKForObject(obj runtime.Object, scheme *runtime.Scheme) (schema.GroupVersionKind, error) {
	gvks, isUnversioned, err := scheme.ObjectKinds(obj)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	if isUnversioned {
		return schema.GroupVersionKind{}, fmt.Errorf("cannot create a new informer for the unversioned type %T", obj)
	}

	if len(gvks) < 1 {
		return schema.GroupVersionKind{}, fmt.Errorf("no group-version-kinds associated with type %T", obj)
	}
	if len(gvks) > 1 {
		// this should only trigger for things like metav1.XYZ --
		// normal versioned types should be fine
		return schema.GroupVersionKind{}, fmt.Errorf(
			"multiple group-version-kinds associated with type %T, refusing to guess at one", obj)
	}
	return gvks[0], nil
}

// RESTClientForGVK constructs a new rest.Interface capable of accessing the resource associated
// with the given GroupVersionKind.
func RESTClientForGVK(gvk schema.GroupVersionKind, baseConfig *rest.Config, codecs serializer.CodecFactory) (rest.Interface, error) {
	gv := gvk.GroupVersion()

	cfg := rest.CopyConfig(baseConfig)
	cfg.GroupVersion = &gv
	if gvk.Group == "" {
		cfg.APIPath = "/api"
	} else {
		cfg.APIPath = "/apis"
	}
	cfg.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: codecs}
	if cfg.UserAgent == "" {
		cfg.UserAgent = rest.DefaultKubernetesUserAgent()
	}
	return rest.RESTClientFor(cfg)
}
