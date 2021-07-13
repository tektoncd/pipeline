# Tekton Pipelines API Specification

<!-- toc -->
- [Abstract](#abstract)
- [Background](#background)
- [Modifying This Specification](#modifying-this-specification)
- [Resource Overview](#resource-overview)
  * [`TaskRun`](#-taskrun-)
- [Detailed Resources - v1beta1](#detailed-resources---v1beta1)
  * [`TaskRun`](#-taskrun--1)
    + [Metadata](#metadata)
    + [Spec](#spec)
    + [Status](#status)
- [Status Signalling](#status-signalling)
- [Listing Resources](#listing-resources)
- [Detailed Resource Types - v1beta1](#detailed-resource-types---v1beta1)
  * [`ArrayOrString`](#arrayorstring)
  * [`ContainerStateRunning`](#containerstaterunning)
  * [`ContainerStateWaiting`](#containerstatewaiting)
  * [`ContainerStateTerminated`](#containerstateterminated)
  * [`EnvVar`](#envvar)
  * [`Param`](#param)
  * [`ParamSpec`](#paramspec)
  * [`Step`](#step)
  * [`StepState`](#stepstate)
  * [`TaskResult`](#taskresult)
  * [`TaskRunResult`](#taskrunresult)
  * [`TaskSpec`](#taskspec)
  * [`WorkspaceBinding`](#workspacebinding)
  * [`WorkspaceDeclaration`](#workspacedeclaration)
<!-- /toc -->

## Abstract

The Tekton Pipelines platform provides common abstractions for describing and executing container-based, run-to-completion workflows, typipcally in service of CI/CD scenarios. This document describes the structure, lifecycle and management of Tekton resources in the context of the [Kubernetes Resource Model](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/architecture/resource-management.md). An understanding of the Kubernetes API interface and the capabilities of [Kubernetes Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) is assumed.

This document does not define the [runtime contract](https://tekton.dev/docs/pipelines/container-contract/) nor prescribe specific implementations of supporting services such as access control, observability, or resource management.

This document makes reference in a few places to different profiles for Tekton installations. A profile in this context is a set of operations, resources, and fields that are accessible to a developer interacting with a Tekton installation. Currently, only a single (minimal) profile for Tekton Pipelines is defined, but additional profiles may be defined in the future to standardize advanced functionality. A minimal profile is one that implements all of the “MUST”, “MUST NOT”, and “REQUIRED” conditions of this document.

## Background

The key words “MUST”, “MUST NOT”, “REQUIRED”, “SHALL”, “SHALL NOT”, “SHOULD”, “SHOULD NOT”, “RECOMMENDED”, “NOT RECOMMENDED”, “MAY”, and “OPTIONAL” are to be interpreted as described in [RFC 2119](https://tools.ietf.org/html/rfc2119).

There is no formal specification of the Kubernetes API and Resource Model. This document assumes Kubernetes 1.18 behavior; this behavior will typically be supported by many future Kubernetes versions. Additionally, this document may reference specific core Kubernetes resources; these references may be illustrative (i.e. an implementation on Kubernetes) or descriptive (i.e. this Kubernetes resource MUST be exposed). References to these core Kubernetes resources will be annotated as either illustrative or descriptive.

## Modifying This Specification

This spec is a living document, meaning new resources and fields may be added, and may transition from being OPTIONAL to RECOMMENDED to REQUIRED over time. In general a resource or field should not be added as REQUIRED directly, as this may cause unsuspecting previously-conformant implementations to suddenly no longer be conformant. These should be first OPTIONAL or RECOMMENDED, then change to be REQUIRED once a survey of conformant implementations indicates that doing so will not cause undue burden on any implementation.

## Resource Overview

The Tekton Pipelines API provides a set of API resources to manage run-to-completion workflows. Those are expressed as [Kubernetes Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)

### `TaskRun`

A `TaskRun` represents an instantiation of a single execution of a `Task`. It can describe the steps of the Task directly.

Its HTTP API endpoint is `/apis/tekton.dev/v1beta1/[parent]/taskruns`,  where  `[parent]` can refer to any string identifying a grouping of `TaskRun`s.

For example, in the Kubernetes implementation `[parent]` refers to a Kubernetes namespace. Other implementations could interpret the string differently, including enabling hierarchies of resources containing `TaskRun`s, for example the API endpoint `/apis/tekton.dev/v1beta1/folders/my-folder/project/my-project/taskruns` could represent a hierarchy of "projects" that own `TaskRun`s.

| HTTP Verb                 | Requirement      |
|---------------------------|------------------|
| Create (POST)             | REQUIRED         |
| Patch (PATCH)*            | RECOMMENDED      |
| Replace (PUT)**           | RECOMMENDED      |
| Delete (DELETE)           | OPTIONAL         |
| Read (GET)                | REQUIRED         |
| List (GET)                | REQUIRED         |
| Watch (GET)               | OPTIONAL         |
| DeleteCollection (DELETE) | OPTIONAL         |

\* Kubernetes only allows JSON merge patch for CRD types. It is recommended that if allowed, at least JSON Merge patch be made available. [JSON Merge Patch Spec (RFC 7386)](https://tools.ietf.org/html/rfc7386)

\** NB: Support for cancellation depends on `Replace`.

## Detailed Resources - v1beta1

The following schema defines a set of REQUIRED or RECOMMENDED resource fields on the Tekton resource types. Whether a field is REQUIRED or RECOMMENDED is denoted in the "Requirement" column.

Additional fields MAY be provided by particular implementations, however it is expected that most extension will be accomplished via the `metadata.labels` and `metadata.annotations` fields, as Tekton implementations MAY validate supplied resources against these fields and refuse resources which specify unknown fields.

Tekton implementations MUST NOT require `spec` fields outside this implementation; to do so would break interoperability between such implementations and implementations which implement validation of field names.

**NB:** All fields and resources not listed below are assumed to be **OPTIONAL**, not RECOMMENDED or REQUIRED. For example, at this time, support for `PipelineRun`s and for CRUD operations on `Task`s or `Pipeline`s is **OPTIONAL**.

### `TaskRun`

#### Metadata

Standard Kubernetes [meta.v1/ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta) resource.

| Field Name           | Field Type         | Requirement |
|----------------------|--------------------|-------------|
| `name`               | string             | REQUIRED    |
| `labels`             | map<string,string> | REQUIRED    |
| `annotations`        | map<string,string> | REQUIRED    |
| `creationTimestamp`* | string             | REQUIRED    |
| `uid`                | string             | RECOMMENDED |
| `resourceVersion`    | string             | RECOMMENDED |
| `generation`         | int64              | RECOMMENDED |
| `namespace`          | string             | RECOMMENDED |
| `generateName`**     | string             | RECOMMENDED |

\* `creationTimestamp` MUST be populated by the implementation, in [RFC3339](https://tools.ietf.org/html/rfc3339).

** If `generateName` is supported by the implementation, when it is specified at creation, it MUST be prepended to a random string and set as the `name`, and not set on the subsequent response.

#### Spec

| Field Name            | Field Type           | Requirement |
|-----------------------|----------------------|-------------|
| `params`              | `[]Param`            | REQUIRED    |
| `taskSpec`            | `TaskSpec`           | REQUIRED    |
| `workspaces`          | `[]WorkspaceBinding` | REQUIRED    |
| `status`              | Enum:<br>- `""` (default)<br>- `"TaskRunCancelled"` | RECOMMENDED |
| `timeout`             | string (duration)    | RECOMMENDED |
| `serviceAccountName`^ | string               | RECOMMENDED |

^ In the Kubernetes implementation, `serviceAccountName` refers to a Kubernetes `ServiceAccount` resource that is assumed to exist in the same namespace. Other implementations MAY interpret this string differently, and impose other requirements on specified values.

#### Status

| Field Name            | Field Type             | Requirement |
|-----------------------|------------------------|-------------|
| `conditions`          | see [#error-signaling] | REQUIRED    |
| `startTime`           | string                 | REQUIRED    |
| `completionTime`*     | string                 | REQUIRED    |
| `steps`               | `[]StepState`          | REQUIRED    |
| `taskResults`         | `[]TaskRunResult`      | REQUIRED    |
| `taskSpec`            | `TaskSpec`             | REQUIRED    |
| `observedGeneration`  | int64                  | RECOMMENDED |

\* `startTime` and `completionTime` MUST be populated by the implementation, in [RFC3339](https://tools.ietf.org/html/rfc3339).

## Status Signalling

The Tekton Pipelines API uses the [Kubernetes Conditions convention](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties) to communicate status and errors to the user.

`TaskRun`'s `status` field MUST have a `conditions` field, which must be a list of `Condition` objects of the following form:

| Field                 | Type   | Requirement |
|-----------------------|--------|-------------|
| `type`                | string | REQUIRED    |
| `status`              | Enum:<br>- `"True"`<br>- `"False"`<br>- `"Unknown"` (default) | REQUIRED |
| `reason`              | string | REQUIRED    |
| `message`             | string | REQUIRED    |
| `severity`            | Enum:<br>- `""` (default)<br>- `"Warning"`<br>- `"Info"` | REQUIRED |
| `lastTransitionTime`* | string | OPTIONAL    |

\* If `lastTransitionTime` is populated by the implementation, it must be in [RFC3339](https://tools.ietf.org/html/rfc3339).

Additionally, the resource's `status.conditions` field MUST be managed as follows to enable clients to present useful diagnostic and error information to the user.

If a resource describes that it must report a Condition of the `type` `Succeeded`, then it must report it in the following manner:

* If the `status` field is `"True"`, that means the execution finished successfully.
* If the `status` field is `"False"`, that means the execution finished unsuccessfully -- the Condition's `reason` and `message` MUST include further diagnostic information.
* If the `status` field is `"Unknown"`, that means the execution is still ongoing, and clients can check again later until the Condition's `status` reports either `"True"` or `"False"`.

Resources MAY report Conditions with other `type`s, but none are REQUIRED or RECOMMENDED.

## Listing Resources

Requests to list resources specify the following fields (based on [`meta.v1/ListOptions`](https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#ListOptions)):

| Field Name        | Field Type | Requirement |
|-------------------|------------|-------------|
| `continue`        | string     | REQUIRED    |
| `limit`           | int64      | REQUIRED    |

List responses have the following fields (based on [`meta.v1/ListMeta`](https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#ListMeta)):

| Field Name           | Field Type | Requirement |
|----------------------|------------|-------------|
| `continue`           | string     | REQUIRED    |
| `items`              | `[]Object` | REQUIRED    |
| `remainingItemCount` | int64      | REQUIRED    |
| `apiVersion`         | string<br>(`"tekton.dev/v1beta1"`) | REQUIRED    |
| `kind`               | string<br>(e.g., `"ObjectList"`)   | REQUIRED    |

**NB:** All other fields inherited from [`ListOptions`] or [`ListMeta`]supported by the Kubernetes implementation (e.g., `fieldSelector`,  `labelSelector`) are **OPTIONAL** for the purposes of this spec.

**NB:** The sort order of items returned by list requests is unspecified at this time.

## Detailed Resource Types - v1beta1

### `ArrayOrString`

| Field Name  | Field Type | Requirement |
|-------------|------------|-------------|
| `type`      | Enum:<br>- `"string"` (default)<br>- `"array"` | REQUIRED |
| `stringVal` | string     | REQUIRED    |
| `arrayVal`  | []string   | REQUIRED    |

### `ContainerStateRunning`

| Field Name   | Field Type | Requirement |
|--------------|------------|-------------|
| `startedAt`* | string     | REQUIRED    |

\* `startedAt` MUST be populated by the implementation, in [RFC3339](https://tools.ietf.org/html/rfc3339).

### `ContainerStateWaiting`

| Field Name | Field Type | Requirement |
|------------|------------|-------------|
| `reason`   | string     | REQUIRED    |
| `message`  | string     | REQUIRED    |

### `ContainerStateTerminated`

| Field Name    | Field Type | Requirement |
|---------------|------------|-------------|
| `exitCode`    | int32      | REQUIRED    |
| `reason`      | string     | REQUIRED    |
| `message`     | string     | REQUIRED    |
| `startedAt`*  | string     | REQUIRED    |
| `finishedAt`* | string     | REQUIRED    |

\* `startedAt` and `finishedAt` MUST be populated by the implementation, in [RFC3339](https://tools.ietf.org/html/rfc3339).

### `EnvVar`

| Field Name | Field Type | Requirement |
|------------|------------|-------------|
| `name`     | string     | REQUIRED    |
| `value`    | string     | REQUIRED    |

**NB:** All other [EnvVar](https://godoc.org/k8s.io/api/core/v1#EnvVar) types inherited from [core.v1/EnvVar](https://godoc.org/k8s.io/api/core/v1#EnvVar) and supported by the Kubernetes implementation (e.g., `valueFrom`) are **OPTIONAL** for the purposes of this spec.

### `Param`

| Field Name | Field Type      | Requirement |
|------------|-----------------|-------------|
| `name`     | string          | REQUIRED    |
| `value`    | `ArrayOrString` | REQUIRED    |

### `ParamSpec`

| Field Name    | Field Type | Requirement |
|---------------|------------|-------------|
| `name`        | string     | REQUIRED    |
| `description` | string     | REQUIRED    |
| `type`        | Enum: <br>- `"string"` (default) <br>- `"array"` | REQUIRED |
| `default`     | `ArrayOrString` | REQUIRED |

### `Step`

| Field Name   | Field Type | Requirement |
|--------------|------------|-------------|
| `name`       | string     | REQUIRED    |
| `image`      | string     | REQUIRED    |
| `args`       | []string   | REQUIRED    |
| `command`    | []string   | REQUIRED    |
| `workingDir` | []string   | REQUIRED    |
| `env`        | `[]EnvVar` | REQUIRED    |
| `script`     | string     | REQUIRED    |

**NB:** All other fields inherited from the [core.v1/Container](https://godoc.org/k8s.io/api/core/v1#Container) type supported by the Kubernetes implementation are **OPTIONAL** for the purposes of this spec.

### `StepState`

| Field Name    | Field Type                 | Requirement |
|---------------|----------------------------|-------------|
| `name`        | string                     | REQUIRED    |
| `imageID`     | string                     | REQUIRED    |
| `waiting`*    | `ContainerStateWaiting`    | REQUIRED    |
| `running`*    | `ContainerStateRunning`    | REQUIRED    |
| `terminated`* | `ContainerStateTerminated` | REQUIRED    |

\* Only one of `waiting`, `running` or `terminated` can be returned at a time.

### `TaskResult`

| Field Name   | Field Type | Requirement |
|--------------|------------|-------------|
| `name`        | string    | REQUIRED    |
| `description` | string    | REQUIRED    |

### `TaskRunResult`

| Field Name | Field Type | Requirement |
|------------|------------|-------------|
| `name`     | string     | REQUIRED    |
| `value`    | string     | REQUIRED    |

### `TaskSpec`

| Field Name    | Field Type               | Requirement |
|---------------|--------------------------|-------------|
| `params`      | `[]ParamSpec`            | REQUIRED    |
| `steps`       | `[]Step`                 | REQUIRED    |
| `results`     | `[]TaskResult`           | REQUIRED    |
| `workspaces`  | `[]WorkspaceDeclaration` | REQUIRED    |
| `description` | string                   | REQUIRED    |

### `WorkspaceBinding`

| Field Name | Field Type   | Requirement |
|------------|--------------|-------------|
| `name`     | string       | REQUIRED    |
| `emptyDir` | empty struct | REQUIRED    |

**NB:** All other Workspace types supported by the Kubernetes implementation are **OPTIONAL** for the purposes of this spec.

### `WorkspaceDeclaration`

| Field Name    | Field Type | Requirement |
|---------------|------------|-------------|
| `name`        | string     | REQUIRED    |
| `description` | string     | REQUIRED    |
| `mountPath`   | string     | REQUIRED    |
| `readOnly`    | boolean    | REQUIRED    |
| `description` | string     | REQUIRED    |
