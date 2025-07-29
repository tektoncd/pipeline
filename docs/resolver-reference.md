<!--
---
linkTitle: "Resolver Reference"
weight: 101
---
-->

# Resolver Reference

Writing a resolver is made easier with the
`github.com/tektoncd/pipeline/pkg/resolution/resolver/framework` package.
This package exposes a number of interfaces that let your code control
what kind of behaviour it should have when running.

To get started really quickly see the [resolver
template](./resolver-template/), or for a howto guide see [how to write
a resolver](./how-to-write-a-resolver.md).

## The `Resolver` Interface

Implementing this interface is required. It provides just enough
configuration for the framework to get a resolver running.

{{% tabs %}}

{{% tab "Upgraded Framework" %}}

| Method  to Implement | Description |
|----------------------|-------------|
| Initialize | Use this method to perform any setup required before the resolver starts receiving requests. |
| GetName | Use this method to return a name to refer to your Resolver by. e.g. `"Git"` |
| GetSelector | Use this method to specify the labels that a resolution request must have to be routed to your resolver. |
| Validate | Use this method to validate the resolution Spec given to your resolver. |
| Resolve | Use this method to perform get the resource based on the ResolutionRequestSpec as input and return it, along with any metadata about it in annotations. Errors returned by this method mark the ResolutionRequest as Failed, unless the error type is considered transient (e.g. a Kubernetes request timeout or etcd leader changes). |

{{% /tab %}}

{{% tab "Previous Framework (Deprecated)" %}}

| Method  to Implement | Description |
|----------------------|-------------|
| Initialize | Use this method to perform any setup required before the resolver starts receiving requests. |
| GetName | Use this method to return a name to refer to your Resolver by. e.g. `"Git"` |
| GetSelector | Use this method to specify the labels that a resolution request must have to be routed to your resolver. |
| ValidateParams | Use this method to validate the params given to your resolver. |
| Resolve | Use this method to perform get the resource based on params as input and return it, along with any metadata about it in annotations |

{{% /tab %}}

{{% /tabs %}}

## The `ConfigWatcher` Interface

Implement this optional interface if your Resolver requires some amount
of admin configuration. For example, if you want to allow admin users to
configure things like timeouts, namespaces, lists of allowed registries,
api endpoints or base urls, service account names to use, etc...

| Method to Implement | Description |
|---------------------|-------------|
| GetConfigName       | Use this method to return the name of the configmap admins will use to configure this resolver. Once this interface is implemented your `Validate` and `Resolve` methods will be able to access your latest resolver configuration by calling `framework.GetResolverConfigFromContext(ctx)`. Note that this configmap must exist when your resolver starts - put a default one in your resolver's `config/` directory. |

## The `TimedResolution` Interface

Implement this optional interface if your Resolver needs to custimze the
timeout a resolution request can take. This may be based on knowledge of
the underlying storage (e.g. some git repositories are slower to clone
than others) or might be something an admin configures with a configmap.

The default timeout of a request is 1 minute if this interface is not
implemented. **Note**: There is currently a global maximum timeout of 1
minute for _all_ resolution requests to prevent zombie requests
remaining in an incomplete state forever.

| Method to Implement | Description |
|---------------------|-------------|
| GetResolutionTimeout | Return a custom timeout duration from this method to control how long a resolution request to this resolver may take. |
