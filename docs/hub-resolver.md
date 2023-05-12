<!--
---
linkTitle: "Hub Resolver"
weight: 311
---
-->

# Hub Resolver

Use resolver type `hub`.

## Parameters

| Param Name       | Description                                                                   | Example Value                                              |
|------------------|-------------------------------------------------------------------------------|------------------------------------------------------------|
| `catalog`        | The catalog from where to pull the resource (Optional)                        | Default:  `tekton-catalog-tasks` (for `task` kind);  `tekton-catalog-pipelines` (for `pipeline` kind)                                        |
| `type`           | The type of Hub from where to pull the resource (Optional). Either `artifact` or `tekton` | Default:  `artifact`                                         |
| `kind`           | Either `task` or `pipeline` (Optional)                                        | Default: `task`                                                     |
| `name`           | The name of the task or pipeline to fetch from the hub                        | `golang-build`                                             |
| `version`        | Version of task or pipeline to pull in from hub. Wrap the number in quotes!   | `"0.5.0"`                                                    |

The Catalogs in the Artifact Hub follows the semVer (i.e.` <major-version>.<minor-version>.0`) and the Catalogs in the Tekton Hub follows the simplified semVer (i.e. `<major-version>.<minor-version>`). Both full and simplified semantic versioning will be accepted by the `version` parameter. The Hub Resolver will map the version to the format expected by the target Hub `type`.

## Requirements

- A cluster running Tekton Pipeline v0.41.0 or later.
- The [built-in remote resolvers installed](./install.md#installing-and-configuring-remote-task-and-pipeline-resolution).
- The `enable-hub-resolver` feature flag in the `resolvers-feature-flags` ConfigMap in the
  `tekton-pipelines-resolvers` namespace set to `true`.
- [Beta features](./additional-configs.md#beta-features) enabled.

## Configuration

This resolver uses a `ConfigMap` for its settings. See
[`../config/resolvers/hubresolver-config.yaml`](../config/resolvers/hubresolver-config.yaml)
for the name, namespace and defaults that the resolver ships with.

### Options

| Option Name                 | Description                                          | Example Values         |
|-----------------------------|------------------------------------------------------|------------------------|
| `default-tekton-hub-catalog`| The default tekton hub catalog from where to pull the resource.| `Tekton`               |
| `default-artifact-hub-task-catalog`| The default artifact hub catalog from where to pull the resource for task kind.| `tekton-catalog-tasks`               |
| `default-artifact-hub-pipeline-catalog`| The default artifact hub catalog from where to pull the resource for pipeline kind.  | `tekton-catalog-pipelines`               |
| `default-kind`              | The default object kind for references.              | `task`, `pipeline`     |
| `default-type`              | The default hub from where to pull the resource.     | `artifact`, `tekton`   |


### Configuring the Hub API endpoint

The Hub Resolver supports to resolve resources from the [Artifact Hub](https://artifacthub.io/) and the [Tekton Hub](https://hub.tekton.dev/),
which can be configured by setting the `type` field of the resolver. 

*(Please note that the [Tekton Hub](https://hub.tekton.dev/) will be deprecated after [migration to the Artifact Hub](https://github.com/tektoncd/hub/issues/667) is done.)*

When setting the `type` field to `artifact`, the resolver will hit the public hub api at https://artifacthub.io/ by default
but you can configure your own (for example to use a private hub
instance) by setting the `ARTIFACT_HUB_API` environment variable in
[`../config/resolvers/resolvers-deployment.yaml`](../config/resolvers/resolvers-deployment.yaml). Example:

```yaml
env
- name: ARTIFACT_HUB_API
  value: "https://artifacthub.io/"
```

When setting the `type` field to `tekton`, you **must** configure your own instance of the Tekton Hub by setting the `TEKTON_HUB_API` environment variable in
[`../config/resolvers/resolvers-deployment.yaml`](../config/resolvers/resolvers-deployment.yaml). Example:

```yaml
env
- name: TEKTON_HUB_API
  value: "https://api.private.hub.instance.dev"
```

The Tekton Hub deployment guide can be found [here](https://github.com/tektoncd/hub/blob/main/docs/DEPLOYMENT.md).

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
      value: tekton-catalog-tasks
    - name: type # optional
      value: artifact 
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
      value: tekton-catalog-pipelines 
    - name: type # optional
      value: artifact
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
