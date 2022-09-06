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

- A cluster running Tekton Pipeline v0.40.0 or later, with the `alpha` feature gate enabled.
- The [built-in remote resolvers installed](./install.md#installing-and-configuring-remote-task-and-pipeline-resolution).
- The `enable-cluster-resolver` feature flag in the `resolvers-feature-flags` ConfigMap
  in the `tekton-pipelines-resolvers` namespace set to `true`.

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

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
