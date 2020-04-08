# Migrating From v1alpha1 to v1beta1

- [Migrating From v1alpha1 to v1beta1](#migrating-from-v1alpha1-to-v1beta1)
  - [Inputs and Outputs](#inputs-and-outputs)
    - [Input Params](#input-params)
    - [Input and Output PipelineResources](#input-and-output-pipelineresources)
  - [PipelineResources and Catalog Tasks](#pipelineresources-and-catalog-tasks)
    - [Git Resource](#git-resource)
    - [Pull Request Resource](#pull-request-resource)
    - [Image Resource](#image-resource)
    - [GCS Resource](#gcs-resource)

This doc describes the changes you'll need to make when converting your `Task`s,
`Pipeline`s, `TaskRun`s, and `PipelineRun`s from the `v1alpha1` `apiVersion` to
`v1beta1`.

## Inputs and Outputs

The `spec.inputs` and `spec.outputs` fields in `Tasks` are removed in `v1beta1`.
`spec.inputs.params` is moved to `spec.params`. `spec.inputs.resources` becomes
`spec.resources.inputs` and `spec.outputs.resources` becomes `spec.resources.outputs`.

#### Input Params

Params move from `spec.inputs.params` in v1alpha1 to just `spec.params` in v1beta1.

Example v1alpha1 Params:

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

Equivalent v1beta1 Params:

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

#### Input and Output PipelineResources

In v1alpha1, `PipelineResources` were stored under `spec.inputs.resources` and
`spec.outputs.resources`. In v1beta1 these change to `spec.resources.inputs` and
`spec.resources.outputs`.

Example v1alpha1 PipelineResources:

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

Equivalent v1beta1 PipelineResources:

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

## PipelineResources and Catalog Tasks

PipelineResources are remaining in alpha while the other resource kinds are
promoted to beta. This is part of a deliberate effort to migrate users away
from PipelineResources onto a broader suite of reusable Tasks provided by
the Tekton catalog.

In the near-term `PipelineResources` will continue to be supported by Tekton.
At some point the feature may well be deprecated and subsequently replaced.

To ease migration away from PipelineResources some types have an equivalent
Task in the Catalog.

### Git Resource

The [`git-clone` Task in the Catalog](https://github.com/tektoncd/catalog/tree/v1beta1/git)
provides an equivalent to the Git Resource.

### Pull Request Resource

The [`pullrequest` Task in the Catalog](https://github.com/tektoncd/catalog/tree/v1beta1/pullrequest)
provides an equivalent to the PullRequest Resource.

### Image Resource

The Image PipelineResource does not come with specific behaviour, it's simply a way to share
a built image's digest with subsequent `Tasks` in a `Pipeline`. The same effect can be achieved
using [`Task` results](./tasks.md#results) and [`Task` params](./tasks.md#parameters)

For an example of how the Image PipelineResource can be replaced, see
the [Kaniko Catalog Task](https://github.com/tektoncd/catalog/blob/v1beta1/kaniko/)
which demonstrates writing the image digest to a result, and the
[Buildah Catalog Task](https://github.com/tektoncd/catalog/blob/v1beta1/buildah/)
which shows how to accept an image digest as a parameter.

### GCS Resource

The [`gcs` Tasks in the Catalog](https://github.com/tektoncd/catalog/tree/v1beta1/gcs) provide
an equivalent to the GCS Resource.
