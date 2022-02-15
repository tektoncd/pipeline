<!--
---
linkTitle: "Migrating from Tekton v1alpha1"
weight: 4000
---
-->

# Migrating From Tekton `v1alpha1` to Tekton `v1beta1`

- [Changes to fields](#changes-to-fields)
- [Changes to input parameters](#changes-to-input-parameters)
- [Replacing `PipelineResources` with `Tasks`](#replacing-pipelineresources-with-tasks)
  - [Replacing a `git` resource](#replacing-a-git-resource)
  - [Replacing a `pullrequest` resource](#replacing-a-pullrequest-resource)
  - [Replacing a `gcs` resource](#replacing-a-gcs-resource)
  - [Replacing an `image` resource](#replacing-an-image-resource)
  - [Replacing a `cluster` resource](#replacing-a-cluster-resource)
- [Changes to `PipelineResources`](#changes-to-pipelineresources)

This document describes the differences between `v1alpha1` Tekton entities and their
`v1beta1` counterparts. It also describes how to replace the supported types of
`PipelineResources` with `Tasks` from the Tekton Catalog of equivalent functionality.

## Changes to fields

In Tekton `v1beta1`, the following fields have been changed:

| Old field | New field |
| --------- | ----------|
| `spec.inputs.params` | [`spec.params`](#changes-to-input-parameters) |
| `spec.inputs` | Removed from `Tasks` |
| `spec.outputs` | Removed from `Tasks` |
| `spec.inputs.resources` | [`spec.resources.inputs`](#changes-to-pipelineresources) |
| `spec.outputs.resources` | [`spec.resources.outputs`](#changes-to-pipelineresources) |

## Changes to input parameters

In Tekton `v1beta1`, input parameters have been moved from `spec.inputs.params` to `spec.params`.

For example, consider the following `v1alpha1` parameters:

```yaml
# Task.yaml (v1alpha1)
spec:
  inputs:
    params:
      - name: ADDR
        description: Address to curl.
        type: string

# TaskRun.yaml (v1alpha1)
spec:
  inputs:
    params:
      - name: ADDR
        value: https://example.com/foo.json
```

The above parameters are now represented as follows in `v1beta1`:

```yaml
# Task.yaml (v1beta1)
spec:
  params:
    - name: ADDR
      description: Address to curl.
      type: string

# TaskRun.yaml (v1beta1)
spec:
  params:
    - name: ADDR
      value: https://example.com/foo.json
```

## Replacing `PipelineResources` with `Tasks`

`PipelineResources` remained in alpha while the other resource kinds were promoted to beta.
Since then, **`PipelineResources` have been deprecated**. We encourage users to use `Tasks` and other replacement 
features instead of `PipelineResources`. Read more about the deprecation in [TEP-0074](https://github.com/tektoncd/community/blob/main/teps/0074-deprecate-pipelineresources.md).

_More on the reasoning and what's left to do in
[Why aren't PipelineResources in Beta?](resources.md#why-aren-t-pipelineresources-in-beta)._

To ease migration away from `PipelineResources`
[some types have an equivalent `Task` in the Catalog](#replacing-pipelineresources-with-tasks).
To use these replacement `Tasks` you will need to combine them with your existing `Tasks` via a `Pipeline`.

For example, if you were using this `Task` which was fetching from `git` and building with
`Kaniko`:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: Task
metadata:
  name: build-push-kaniko
spec:
  inputs:
    resources:
      - name: workspace
        type: git
    params:
      - name: pathToDockerFile
        description: The path to the dockerfile to build
        default: /workspace/workspace/Dockerfile
      - name: pathToContext
        description: The build context used by Kaniko
        default: /workspace/workspace
  outputs:
    resources:
      - name: builtImage
        type: image
  steps:
    - name: build-and-push
      image: gcr.io/kaniko-project/executor:v0.17.1
      env:
        - name: "DOCKER_CONFIG"
          value: "/tekton/home/.docker/"
      args:
        - --dockerfile=$(inputs.params.pathToDockerFile)
        - --destination=$(outputs.resources.builtImage.url)
        - --context=$(inputs.params.pathToContext)
        - --oci-layout-path=$(inputs.resources.builtImage.path)
      securityContext:
        runAsUser: 0
```

To do the same thing with the `git` catalog `Task` and the kaniko `Task` you will need to combine them in a
`Pipeline`.

For example this Pipeline uses the Kaniko and `git` catalog Tasks:

```yaml
apiVersion: tekton.dev/v1beta1
kind: Pipeline
metadata:
  name: kaniko-pipeline
spec:
  params:
    - name: git-url
    - name: git-revision
    - name: image-name
    - name: path-to-image-context
    - name: path-to-dockerfile
  workspaces:
    - name: git-source
  tasks:
    - name: fetch-from-git
      taskRef:
        name: git-clone
      params:
        - name: url
          value: $(params.git-url)
        - name: revision
          value: $(params.git-revision)
      workspaces:
        - name: output
          workspace: git-source
    - name: build-image
      taskRef:
        name: kaniko
      params:
        - name: IMAGE
          value: $(params.image-name)
        - name: CONTEXT
          value: $(params.path-to-image-context)
        - name: DOCKERFILE
          value: $(params.path-to-dockerfile)
      workspaces:
        - name: source
          workspace: git-source
  # If you want you can add a Task that uses the IMAGE_DIGEST from the kaniko task
  # via $(tasks.build-image.results.IMAGE_DIGEST) - this was a feature we hadn't been
  # able to fully deliver with the Image PipelineResource!
```

_Note that [the `image` `PipelineResource` is gone in this example](#replacing-an-image-resource) (replaced with
a [`result`](tasks.md#emitting-results)), and also that now the `Task` doesn't need to know anything
about where the files come from that it builds from._

### Replacing a `git` resource

You can replace a `git` resource with the [`git-clone` Catalog `Task`](https://github.com/tektoncd/catalog/tree/main/task/git-clone).

### Replacing a `pullrequest` resource

You can replace a `pullrequest` resource with the [`pullrequest` Catalog `Task`](https://github.com/tektoncd/catalog/tree/main/task/pull-request).

### Replacing a `gcs` resource

You can replace a `gcs` resource with the [`gcs` Catalog `Task`](https://github.com/tektoncd/catalog/tree/main/task/gcs-generic).

### Replacing an `image` resource

Since the `image` resource is simply a way to share the digest of a built image with subsequent
`Tasks` in your `Pipeline`, you can use [`Task` results](tasks.md#emitting-results) to
achieve equivalent functionality.

For examples of replacing an `image` resource, see the following Catalog `Tasks`:

- The [Kaniko Catalog `Task`](https://github.com/tektoncd/catalog/blob/v1beta1/kaniko/)
  illustrates how to write the digest of an image to a result.
- The [Buildah Catalog `Task`](https://github.com/tektoncd/catalog/blob/v1beta1/buildah/)
  illustrates how to accept an image digest as a parameter.

### Replacing a `cluster` resource

You can replace a `cluster` resource with the [`kubeconfig-creator` Catalog `Task`](https://github.com/tektoncd/catalog/tree/main/task/kubeconfig-creator).

### Replacing a `cloudEvent` resource

You can replace a `cloudEvent` resource with the [`CloudEvent` Catalog `Task`](https://github.com/tektoncd/catalog/tree/main/task/cloudevent).

## Changes to PipelineResources

In Tekton `v1beta1`, `PipelineResources` have been moved from `spec.input.resources`
and `spec.output.resources` to `spec.resources.inputs` and `spec.resources.outputs`,
respectively.

For example, consider the following `v1alpha1` definition:

```yaml
# Task.yaml (v1alpha1)
spec:
  inputs:
    resources:
      - name: skaffold
        type: git
  outputs:
    resources:
      - name: baked-image
        type: image

# TaskRun.yaml (v1alpha1)
spec:
  inputs:
    resources:
      - name: skaffold
        resourceSpec:
          type: git
          params:
            - name: revision
              value: v0.32.0
            - name: url
              value: https://github.com/GoogleContainerTools/skaffold
  outputs:
    resources:
      - name: baked-image
        resourceSpec:
          - type: image
            params:
              - name: url
                value: gcr.io/foo/bar
```

The above definition becomes the following in `v1beta1`:

```yaml
# Task.yaml (v1beta1)
spec:
  resources:
    inputs:
      - name: src-repo
        type: git
    outputs:
      - name: baked-image
        type: image

# TaskRun.yaml (v1beta1)
spec:
  resources:
    inputs:
      - name: src-repo
        resourceSpec:
          type: git
          params:
            - name: revision
              value: main
            - name: url
              value: https://github.com/tektoncd/pipeline
    outputs:
      - name: baked-image
        resourceSpec:
          - type: image
            params:
              - name: url
                value: gcr.io/foo/bar
```
