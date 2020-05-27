<!--
---
linkTitle: "Resources labelling"
weight: 10
---
-->
# Resources labelling

Tekton applications sometimes need to find resources belonging to other applications (dashboard
needs to talk to pipelines and triggers, triggers needs to talk to pipelines, etc.).

In order to facilitate identifying resources that belong to an application, the following
[recommended labels](https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/) are added to every resource.

In the end, resource labels are used for two things:
- Identify the application and/or component resources belong to
- Allow looking up the resource when necessary

The labels set should not cover more than what is needed, for example, if the resource is not to be looked up, it's probably not necessary to set the `app.kubernetes.io/name` label.

## Supported labels

- `app.kubernetes.io/part-of`: defines the application the resource belongs to (`tekton-pipelines`, `tekton-dashboard`, `tekton-triggers`, etc).
- `app.kubernetes.io/component`: defines the component the resource belongs to (`controller`, `webhook`). This label is not always present, an application may not be made of multiple components or the resource may not belong to a specific component of the application.
This label should be present only if the resource belongs to a given component (some resources are )
- `app.kubernetes.io/instance`: defines the instance of the application the resource belongs to. This label should always be set. It is particularily useful when multiple instances of an application are deployed in the same cluster/namespace.
- `app.kubernetes.io/name`: this label should uniquely identify a resource when combined with `app.kubernetes.io/part-of`, `app.kubernetes.io/component` and `app.kubernetes.io/instance` labels. Some resources don't need to be looked up, in this case the `app.kubernetes.io/name` label is not necessary.
- `app.kubernetes.io/version`: defines the version for the resource.

## Looking up resources

Looking up a given resource should not rely on resource names but use the previously documented labels.

For example, if Tekton Pipelines was deployed in the `tekton-pipelines` namespace, and you need to lookup the webhook service.
This should be done by looking up the `Service` in the `tekton-pipelines` namespace that matches the following labels:

| label | value |
| --- | --- |
| `app.kubernetes.io/part-of` | `tekton-pipelines` |
| `app.kubernetes.io/component` | `webhook` |
| `app.kubernetes.io/instance` | `default` |
| `app.kubernetes.io/name` | `webhook` |

Equivalent `kubectl` command:
```bash
kubectl --namespace=tekton-pipelines get svc -l "app.kubernetes.io/part-of=tekton-pipelines,app.kubernetes.io/component=webhook,app.kubernetes.io/instance=default,app.kubernetes.io/name=webhook
```

See below for the list of applications, components, and names for most commonly used resources.

## Reference table

The table below lists applications, components, and names for most commonly used resources.

| resource | type | app.kubernetes.io/part-of | app.kubernetes.io/component | app.kubernetes.io/name |
| --- | --- | --- | --- | --- |
| pipelines controller deployment | `Deployment` | `tekton-pipelines` | `controller` | `controller` |
| pipelines controller service | `Service` | `tekton-pipelines` | `controller` | `controller` |
| pipelines webhook service | `Service` | `tekton-pipelines` | `webhook` | `webhook` |
| triggers controller deployment | `Deployment` | `tekton-triggers` | `controller` | `controller` |
| triggers controller service | `Service` | `tekton-triggers` | `controller` | `controller` |
| triggers webhook service | `Service` | `tekton-triggers` | `webhook` | `webhook` |
| dashboard controller deployment | `Deployment` | `tekton-dashboard` | `dashboard` | `dashboard` |
| dashboard controller service | `Service` | `tekton-dashboard` | `dashboard` | `dashboard` |

**NOTE**: the `app.kubernetes.io/instance` label is set to `default` when deployed with yaml manifests provided in the releases.
