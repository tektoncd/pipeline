# Tekton Pipelines API Specification

<!-- toc -->
- [Tekton Pipelines API Specification](#tekton-pipelines-api-specification)
  - [Abstract](#abstract)
  - [Background](#background)
  - [Modifying This Specification](#modifying-this-specification)
  - [Resource Overview - v1](#resource-overview---v1)
    - [`Task`](#task)
    - [`Pipeline`](#pipeline)
    - [`TaskRun`](#taskrun)
    - [`PipelineRun`](#pipelinerun)
  - [Detailed Resource Types - `v1`](#detailed-resource-types---v1)
    - [TypeMeta](#typemeta)
    - [ObjectMeta](#objectmeta)
    - [TaskSpec](#taskspec)
    - [ParamSpec](#paramspec)
    - [ParamType](#paramtype)
    - [Step](#step)
    - [Sidecar](#sidecar)
    - [SecurityContext](#securitycontext)
    - [TaskResult](#taskresult)
    - [ResultsType](#resultstype)
    - [PipelineSpec](#pipelinespec)
    - [PipelineTask](#pipelinetask)
    - [TaskRef](#taskref)
    - [ResolverRef](#resolverref)
    - [Param](#param)
    - [ParamValue](#paramvalue)
    - [PipelineResult](#pipelineresult)
    - [TaskRunSpec](#taskrunspec)
    - [TaskRunStatus](#taskrunstatus)
    - [Condition](#condition)
    - [StepState](#stepstate)
    - [ContainerState](#containerstate)
    - [`ContainerStateRunning`](#containerstaterunning)
    - [`ContainerStateWaiting`](#containerstatewaiting)
    - [`ContainerStateTerminated`](#containerstateterminated)
    - [TaskRunResult](#taskrunresult)
    - [SidecarState](#sidecarstate)
    - [PipelineRunSpec](#pipelinerunspec)
    - [PipelineRef](#pipelineref)
    - [PipelineRunStatus](#pipelinerunstatus)
    - [PipelineRunResult](#pipelinerunresult)
    - [ChildStatusReference](#childstatusreference)
    - [TimeoutFields](#timeoutfields)
    - [WorkspaceDeclaration](#workspacedeclaration)
    - [WorkspacePipelineTaskBinding](#workspacepipelinetaskbinding)
    - [PipelineWorkspaceDeclaration](#pipelineworkspacedeclaration)
    - [WorkspaceBinding](#workspacebinding)
    - [EnvVar](#envvar)
  - [Status Signalling](#status-signalling)
<!-- /toc -->

## Abstract

The Tekton Pipelines platform provides common abstractions for describing and executing container-based, run-to-completion workflows, typically in service of CI/CD scenarios. The Tekton Conformance Policy defines the requirements that Tekton implementations must meet to claim conformance with the Tekton API. [TEP-0131](https://github.com/tektoncd/community/blob/main/teps/0131-tekton-conformance-policy.md) lay out details of the policy itself.

According to the policy, Tekton implementations can claim Conformance on GA Primitives, thus, all API Spec in this doc is for Tekton V1 APIs. Implementations are only required to provide resource management (i.e. CRUD APIs) for Runtime Primitives (TaskRun and PipelineRun). For Authoring-time Primitives (Task and Pipeline), supporting CRUD APIs is not a requirement but we recommend referencing them in runtime types (e.g. from git, catalog, within the cluster etc.)

This document describes the structure, and lifecycle of Tekton resources. This document does not define the [runtime contract](https://tekton.dev/docs/pipelines/container-contract/) nor prescribe specific implementations of supporting services such as access control, observability, or resource management.

This document makes reference in a few places to different profiles for Tekton installations. A profile in this context is a set of operations, resources, and fields that are accessible to a developer interacting with a Tekton installation. Currently, only a single (minimal) profile for Tekton Pipelines is defined, but additional profiles may be defined in the future to standardize advanced functionality. A minimal profile is one that implements all of the “MUST”, “MUST NOT”, and “REQUIRED” conditions of this document.

## Background

The key words “MUST”, “MUST NOT”, “REQUIRED”, “SHALL”, “SHALL NOT”, “SHOULD”, “SHOULD NOT”, “RECOMMENDED”, “NOT RECOMMENDED”, “MAY”, and “OPTIONAL” are to be interpreted as described in [RFC 2119](https://tools.ietf.org/html/rfc2119).

There is no formal specification of the Kubernetes API and Resource Model. This document assumes Kubernetes 1.25 behavior; this behavior will typically be supported by many future Kubernetes versions. Additionally, this document may reference specific core Kubernetes resources; these references may be illustrative (i.e. an implementation on Kubernetes) or descriptive (i.e. this Kubernetes resource MUST be exposed). References to these core Kubernetes resources will be annotated as either illustrative or descriptive.

## Modifying This Specification

This spec is a living document, meaning new resources and fields may be added, and may transition from being OPTIONAL to RECOMMENDED to REQUIRED over time. In general a resource or field should not be added as REQUIRED directly, as this may cause unsuspecting previously-conformant implementations to suddenly no longer be conformant. These should be first OPTIONAL or RECOMMENDED, then change to be REQUIRED once a survey of conformant implementations indicates that doing so will not cause undue burden on any implementation.

## Resource Overview - v1

The following schema defines a set of REQUIRED or RECOMMENDED resource fields on the Tekton resource types. Whether a field is REQUIRED or RECOMMENDED is denoted in the "Requirement" column.

Additional fields MAY be provided by particular implementations, however it is expected that most extension will be accomplished via the `metadata.labels` and `metadata.annotations` fields, as Tekton implementations MAY validate supplied resources against these fields and refuse resources which specify unknown fields.

Tekton implementations MUST NOT require `spec` fields outside this implementation; to do so would break interoperability between such implementations and implementations which implement validation of field names.

**NB:** All fields and resources not listed below are assumed to be **OPTIONAL**, not RECOMMENDED or REQUIRED.

### `Task`

A Task is a collection of Steps that is defined and arranged in a sequential order of execution.

| Field        | Type                        | Requirement | Notes                                          |
|--------------|-----------------------------|-------------|------------------------------------------------|
| `kind`       | string                      | RECOMMENDED | Describes the type of the resource i.e. `Task` |
| `apiVersion` | string                      | RECOMMENDED | Schema version i.e. `v1`                       |
| `metadata`   | [`ObjectMeta`](#objectmeta) | REQUIRED    | Common metadata about a resource               |
| `spec`       | [`TaskSpec`](#taskspec)     | REQUIRED    | Defines the desired state of Task.             |

**NB:** If `kind` and `apiVersion` are not supported, an alternative method of identifying the type of resource must be supported.

### `Pipeline`

A Pipeline is a collection of Tasks that is defined and arranged in a specific order of execution

| Field        | Type                            | Requirement | Notes                                              |
|--------------|---------------------------------|-------------|----------------------------------------------------|
| `kind`       | string                          | RECOMMENDED | Describes the type of the resource i.e. `Pipeline` |
| `apiVersion` | string                          | RECOMMENDED | Schema version i.e. `v1`                           |
| `metadata`   | [`ObjectMeta`](#objectmeta)     | REQUIRED    | Common metadata about a resource                   |
| `spec`       | [`PipelineSpec`](#pipelinespec) | REQUIRED    | Defines the desired state of Pipeline.             |

**NB:** If `kind` and `apiVersion` are not supported, an alternative method of identifying the type of resource must be supported.

### `TaskRun`

A `TaskRun` represents an instantiation of a single execution of a `Task`. It can describe the steps of the Task directly.

| Field        | Type                              | Requirement | Notes                                            |
|--------------|-----------------------------------|-------------|--------------------------------------------------|
| `kind`       | string                            | RECOMMENDED | Describes the type of the resource i.e.`TaskRun` |
| `apiVersion` | string                            | RECOMMENDED | Schema version i.e. `v1`                         |
| `metadata`   | [`ObjectMeta`](#objectmeta)       | REQUIRED    | Common metadata about a resource                 |
| `spec`       | [`TaskRunSpec`](#taskrunspec)     | REQUIRED    | Defines the desired state of TaskRun             |
| `status`     | [`TaskRunStatus`](#taskrunstatus) | REQUIRED    | Defines the current status of TaskRun            |

**NB:** If `kind` and `apiVersion` are not supported, an alternative method of identifying the type of resource must be supported.

### `PipelineRun`

A `PipelineRun` represents an instantiation of a single execution of a `Pipeline`. It can describe the spec of the Pipeline directly.

| Field        | Type                                      | Requirement | Notes                                                |
|--------------|-------------------------------------------|-------------|------------------------------------------------------|
| `kind`       | string                                    | RECOMMENDED | Describes the type of the resource i.e.`PipelineRun` |
| `apiVersion` | string                                    | RECOMMENDED | Schema version i.e. `v1`                             |
| `metadata`   | [`ObjectMeta`](#objectmeta)               | REQUIRED    | Common metadata about a resource                     |
| `spec`       | [`PipelineRunSpec`](#pipelinerunspec)     | REQUIRED    | Defines the desired state of PipelineRun             |
| `status`     | [`PipelineRunStatus`](#pipelinerunstatus) | REQUIRED    | Defines the current status of PipelineRun            |

**NB:** If `kind` and `apiVersion` are not supported, an alternative method of identifying the type of resource must be supported.

## Detailed Resource Types - `v1`

### TypeMeta

Derived from [Kuberentes Type Meta](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#TypeMeta)

| Field        | Type   | Notes                                                             |
|--------------|--------|-------------------------------------------------------------------|
| `kind`       | string | A string value representing the resource this object represents.  |
| `apiVersion` | string | Defines the versioned schema of this representation of an object. |

### ObjectMeta

Derived from standard Kubernetes [meta.v1/ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta) resource.

| Field               | Type               | Requirement         | Notes                                                                                                                                                                                                                               |
| ------------------- | ------------------ | ------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `name`              | string             | REQUIRED            | Mutually exclusive with the `generateName` field.                                                                                                                                                                                   |
| `labels`            | map<string,string> | RECOMMENDED         |                                                                                                                                                                                                                                     |
| `annotations`       | map<string,string> | RECOMMENDED         | `annotations` are necessary in order to support integration with Tekton ecosystem tooling such as Results and Chains                                                                                                                |
| `creationTimestamp` | string             | REQUIRED (see note) | `creationTimestamp` MUST be populated by the implementation, in [RFC3339](https://tools.ietf.org/html/rfc3339). <br>The field is required for any runtimeTypes such as `TaskRun` and `PipelineRun` and RECOMMENDED for othet types. |
| `uid`               | string             | RECOMMENDED         | If `uid` is not supported, the implementation must support another way of uniquely identifying a runtime object such as using a combination of `namespace` and `name`                                                               |
| `resourceVersion`   | string             | OPTIONAL            |                                                                                                                                                                                                                                     |
| `generation`        | int64              | OPTIONAL            |                                                                                                                                                                                                                                     |
| `generateName`      | string             | RECOMMENDED         | If supported by the implementation, when `generateName` is specified at creation, it MUST be prepended to a random string and set as the `name`, and not set on the subsequent response.                                            |

### TaskSpec

Defines the desired state of Task

| Field         | Type                                              | Requirement | Notes |
|---------------|---------------------------------------------------|-------------|-------|
| `description` | string                                            | REQUIRED    |       |
| `params`      | [][`ParamSpec`](#paramspec)                       | REQUIRED    |       |
| `steps`       | [][`Step`](#step)                                 | REQUIRED    |       |
| `sidecars`    | [][`Sidecar`](#sidecar)                           | REQUIRED    |       |
| `results`     | [][`TaskResult`](#taskresult)                     | REQUIRED    |       |
| `workspaces`  | [][`WorkspaceDeclaration`](#workspacedeclaration) | REQUIRED    |       |

### ParamSpec

Declares a parameter whose value has to be provided at runtime

| Field Name    | Field Type                  | Requirement         | Notes                                                                                                                                                                                    |
|---------------|-----------------------------|---------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `name`        | string                      | REQUIRED            |                                                                                                                                                                                          |
| `description` | string                      | REQUIRED            |                                                                                                                                                                                          |
| `type`        | [`ParamType`](#paramtype)   | REQUIRED (see note) | The values `string` and `array` for this field are REQUIRED, and the value `object` is RECOMMENDED.                                                                                      |
| `properties`  | map<string,PropertySpec>    | RECOMMENDED         | `PropertySpec` is a type that defines the spec of an individual key. See how to define the `properties` section in the [example](../examples/v1/taskruns/beta/object-param-result.yaml). |
| `default`     | [`ParamValue`](#paramvalue) | REQUIRED            |                                                                                                                                                                                          |

### ParamType

Defines the type of a parameter

string enum, allowed values are `string`, `array`, and `object`. Supporting `string` and `array` are required while the other types are optional for conformance.

### Step

A Step is a reference to a container image that executes a specific tool on a specific input and produces a specific output.
**NB:** All other fields inherited from the [core.v1/Container](https://godoc.org/k8s.io/api/core/v1#Container) type supported by the Kubernetes implementation are **OPTIONAL** for the purposes of this spec.

| Field Name        | Field Type                            | Requirement | Notes |
| ----------------- | ------------------------------------- | ----------- | ----- |
| `name`            | string                                | REQUIRED    |       |
| `image`           | string                                | REQUIRED    |       |
| `args`            | []string                              | REQUIRED    |       |
| `command`         | []string                              | REQUIRED    |       |
| `workingDir`      | []string                              | REQUIRED    |       |
| `env`             | [][`EnvVar`](#envvar)                 | REQUIRED    |       |
| `script`          | string                                | REQUIRED    |       |
| `securityContext` | [`SecurityContext`](#securitycontext) | REQUIRED    |       |

### Sidecar

Specifies a list of containers to run alongside the Steps in a Task. If sidecars are supported, the following fields are required:

| Field             | Type                                  | Requirement | Notes                                                                                                                       |
|-------------------|---------------------------------------|-------------|-----------------------------------------------------------------------------------------------------------------------------|
| `name`            | string                                | REQUIRED    | Name of the Sidecar specified as a DNS_LABEL. Each Sidecar in a Task must have a unique name (DNS_LABEL).Cannot be updated. |
| `image`           | string                                | REQUIRED    | [Container image name](https://kubernetes.io/docs/concepts/containers/images/#image-names)                                  |
| `command`         | []string                              | REQUIRED    | Entrypoint array. Not executed within a shell. The image's ENTRYPOINT is used if this is not provided.                      |
| `args`            | []string                              | REQUIRED    | Arguments to the entrypoint. The image's CMD is used if this is not provided.                                               |
| `script`          | string                                | REQUIRED    | Script is the contents of an executable file to execute. If Script is not empty, the Sidecar cannot have a Command or Args. |
| `securityContext` | [`SecurityContext`](#securitycontext) | REQUIRED    | Defines the security options the Sidecar should be run with.                                                                |

### SecurityContext

All other fields derived from [core.v1/SecurityContext](https://pkg.go.dev/k8s.io/api/core/v1#SecurityContext) are OPTIONAL for the purposes of this spec.

| Field        | Type | Requirement | Notes                                                  |
| ------------ | ---- | ----------- | ------------------------------------------------------ |
| `privileged` | bool | REQUIRED    | Run the container in privileged mode. Default to false |

### TaskResult

Defines a result produced by a Task

| Field         | Type                          | Requirement | Notes                                                                                                                                                                                    |
|---------------|-------------------------------|-------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `name`        | string                        | REQUIRED    | Declares the name by which a parameter is referenced.                                                                                                                                    |
| `type`        | [`ResultsType`](#resultstype) | REQUIRED    | Type is the user-specified type of the result.  The values `string` this field is REQUIRED, and the values `array` and `object` are RECOMMENDED.                                         |
| `description` | string                        | RECOMMENDED | Description of the result                                                                                                                                                                |
| `properties`  | map<string,PropertySpec>      | RECOMMENDED | `PropertySpec` is a type that defines the spec of an individual key. See how to define the `properties` section in the [example](../examples/v1/taskruns/beta/object-param-result.yaml). |

### ResultsType

ResultsType indicates the type of a result.

string enum, Allowed values are `string`, `array`, and `object`. Supporting `string` is required while the other types are optional for conformance.

### PipelineSpec

Defines a pipeline

| Field        | Type                                                              | Requirement | Notes                                                                                            |
|--------------|-------------------------------------------------------------------|-------------|--------------------------------------------------------------------------------------------------|
| `params`     | [][`ParamSpec`](#paramspec)                                       | REQUIRED    | Params declares a list of input parameters that must be supplied when this Pipeline is run.      |
| `tasks`      | [][`PipelineTask`](#pipelinetask)                                 | REQUIRED    | Tasks declares the graph of Tasks that execute when this Pipeline is run.                        |
| `results`    | [][`PipelineResult`](#pipelineresult)                             | REQUIRED    | Values that this pipeline can output once run.                                                   |
| `finally`    | [][`PipelineTask`](#pipelinetask)                                 | REQUIRED    | The list of Tasks that execute just before leaving the Pipeline                                  |
| `workspaces` | [][`PipelineWorkspaceDeclaration`](#pipelineworkspacedeclaration) | REQUIRED    | Workspaces declares a set of named workspaces that are expected to be provided by a PipelineRun. |

### PipelineTask

PiplineTask defines a task in a Pipeline, passing inputs from both `Params`` and from the output of previous tasks.

| Field        | Type                                                              | Requirement | Notes                                                                                                                                                                             |
|--------------|-------------------------------------------------------------------|-------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `name`       | string                                                            | REQUIRED    | The name of this task within the context of a Pipeline. Used as a coordinate with the from and runAfter fields to establish the execution order of tasks relative to one another. |
| `taskRef`    | [`TaskRef`](#taskref)                                             | RECOMMENDED | TaskRef is a reference to a task definition. Mutually exclusive with TaskSpec                                                                                                     |
| `taskSpec`   | [`TaskSpec`](#taskspec)                                           | REQUIRED    | TaskSpec is a specification of a task. Mutually exclusive with TaskRef                                                                                                            |
| `runAfter`   | []string                                                          | REQUIRED    | RunAfter is the list of PipelineTask names that should be executed before this Task executes. (Used to force a specific ordering in graph execution.)                             |
| `params`     | [][`Param`](#param)                                               | REQUIRED    | Declares parameters passed to this task.                                                                                                                                          |
| `workspaces` | [][`WorkspacePipelineTaskBinding`](#workspacepipelinetaskbinding) | REQUIRED    | Workspaces maps workspaces from the pipeline spec to the workspaces declared in the Task.                                                                                         |
| `timeout`    | int64                                                             | REQUIRED    | Time after which the TaskRun times out.  Setting the timeout to 0 implies no timeout. There isn't a default max timeout set.                                                      |

### TaskRef

Refers to a Task. Tasks should be referred either by a name or by using the Remote Resolution framework.

| Field      | Type                | Requirement | Notes                   |
|------------|---------------------|-------------|-------------------------|
| `name`     | string              | RECOMMENDED | Name of the referent.   |
| `resolver` | string              | RECOMMENDED | A field of ResolverRef. |
| `params`   | [][`Param`](#param) | RECOMMENDED | A field of ResolverRef. |

### ResolverRef

| Field      | Type                | Requirement | Notes                                                                                                                 |
|------------|---------------------|-------------|-----------------------------------------------------------------------------------------------------------------------|
| `resolver` | string              | RECOMMENDED | Resolver is the name of the resolver that should perform resolution of the referenced Tekton resource, such as "git". |
| `params`   | [][`Param`](#param) | RECOMMENDED | Contains the parameters used to identify the referenced Tekton resource.                                              |

### Param

Provides a value for the named paramter.

| Field   | Type                        | Requirement | Notes |
|---------|-----------------------------|-------------|-------|
| `name`  | string                      | REQUIRED    |       |
| `value` | [`ParamValue`](#paramvalue) | REQUIRED    |       |

### ParamValue

A `ParamValue` may be a string, a list of string, or a map of string to string.

### PipelineResult

| Field   | Type                          | Requirement | Notes |
|---------|-------------------------------|-------------|-------|
| `name`  | string                        | REQUIRED    |       |
| `type`  | [`ResultsType`](#resultstype) | REQUIRED    |       |
| `value` | [`ParamValue`](#paramvalue)   | REQUIRED    |       |

### TaskRunSpec

| Field                 | Type                                                | Requirement | Notes                                                                                                                                                                                                                                                                   |
|-----------------------|-----------------------------------------------------|-------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `params`              | [][`Param`](#param)                                 | REQUIRED    |                                                                                                                                                                                                                                                                         |
| `taskRef`             | [`TaskRef`](#taskref)                               | REQUIRED    |                                                                                                                                                                                                                                                                         |
| `taskSpec`            | [`TaskSpec`](#taskspec)                             | REQUIRED    |                                                                                                                                                                                                                                                                         |
| `workspaces`          | [][`WorkspaceBinding`](#workspacebinding)           | REQUIRED    |                                                                                                                                                                                                                                                                         |
| `timeout`             | string (duration)                                   | REQUIRED    | Time after which one retry attempt times out. Defaults to 1 hour.                                                                                                                                                                                                       |
| `status`              | Enum:<br>- `""` (default)<br>- `"TaskRunCancelled"` | RECOMMENDED |                                                                                                                                                                                                                                                                         |
| `serviceAccountName`^ | string                                              | RECOMMENDED | In the Kubernetes implementation, `serviceAccountName` refers to a Kubernetes `ServiceAccount` resource that is assumed to exist in the same namespace. Other implementations MAY interpret this string differently, and impose other requirements on specified values. |

### TaskRunStatus

| Field                | Type                                | Requirement | Notes                                                                                                                           |
|----------------------|-------------------------------------|-------------|---------------------------------------------------------------------------------------------------------------------------------|
| `conditions`         | [][`Condition`](#condition)         | REQUIRED    | Condition type `Succeeded` MUST be populated. See [Status Signalling](#status-signalling) for details. Other types are OPTIONAL |
| `startTime`          | string                              | REQUIRED    | MUST be populated by the implementation, in [RFC3339](https://tools.ietf.org/html/rfc3339).                                     |
| `completionTime`     | string                              | REQUIRED    | MUST be populated by the implementation, in [RFC3339](https://tools.ietf.org/html/rfc3339).                                     |
| `taskSpec`           | [`TaskSpec`](#taskspec)             | REQUIRED    |                                                                                                                                 |
| `steps`              | [][`StepState`](#stepstate)         | REQUIRED    |                                                                                                                                 |
| `results`            | [][`TaskRunResult`](#taskrunresult) | REQUIRED    |                                                                                                                                 |
| `sidecars`           | [][`SidecarState`](#sidecarstate)   | RECOMMENDED |                                                                                                                                 |
| `observedGeneration` | int64                               | RECOMMENDED |                                                                                                                                 |

### Condition

| Field     | Type   | Requirement | Notes                                                                                                                                                                  |
|-----------|--------|-------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `type`    | string | REQUIRED    | Required values: <br> `Succeeded`: specifies that the resource has finished.<br> Other OPTIONAL values: <br> `TaskRunResultsVerified` <br>  `TrustedResourcesVerified` |
| `status`  | string | REQUIRED    | Valid values: <br> "UNKNOWN": .<br> "TRUE" <br> "FALSE". (Also see [Status Signalling](#status-signalling))                                                            |
| `reason`  | string | REQUIRED    | The reason for the condition's last transition.                                                                                                                        |
| `message` | string | RECOMMENDED | Message describing the status and reason.                                                                                                                              |

### StepState

| Field            | Type                                | Requirement | Notes                     |
|------------------|-------------------------------------|-------------|---------------------------|
| `name`           | string                              | REQUIRED    | Name of the StepState.    |
| `imageID`        | string                              | REQUIRED    | Image ID of the StepState |
| `containerState` | [`ContainerState`](#containerstate) | REQUIRED    | State of the container    |

### ContainerState

| Field        | Type                         | Requirement | Notes                                |
|--------------|------------------------------|-------------|--------------------------------------|
| `waiting`    | [`ContainerStateWaiting`]    | REQUIRED    | Details about a waiting container    |
| `running`    | [`ContainerStateRunning`]    | REQUIRED    | Details about a running container    |
| `terminated` | [`ContainerStateTerminated`] | REQUIRED    | Details about a terminated container |

\* Only one of `waiting`, `running` or `terminated` can be returned at a time.

### `ContainerStateRunning`

| Field Name   | Field Type | Requirement | Notes                                                                                                                                                     |
|--------------|------------|-------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------|
| `startedAt`* | string     | REQUIRED    | Time at which the container was last (re-)started.`startedAt` MUST be populated by the implementation, in [RFC3339](https://tools.ietf.org/html/rfc3339). |

### `ContainerStateWaiting`

| Field Name | Field Type | Requirement | Notes                                                   |
|------------|------------|-------------|---------------------------------------------------------|
| `reason`   | string     | REQUIRED    | Reason the container is not yet running.                |
| `message`  | string     | RECOMMENDED | Message regarding why the container is not yet running. |

### `ContainerStateTerminated`

| Field Name    | Field Type | Requirement | Notes                                                   |
|---------------|------------|-------------|---------------------------------------------------------|
| `exitCode`    | int32      | REQUIRED    | Exit status from the last termination of the container. |
| `reason`      | string     | REQUIRED    | Reason from the last termination of the container.      |
| `message`     | string     | RECOMMENDED | Message regarding the last termination of the container |
| `startedAt`*  | string     | REQUIRED    | Time at which the container was last (re-)started.      |
| `finishedAt`* | string     | REQUIRED    | Time at which the container last terminated.            |

\* `startedAt` and `finishedAt` MUST be populated by the implementation, in [RFC3339](https://tools.ietf.org/html/rfc3339).

### TaskRunResult

| Field   | Type                          | Requirement | Notes |
|---------|-------------------------------|-------------|-------|
| `name`  | string                        | REQUIRED    |       |
| `type`  | [`ResultsType`](#resultstype) | REQUIRED    |       |
| `value` | [`ParamValue`](#paramvalue)   | REQUIRED    |       |

### SidecarState

| Field            | Type                                | Requirement | Notes                         |
|------------------|-------------------------------------|-------------|-------------------------------|
| `name`           | string                              | REQUIRED    | Name of the SidecarState.     |
| `imageID`        | string                              | REQUIRED    | Image ID of the SidecarState. |
| `containerState` | [`ContainerState`](#containerstate) | REQUIRED    | State of the container.       |

### PipelineRunSpec

| Field          | Type                                    | Requirement | Notes                                    |
|----------------|-----------------------------------------|-------------|------------------------------------------|
| `params`       | [][`Param`](#param)                     | REQUIRED    |                                          |
| `pipelineRef`  | [`PipelineRef`](#pipelineref)           | RECOMMENDED |                                          |
| `pipelineSpec` | [`PipelineSpec`](#pipelinespec)         | REQUIRED    |                                          |
| `timeouts`     | [`TimeoutFields`](#timeoutfields)       | REQUIRED    | Time after which the Pipeline times out. |
| `workspaces`   | [`WorkspaceBinding`](#workspacebinding) | REQUIRED    |                                          |

### PipelineRef

| Field      | Type              | Requirement | Note                    |
|------------|-------------------|-------------|-------------------------|
| `name`     | string            | RECOMMENDED | Name of the referent.   |
| `resolver` | string            | RECOMMENDED | A field of ResolverRef. |
| `params`   | [][Param](#param) | RECOMMENDED | A field of ResolverRef. |

### PipelineRunStatus

| Field             | Type                                            | Requirement | Notes                                                                                                                           |
|-------------------|-------------------------------------------------|-------------|---------------------------------------------------------------------------------------------------------------------------------|
| `conditions`      | [][`Condition`](#condition)                     | REQUIRED    | Condition type `Succeeded` MUST be populated. See [Status Signalling](#status-signalling) for details. Other types are OPTIONAL |
| `startTime`       | string                                          | REQUIRED    | MUST be populated by the implementation, in [RFC3339](https://tools.ietf.org/html/rfc3339).                                     |
| `completionTime`  | string                                          | REQUIRED    | MUST be populated by the implementation, in [RFC3339](https://tools.ietf.org/html/rfc3339).                                     |
| `pipelineSpec`    | [`PipelineSpec`](#pipelinespec)                 | RECOMMEDED  | Resolved spec of the pipeline that was executed                                                                                 |
| `results`         | [][`PipelineRunResult`](#pipelinerunresult)     | RECOMMENDED | Results produced from the pipeline                                                                                              |
| `childReferences` | [][ChildStatusReference](#childstatusreference) | REQUIRED    | References to any child Runs created as part of executing the pipelinerun                                                       |

### PipelineRunResult

| Field   | Type                        | Requirement | Notes                                                               |
|---------|-----------------------------|-------------|---------------------------------------------------------------------|
| `name`  | string                      | RECOMMENDED | Name is the result's name as declared by the Pipeline               |
| `value` | [`ParamValue`](#paramvalue) | RECOMMENDED | Value is the result returned from the execution of this PipelineRun |

### ChildStatusReference

| Field              | Type   | Requirement | Notes                                                                 |
|--------------------|--------|-------------|-----------------------------------------------------------------------|
| `Name`             | string | REQUIRED    | Name is the name of the TaskRun this is referencing.                  |
| `PipelineTaskName` | string | REQUIRED    | PipelineTaskName is the name of the PipelineTask this is referencing. |

### TimeoutFields

| Field      | Type              | Requirement | Notes                                                                                                                                                             |
|------------|-------------------|-------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `Pipeline` | string (duration) | REQUIRED    | Pipeline sets the maximum allowed duration for execution of the entire pipeline. The sum of individual timeouts for tasks and finally must not exceed this value. |
| `Tasks`    | string (duration) | REQUIRED    | Tasks sets the maximum allowed duration of this pipeline's tasks                                                                                                  |
| `Finally`  | string (duration) | REQUIRED    | Finally sets the maximum allowed duration of this pipeline's finally                                                                                              |

**string (duration)** :  A duration string is a possibly signed sequence of decimal numbers, each with optional fraction and a unit suffix, such as "300ms", "-1.5h" or "2h45m". Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h"

**Note:** Currently three keys are accepted in the map: pipeline, tasks and finally. Timeouts.pipeline >= Timeouts.tasks + Timeouts.finally

### WorkspaceDeclaration

| Field         | Type    | Requirement | Notes                                                         |
|---------------|---------|-------------|---------------------------------------------------------------|
| `name`        | string  | REQUIRED    | Name is the name by which you can bind the volume at runtime. |
| `description` | string  | RECOMMENDED |                                                               |
| `mountPath`   | string  | RECOMMENDED |                                                               |
| `readOnly`    | boolean | RECOMMENDED | Defaults to false.                                            |

### WorkspacePipelineTaskBinding

| Field       | Type   | Requirement | Notes                                                           |
|-------------|--------|-------------|-----------------------------------------------------------------|
| `name`      | string | REQUIRED    | Name is the name of the workspace as declared by the task       |
| `workspace` | string | REQUIRED    | Workspace is the name of the workspace declared by the pipeline |

### PipelineWorkspaceDeclaration

| Field  | Type   | Requirement | Notes                                                            |
|--------|--------|-------------|------------------------------------------------------------------|
| `name` | string | REQUIRED    | Name is the name of a workspace to be provided by a PipelineRun. |

### WorkspaceBinding

| Field Name | Field Type   | Requirement | Notes |
|------------|--------------|-------------|-------|
| `name`     | string       | REQUIRED    |       |
| `emptyDir` | empty struct | REQUIRED    |       |

**NB:** All other Workspace types supported by the Kubernetes implementation are **OPTIONAL** for the purposes of this spec.
### EnvVar

| Field Name | Field Type | Requirement | Notes |
|------------|------------|-------------|-------|
| `name`     | string     | REQUIRED    |       |
| `value`    | string     | REQUIRED    |       |

**NB:** All other [EnvVar](https://godoc.org/k8s.io/api/core/v1#EnvVar) types inherited from [core.v1/EnvVar](https://godoc.org/k8s.io/api/core/v1#EnvVar) and supported by the Kubernetes implementation (e.g., `valueFrom`) are **OPTIONAL** for the purposes of this spec.

## Status Signalling
 <!-- wokeignore:rule=master --> 
The Tekton Pipelines API uses the [Kubernetes Conditions convention](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties) to communicate status and errors to the user.

`TaskRun`'s `status` field MUST have a `conditions` field, which must be a list of `Condition` objects of the following form:

| Field                 | Type                                                          | Requirement |
|-----------------------|---------------------------------------------------------------|-------------|
| `type`                | string                                                        | REQUIRED    |
| `status`              | Enum:<br>- `"True"`<br>- `"False"`<br>- `"Unknown"` (default) | REQUIRED    |
| `reason`              | string                                                        | REQUIRED    |
| `message`             | string                                                        | REQUIRED    |
| `severity`            | Enum:<br>- `""` (default)<br>- `"Warning"`<br>- `"Info"`      | REQUIRED    |
| `lastTransitionTime`* | string                                                        | OPTIONAL    |

\* If `lastTransitionTime` is populated by the implementation, it must be in [RFC3339](https://tools.ietf.org/html/rfc3339).

Additionally, the resource's `status.conditions` field MUST be managed as follows to enable clients to present useful diagnostic and error information to the user.

If a resource describes that it must report a Condition of the `type` `Succeeded`, then it must report it in the following manner:

- If the `status` field is `"True"`, that means the execution finished successfully.
- If the `status` field is `"False"`, that means the execution finished unsuccessfully -- the Condition's `reason` and `message` MUST include further diagnostic information.
- If the `status` field is `"Unknown"`, that means the execution is still ongoing, and clients can check again later until the Condition's `status` reports either `"True"` or `"False"`.

Resources MAY report Conditions with other `type`s, but none are REQUIRED or RECOMMENDED.
