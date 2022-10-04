# Hub Resolver

Use resolver type `hub`.

## Parameters

| Param Name       | Description                                                                   | Example Value                                              |
|------------------|-------------------------------------------------------------------------------|------------------------------------------------------------|
| `catalog`        | The catalog from where to pull the resource (Optional)                        | Default:  `Tekton`                                         |
| `kind`           | Either `task` or `pipeline`                                                   | `task`                                                     |
| `name`           | The name of the task or pipeline to fetch from the hub                        | `golang-build`                                             |
| `version`        | Version of task or pipeline to pull in from hub. Wrap the number in quotes!   | `"0.5"`                                                    |

## Requirements

- A cluster running Tekton Pipeline v0.40.0 or later, with the `alpha` feature gate enabled.
- The [built-in remote resolvers installed](./install.md#installing-and-configuring-remote-task-and-pipeline-resolution).
- The `enable-hub-resolver` feature flag in the `resolvers-feature-flags` ConfigMap in the
  `tekton-pipelines-resolvers` namespace set to `true`.

## Configuration

This resolver uses a `ConfigMap` for its settings. See
[`../config/resolvers/hubresolver-config.yaml`](../config/resolvers/hubresolver-config.yaml)
for the name, namespace and defaults that the resolver ships with.

### Options

| Option Name       | Description                                          | Example Values     |
|-------------------|------------------------------------------------------|--------------------|
| `default-catalog` | The default catalog from where to pull the resource. | `tekton`           |
| `default-kind`    | The default object kind for references.              | `task`, `pipeline` |


### Configuring the Hub API endpoint

By default this resolver will hit the public hub api at https://hub.tekton.dev/
but you can configure your own (for example to use a private hub
instance) by setting the `HUB_API` environment variable in
[`../config/resolvers/resolvers-deployment.yaml`](../config/resolvers/resolvers-deployment.yaml). Example:

```yaml
env
- name: HUB_API
  value: "https://api.hub.tekton.dev/"
```

## Usage

### Task Resolution

```yaml
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: remote-task-reference
spec:
  taskRef:
    resolver: hub
    params:
    - name: catalog # optional
      value: Tekton
    - name: kind
      value: task
    - name: name
      value: git-clone
    - name: version
      value: "0.6"
```

### Pipeline Resolution

```yaml
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: hub-demo
spec:
  pipelineRef:
    resolver: hub
    params:
    - name: catalog # optional
      value: Tekton 
    - name: kind
      value: pipeline
    - name: name
      value: buildpacks
    - name: version
      value: "0.1"
  # Note: the buildpacks pipeline requires parameters.
  # Resolution of the pipeline will succeed but the PipelineRun
  # overall will not succeed without those parameters.
```

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
