<!--
---
linkTitle: "Bundles Resolver"
weight: 308
---
-->

# Bundles Resolver

## Resolver Type

This Resolver responds to type `bundles`.

## Parameters

| Param Name       | Description                                                                   | Example Value                                              |
|------------------|-------------------------------------------------------------------------------|------------------------------------------------------------|
| `serviceAccount` | The name of the service account to use when constructing registry credentials | `default`                                                  |
| `bundle`         | The bundle url pointing at the image to fetch                                 | `gcr.io/tekton-releases/catalog/upstream/golang-build:0.1` |
| `name`           | The name of the resource to pull out of the bundle                            | `golang-build`                                             |
| `kind`           | The resource kind to pull out of the bundle                                   | `task`                                                     |

## Requirements

- A cluster running Tekton Pipeline v0.41.0 or later.
- The [built-in remote resolvers installed](./install.md#installing-and-configuring-remote-task-and-pipeline-resolution).
- The `enable-bundles-resolver` feature flag in the `resolvers-feature-flags` ConfigMap 
  in the `tekton-pipelines-resolvers` namespace set to `true`.
- [Beta features](./additional-configs.md#beta-features) enabled.

## Configuration

This resolver uses a `ConfigMap` for its settings. See
[`../config/resolvers/bundleresolver-config.yaml`](../config/resolvers/bundleresolver-config.yaml)
for the name, namespace and defaults that the resolver ships with.

### Options

| Option Name               | Description                                                  | Example Values        |
|---------------------------|--------------------------------------------------------------|-----------------------|
| `default-service-account` | The default service account name to use for bundle requests. | `default`, `someuser` |
| `default-kind`            | The default layer kind in the bundle image.                  | `task`, `pipeline`    |

## Usage

### Task Resolution

```yaml
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: remote-task-reference
spec:
  taskRef:
    resolver: bundles
    params:
    - name: bundle
      value: docker.io/ptasci67/example-oci@sha256:053a6cb9f3711d4527dd0d37ac610e8727ec0288a898d5dfbd79b25bcaa29828
    - name: name
      value: hello-world
    - name: kind
      value: task
```

### Pipeline Resolution

Unfortunately the Tekton Catalog does not publish pipelines at the
moment. Here's an example PipelineRun that talks to a private registry
but won't work unless you tweak the `bundle` field to point to a
registry with a pipeline in it:

```yaml
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: bundle-demo
spec:
  pipelineRef:
    resolver: bundles
    params:
    - name: bundle
      value: 10.96.190.208:5000/simple/pipeline:latest
    - name: name
      value: hello-pipeline
    - name: kind
      value: pipeline
  params:
  - name: username
    value: "tekton pipelines"
```

## `ResolutionRequest` Status
`ResolutionRequest.Status.RefSource` field captures the source where the remote resource came from. It includes the 3 subfields: `url`, `digest` and `entrypoint`.
- `uri`: The image repository URI
- `digest`: The map of the algorithm portion -> the hex encoded portion of the image digest.
- `entrypoint`: The resource name in the OCI bundle image.

Example:
- TaskRun Resolution
```yaml
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: remote-task-reference
spec:
  taskRef:
    resolver: bundles
    params:
    - name: bundle
      value: gcr.io/tekton-releases/catalog/upstream/git-clone:0.7
    - name: name
      value: git-clone
    - name: kind
      value: task
  params:
    - name: url
      value: https://github.com/octocat/Hello-World
  workspaces:
    - name: output
      volumeClaimTemplate:
        spec:
          accessModes:
            - ReadWriteOnce
          resources:
            requests:
              storage: 500Mi
```

- `ResolutionRequest`
```yaml
apiVersion: resolution.tekton.dev/v1beta1
kind: ResolutionRequest
metadata:
  ...
  labels:
    resolution.tekton.dev/type: bundles
  name: bundles-21ad80ec13f3e8b73fed5880a64d4611
  ...
spec:
  params:
  - name: bundle
    value: gcr.io/tekton-releases/catalog/upstream/git-clone:0.7
  - name: name
    value: git-clone
  - name: kind
    value: task
status:
  annotations: ...
  ...
  data: xxx
  observedGeneration: 1
  refSource:
    digest:
      sha256: f51ca50f1c065acba8290ef14adec8461915ecc5f70a8eb26190c6e8e0ededaf
    entryPoint: git-clone
    uri: gcr.io/tekton-releases/catalog/upstream/git-clone
```

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
