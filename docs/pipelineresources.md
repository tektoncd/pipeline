<!--
---
linkTitle: "Replacing PipelineResources with Tasks"
weight: 207
---
-->

## Replacing PipelineResources with Tasks

`PipelineResources` remained in alpha while the other resource kinds were promoted to beta.
Since then, **`PipelineResources` have been removed**.
Read more about the deprecation in [TEP-0074](https://github.com/tektoncd/community/blob/main/teps/0074-deprecate-pipelineresources.md).

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
