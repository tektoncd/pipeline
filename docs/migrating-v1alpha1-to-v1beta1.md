<!--
---
linkTitle: "Migrating from Tekton v1alpha1"
weight: 102
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
See ["Replacing PipelineResources with Tasks"](https://github.com/tektoncd/pipeline/blob/main/docs/pipelineresources.md#replacing-pipelineresources-with-tasks) for information and examples on how to replace PipelineResources when migrating from v1alpha1 to v1beta1.

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
