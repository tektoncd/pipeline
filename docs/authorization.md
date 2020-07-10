<!--
---
linkTitle: "Authorization"
weight: 1
---
-->

# Authorization

This document explains use of Kubernetes "Rules Based Access Control" 
or ["RBAC"](https://kubernetes.io/docs/reference/access-authn-authz/authorization/), beyond the 
definition of various RBAC objects in the `config` folder of this project.

## Controller Access to ClusterTasks from PipelineRuns and TaskRuns

By default, Tekton allows any `PipelineRun` or `TaskRun` to access any ClusterTask because the 
controller(s) has access to any ClusterTask.  However, if you happened to log into the cluster
using the credentials of the `ServiceAccount` associated with the `PipelineRun` or `TaskRun` and ran

```bash
kubectl  auth can-i get clustertask <cluster task name>
``` 

you will get `false` because by default the `ServiceAccounts` used in `PipelineRuns` and `TaskRuns` are
not granted permission to access the cluster scoped resource `ClusterTask`.  And remember, if no `ServiceAccount` is 
set on the `PipelineRun` or `TaskRun`, the `default` `ServiceAccount` is used by Kubernetes when running the underlying
`Pod`.

If as an administrator, you want to control which `PipelineRun` and `TaskRun` instances, via their `ServiceAccounts`, 
have access to given `ClusterTasks`, consider these steps:

- First, as a convenience, Tekton adds viewing of `ClusterTask` as part of its aggregation of permissions to the default
`view` / `edit` / `admin` `ClusterRoles` present with any Kubernetes cluster.  You can then assign one of those default
`ClusterRoles` to any `ServiceAccount` that should have access to `ClusterTasks`
- If one of those default `ClusterRoles` is too broad, then create more precise `Role` / `RoleBinding` or `ClusterRole` 
/ `ClusterRoleBinding` and assign those to the `ServiceAccounts` in questions.
- Then, an update to the Tekton webhook `Deployment` named `tekton-pipelines-webhook` in the `tekton-pipelines` namespace 
is needed to enable the validating admission webhook that will ensure that the `ServiceAccount` associated with a 
`PipelineRun` or `TaskRun` can read its referenced `ClusterTask`.
- The specific update needed is setting the environment variable `ENABLE_CLUSTER_TASK_ACCESS_VALIDATION` to "true" on
the webhook's underlying `Container`.
- Once set, enforcement that the necessary permissions exist for `ClusterTask` access should start. 

## Pod Security Policy

### Background

From Kubernetes documentation, [using Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/) 
introduces the pluggable authorization facility of Kubernetes. The concept of admission webhooks, with mutating and validating 
specific types, are the substance of admission controllers. 

There is a set of default admission controller “plugins” available for opt-in/opt-out, listed [here](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#what-does-each-admission-controller-do),
that provide a slew of validations.  Of that list, the PodSecurityPolicy, which is introduced [here](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#podsecuritypolicy)
and detailed [here](https://kubernetes.io/docs/concepts/policy/pod-security-policy/), is perhaps the fundamental, 
and controversial, of those default plugins. 


Some key points regarding the PodSecurityPolicy plugin:

- It regulates whether a `Pod` can be created
- It has to be enabled.  It is off by default with Kubernetes out of the box.
- It considers the permissions of **EITHER** the `ServiceAccount` associated with the `Pod`, or the user that invokes
the HTTP requests against the API Server's `Pod` endpoint to create the `Pod` (this user is commonly referred to as 
the "requesting user")
- It maps both the "requesting user" and `ServiceAccount` to whatever `PodSecurityPolicy` objects that exist in 
the Cluster (where RBAC can be defined such that a user can `use` a given `PodSecuirtyPolicy`).
- `PodSecurityPolicy` defines whether certain fields in the `Pod` or underlying `Containers` can be set in certain ways

### Best Practice Recommendations with Tekton

So an exploration of the default configuration provide with Tekton discovers that an example `PodSecurityPolicy` is 
provided [here](https://github.com/tektoncd/pipeline/blob/master/config/101-podsecuritypolicy.yaml).  And it is assigned 
to the Tekton controller [here](https://github.com/tektoncd/pipeline/blob/master/config/200-clusterrole.yaml)

An examination of the `PodSecurityPolicy` shows that it is fairly restrictive:

- It does not allow privileged or escalation to privileged
- It restricts volumes to emptyDir, `ConfigMap`, and `Secret`
- it restricts access to the host networks and namespaces

Now, if you run in a cluster with PodSecurityPolicy or similar plugin enabled, and want to allow escalated permissions
for `Pods` for **SOME, BUT NOT ALL,** users, it is important that you create escalated `PodSecurityPolicy` 
instances and assign them to the `ServiceAccounts` assigned to the `PipelineRun` or `TaskRun`, and **NOT** the Tekton
controllers.

Why?

Because of the whose permissions the PodSecurityPolicy plugin considered when deciding if a `Pod` can be created.  Only
1 of the users considered, either the "requesting user" or `ServiceAccount`, need to have permission.  The Tekton 
controllers are the "requesting user" when a `PipelineRun` or `TaskRun` are converted into a `Pod`.  It is the Tekton
controllers that issue the HTTP request to create the `Pod`.  If it has escalated permissions, that means **ANY** user
in the system that can create `PipelineRun` or `TaskRun` objects can create `Pods` with escalated permissions. 


