# Migrating From Tekton `v1alpha1` to Tekton `v1beta1`

- [Changes to fields](#changes-to-fields)
- [Changes to input parameters](#changes-to-input-parameters)
- [Changes to `PipelineResources`](#changes-to-pipelineresources)
- [Replacing `PipelineResources` with `Tasks`](#replacing-pipelineresources-with-tasks)
  - [Replacing a `git` resource](#replacing-a-git-resource)
  - [Replacing a `pullrequest` resource](#replacing-a-pullrequest-resource)
  - [Replacing a `gcs` resource](#replacing-a-gcs-resource)
  - [Replacing an `image` resource](#replacing-an-image-resource)

This document describes the differences between `v1alpha` Tekton entities and their
`v1beta1` counterparts. It also describes how to replace the supported types of
`PipelineResources` with `Tasks` from the Tekton Catalog of equivalent functionality.

## Changes to fields

In Tekton `v1beta`, the following fields have been changed:

| Old field | New field |
| --------- | ----------|
| `spec.inputs.params` | `spec.params` |
|  `spec.inputs.resources` | `spec.resources.inputs` |
| `spec.outputs.resources` | `spec.resources.outputs` |
| `spec.inputs` | Removed from `Tasks` |
| `spec.outputs` | Removed from `Tasks` | 


## Changes to input parameters

In Tekton `v1beta`, input parameters have been moved from `spec.inputs.params` to `spec.params`. 

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

## Changes to `PipelineResources`

In Tekton `v1beta`, `PipelineResources` have been moved from `spec.input.respources`
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
            value: master
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

## Replacing `PipelineResources` with `Tasks`

While Tekton Pipelines has advanced to beta, `PipelineResources` remain in alpha
in a deliberate effort to migrate users away from `PipelineResources` and onto
a broader suite of reusable `Tasks` in the Tekton Catalog.

In the near term, Tekton continues to support `PipelineResources`; however, they
will be eventually deprecated and removed from Tekton.

You can replace the types of `PipelineResources` listed below with `Tasks` from the
Tekton Catalog of equivalent functionality as described below.

### Replacing a `git` resource

You can replace a `git` resource with the [`git-clone` Catalog `Task`](https://github.com/tektoncd/catalog/tree/v1beta1/git).

### Replacing a `pullrequest` resource

You can replace a `pullrequest` resource with the [`pullrequest` Catalog `Task`](https://github.com/tektoncd/catalog/tree/v1beta1/pullrequest).

### Replacing a `gcs` resource

You can replace a `gcs` resource with the [`gcs` Catalog `Task`](https://github.com/tektoncd/catalog/tree/v1beta1/gcs).

### Replacing an `image` resource

Since the `image` resource is simply a way to share the digest of a built image with subsequent
`Tasks` in your `Pipeline`, you can use [`Task` results](tasks.md#storing-execution-results) to
achieve equivalent functionality.

For examples of replacing an `image` resource, see the following Catalog `Tasks`:

- The [Kaniko Catalog `Task`](https://github.com/tektoncd/catalog/blob/v1beta1/kaniko/)
  illustrates how to write the digest of an image to a result.
- The [Buildah Catalog `Task`](https://github.com/tektoncd/catalog/blob/v1beta1/buildah/)
  illustrates how to accept an image digest as a parameter.
