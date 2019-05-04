# Developer docs

This document is aimed at helping maintainers/developers of project understand
the complexity.

## How are resources shared between tasks

`PipelineRun` uses PVC to share resources between tasks. PVC volume is mounted
on path `/pvc` by PipelineRun.

- If a resource in a task is declared as output then the `TaskRun` controller
  adds a step to copy each output resource to the directory path
  `/pvc/task_name/resource_name`.

- If an input resource includes `from` condition then the `TaskRun` controller
  adds a step to copy from PVC directory path:
  `/pvc/previous_task/resource_name`.

Another alternatives is to use a GCS storage bucket to share the artifacts. This
can be configured using a ConfigMap with the name `config-artifact-bucket` with
the following attributes:

- location: the address of the bucket (for example gs://mybucket)
- bucket.service.account.secret.name: the name of the secret that will contain
  the credentials for the service account with access to the bucket
- bucket.service.account.secret.key: the key in the secret with the required
  service account json. The bucket is recommended to be configured with a
  retention policy after which files will be deleted.

Both options provide the same functionality to the pipeline. The choice is based
on the infrastructure used, for example in some Kubernetes platforms, the
creation of a persistent volume could be slower than uploading/downloading files
to a bucket, or if the the cluster is running in multiple zones, the access to
the persistent volume can fail.

## How are inputs handled

Input resources, like source code (git) or artifacts, are dumped at path
`/workspace/task_resource_name`. Resource definition in task can have custom
target directory. If `targetPath` is mentioned in task input then the
controllers are responsible for adding container definitions to create
directories and also to fetch the versioned artifacts into that directory.

## How are outputs handled

Output resources, like source code (git) or artifacts (storage resource), are
expected in directory path `/workspace/output/resource_name`.

- If resource has an output "action" like upload to blob storage, then the
  container step is added for this action.
- If there is PVC volume present (TaskRun holds owner reference to PipelineRun)
  then copy step is added as well.

- If the resource is declared only in output but not in input for task then the
  copy step includes resource being copied to PVC to path
  `/pvc/task_name/resource_name` from `/workspace/output/resource_name` like the
  following example.

  ```yaml
  kind: Task
  metadata:
    name: get-gcs-task
    namespace: default
  spec:
    outputs:
      resources:
        - name: gcs-workspace
          type: storage
  ```

- If the resource is declared only in output but not in input for task and the
  resource defined with `TargetPath` then the copy step includes resource being
  copied to PVC to path `/pvc/task_name/resource_name` from
  `/workspace/outputstuff` like the following example.

  ```yaml
  kind: Task
  metadata:
    name: get-gcs-task
    namespace: default
  spec:
    outputs:
      resources:
        - name: gcs-workspace
          type: storage
          targetPath: /workspace/outputstuff
  ```

- If the resource is declared both in input and output for task the then copy
  step includes resource being copied to PVC to path
  `/pvc/task_name/resource_name` from `/workspace/random-space/` if input
  resource has custom target directory (`random-space`) declared like the
  following example.

  ```yaml
  kind: Task
  metadata:
    name: get-gcs-task
    namespace: default
  spec:
    inputs:
      resources:
        - name: gcs-workspace
          type: storage
          targetPath: random-space
    outputs:
      resources:
        - name: gcs-workspace
          type: storage
  ```

  - If resource is declared both in input and output for task without custom
    target directory then copy step includes resource being copied to PVC to
    path `/pvc/task_name/resource_name` from `/workspace/resource_name/` like
    the following example.

  ```yaml
  kind: Task
  metadata:
    name: get-gcs-task
    namespace: default
  spec:
    inputs:
      resources:
        - name: gcs-workspace
          type: storage
    outputs:
      resources:
        - name: gcs-workspace
          type: storage
  ```

## Entrypoint rewriting and step ordering

`Entrypoint` is injected into the `Task` Container(s), wraps the `Task` step to
manage the execution order of the containers. The `entrypoint` binary has the
following arguments:

- `wait_file` - If specified, file to wait for
- `wait_file_content` - If specified, wait until the file has non-zero size
- `post_file` - If specified, file to write upon completion
- `entrypoint` - The command to run in the image being wrapped

As part of the PodSpec created by `TaskRun` the entrypoint for each `Task` step
is changed to the entrypoint binary with the mentioned arguments and a volume
with the binary and file(s) is mounted.

If the image is a private registry, the service account should include an
[ImagePullSecret](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#add-imagepullsecrets-to-a-service-account)

## Builder namespace on containers

The `/builder/` namespace is reserved on containers for various system tools,
such as the following:

- The environment variable HOME is set to `/builder/home`, used by the builder
  tools and injected on into all of the step containers
- Default location for output-images `/builder/output-images`

## Handling of injected sidecars

Tekton has to take some special steps to support sidecars that are injected into
TaskRun Pods. Without intervention sidecars will typically run for the entire
lifetime of a Pod but in Tekton's case it's desirable for the sidecars to run
only as long as Steps take to complete. There's also a need for Tekton to
schedule the sidecars to start before a Task's Steps begin, just in case the
Steps rely on a sidecars behaviour, for example to join an Istio service mesh.
To handle all of this, Tekton Pipelines implements the following lifecycle
for sidecar containers:

First, the [Downward API](https://kubernetes.io/docs/tasks/inject-data-application/downward-api-volume-expose-pod-information/#the-downward-api)
is used to project an annotation on the TaskRun's Pod into the `entrypoint`
container as a file. The annotation starts as an empty string, so the file
projected by the downward API has zero length. The entrypointer spins, waiting
for that file to have non-zero size.

The sidecar containers start up. Once they're all in a ready state, the
annotation is populated with string "READY", which in turn populates the
Downward API projected file. The entrypoint binary recognizes
that the projected file has a non-zero size and allows the Task's steps to
begin.

On completion of all steps in a Task the TaskRun reconciler stops any
sidecar containers. The `Image` field of any sidecar containers is swapped
to the nop image. Kubernetes observes the change and relaunches the container
with updated container image. The nop container image exits. The container
is considered `Terminated` by Kubernetes and the TaskRun's Pod stops.
