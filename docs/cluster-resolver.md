<!--
---
linkTitle: "Cluster Resolver"
weight: 310
---
-->

# Cluster Resolver

## Resolver Type

This Resolver responds to type `cluster`.

## Parameters

| Param Name  | Description                                           | Example Value                |
|-------------|-------------------------------------------------------|------------------------------|
| `kind`      | The kind of resource to fetch.                        | `task`, `pipeline`           |
| `name`      | The name of the resource to fetch.                    | `some-pipeline`, `some-task` |
| `namespace` | The namespace in the cluster containing the resource. | `default`, `other-namespace` |

## Requirements

- A cluster running Tekton Pipeline v0.41.0 or later.
- The [built-in remote resolvers installed](./install.md#installing-and-configuring-remote-task-and-pipeline-resolution).
- The `enable-cluster-resolver` feature flag in the `resolvers-feature-flags` ConfigMap
  in the `tekton-pipelines-resolvers` namespace set to `true`.
- [Beta features](./additional-configs.md#beta-features) enabled.

## Configuration

This resolver uses a `ConfigMap` for its settings. See
[`../config/resolvers/cluster-resolver-config.yaml`](../config/resolvers/cluster-resolver-config.yaml)
for the name, namespace and defaults that the resolver ships with.

### Options

| Option Name          | Description                                                                                                                                         | Example Values                     |
|----------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------|------------------------------------|
| `default-kind`       | The default resource kind to fetch if not specified in parameters.                                                                                  | `task`, `pipeline`                 |
| `default-namespace`  | The default namespace to fetch resources from if not specified in parameters.                                                                       | `default`, `some-namespace`        |
| `allowed-namespaces` | An optional comma-separated list of namespaces which the resolver is allowed to access. Defaults to empty, meaning all namespaces are allowed.      | `default,some-namespace`, (empty)  |
| `blocked-namespaces` | An optional comma-separated list of namespaces which the resolver is blocked from accessing. Defaults to empty, meaning all namespaces are allowed. | `default,other-namespace`, (empty) |       

## Usage

### Task Resolution

```yaml
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: remote-task-reference
spec:
  taskRef:
    resolver: cluster
    params:
    - name: kind
      value: task
    - name: name
      value: some-task
    - name: namespace
      value: namespace-containing-task
```

### Pipeline resolution

```yaml
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: remote-pipeline-reference
spec:
  pipelineRef:
    resolver: cluster
    params:
    - name: kind
      value: pipeline
    - name: name
      value: some-pipeline
    - name: namespace
      value: namespace-containing-pipeline
```

## `ResolutionRequest` Status
`ResolutionRequest.Status.RefSource` field captures the source where the remote resource came from. It includes the 3 subfields: `url`, `digest` and `entrypoint`.
- `url`: url is the unique full identifier for the resource in the cluster. It is in the format of `<resource uri>@<uid>`. Resource URI part is the namespace-scoped uri i.e. `/apis/GROUP/VERSION/namespaces/NAMESPACE/RESOURCETYPE/NAME`. See [K8s Resource URIs](https://kubernetes.io/docs/reference/using-api/api-concepts/#resource-uris) for more details.
- `digest`: hex-encoded sha256 checksum of the content in the in-cluster resource's spec field. The reason why it's the checksum of the spec content rather than the whole object is because the metadata of in-cluster resources might be modified i.e. annotations. Therefore, the checksum of the spec content should be sufficient for source verifiers to verify if things have been changed maliciously even though the metadata is modified with good intentions.
- `entrypoint`: ***empty*** because the path information is already available in the url field.

Example:
- TaskRun Resolution

```yaml
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: cluster-demo
spec:
  taskRef:
    resolver: cluster
    params:
    - name: kind
      value: task
    - name: name
      value: a-simple-task
    - name: namespace
      value: default
```


- `ResolutionRequest`
```yaml
apiVersion: resolution.tekton.dev/v1beta1
kind: ResolutionRequest
metadata:
  labels:
    resolution.tekton.dev/type: cluster
  name: cluster-7a04be6baa3eeedd232542036b7f3b2d
  namespace: default
  ownerReferences: ...
spec:
  params:
  - name: kind
    value: task
  - name: name
    value: a-simple-task
  - name: namespace
    value: default
status:
  annotations: ...
  conditions: ...
  data: xxx
  refSource:
    digest:
      sha256: 245b1aa918434cc8195b4d4d026f2e43df09199e2ed31d4dfd9c2cbea1c7ce54
    uri: /apis/tekton.dev/v1beta1/namespaces/default/task/a-simple-task@3b82d8c4-f89e-47ea-a49d-3be0dca4c038
```
---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
