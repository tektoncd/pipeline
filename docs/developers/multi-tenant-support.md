# Support for running in multi-tenant configuration

Multi-tenancy for Tekton is intended as running multiple instances of the
Tekton control plane in a single Kubernetes cluster, with each control plane
responsible for reconciliation of the Tekton resources in a single namespace.

This type of setup involves various aspects of Tekton that are normally
cluster-wide or associated with namespaces:

- Namespaces
- Custom Resource Definitions (CRDs)
- RBAC (Service Accounts, Roles and Bindings)
- Validation and Mutation Webhooks
- Conversion Webhook

## Namespaces

The Tekton control plance namespace resource is defined in a dedicated
[sub-folder](/config/100-namespace/),  folder that includes
all Tekton resources. When installing from source, the namespace folder can be
skipped, and the namespace can be created beforehand instead.

To customize the namespace where the control plan is installed, all namespaced
resources under the [`config`](/config/) folder (or in the release file) must be updated.
That can be achieved by search and replace, or using tools like `ko`, `sed` or `kustomize`.

More details for the installation from source are available in the
[root development document](/DEVELOPMENT.md#installing-into-custom-namespaces).

The Tekton control plane by default watches all namespaces for Tekton resources.
To change this behaviour, and limit the Tekton controller to a single namespace,
add the `--namespace <namespace-name>` command line flag to the Tekton controller
[deployment file](/config/controller.yaml).

## Custom Resource Definitions

Custom Resource Definitions are defined in a dedicated [sub-folder](/config/300-crds/),
under the [`config`](/config/) folder that includes all Tekton resources.

When installing from source, instead of applying resources recursively, apply only
the content of the [`config`](/config/) folder, for instance:

```sh
ko apply -f config
```

Note that the namespace will have to be created beforehand.
When installing from a release file, the [`yq(https://github.com/mikefarah/yq) tool
comes in handy:

```sh
yq  e '. | select(.kind != "CustomResourceDefinition")' < release.yaml > release-nocrd.yaml
```

## RBAC

In order to support potential multi-tenant configurations the roles of the
controller are split into two:

    `tekton-pipelines-controller-cluster-access`: those permissions needed cluster-wide by the controller.
    `tekton-pipelines-controller-tenant-access`: those permissions needed on a namespace-by-namespace basis.

By default the roles are cluster-scoped for backwards-compatibility and
ease-of-use. If you want to start running a multi-tenant service you are able to
bind `tekton-pipelines-controller-tenant-access` using a `RoleBinding` instead
of a `ClusterRoleBinding`, thereby limiting the access that the controller has
to specific tenant namespaces.

## Validation and Mutation Webhooks

Validation and mutation webhooks configurations by default act with cluster scope.
Normally only instance of the webhooks per cluster is enough to serve all instances
of Tekton installed.

If different versions of Tekton must coexist in the same cluster, they might require
different webhooks. Webhook configurations can be namespaced by using the
[`namespaceSelector`](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#matching-requests-namespaceselector).

The name of the [configurations](/config/webhook.yaml) must also be adapted so that
many instances may coexist, one per namespace.

Note that running multiple versions of Tekton in a single cluster might work in
certain cases, but it's not officially supported.

## Conversion Webhook

Kubernets only allows one specific version of all resources associated with a specific
CRD being stored in etcd. For instance, all `Tasks` will be stored as `v1`, even if
both `v1` and `v1beta1` are served by the API.

The conversion webhook converts versions on the fly when a version other than the stored
one is used. Tekton's conversion webhook is responsible for reconciling the CRD
definitions. Since those are defined at cluster level, only once conversion webhook may
exist in the cluster.

The conversion webhook is bundled in a single binary with the other webhooks, so today
it's not possible to run multiple validation and mutation webhooks and a single
conversion webhook. [TEP-0129](https://github.com/tektoncd/community/blob/main/teps/0129-multiple-tekton-instances-per-cluster.md)
will change that, once implemented.
