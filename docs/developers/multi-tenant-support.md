# Support for running in multi-tenant configuration

In order to support potential multi-tenant configurations the roles of the
controller are split into two:

    `tekton-pipelines-controller-cluster-access`: those permissions needed cluster-wide by the controller.
    `tekton-pipelines-controller-tenant-access`: those permissions needed on a namespace-by-namespace basis.

By default the roles are cluster-scoped for backwards-compatibility and
ease-of-use. If you want to start running a multi-tenant service you are able to
bind `tekton-pipelines-controller-tenant-access` using a `RoleBinding` instead
of a `ClusterRoleBinding`, thereby limiting the access that the controller has
to specific tenant namespaces.