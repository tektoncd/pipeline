# PipelineResources Logic

## How are resources shared between tasks

> :warning: **`PipelineResources` are [deprecated](deprecations.md#deprecation-table).**
>
> Consider using replacement features instead. Read more in [documentation](migrating-v1alpha1-to-v1beta1.md#replacing-pipelineresources-with-tasks)
> and [TEP-0074](https://github.com/tektoncd/community/blob/main/teps/0074-deprecate-pipelineresources.md).

`PipelineRun` uses PVC to share `PipelineResources` between tasks. PVC volume is
mounted on path `/pvc` by PipelineRun.

- If a resource in a task is declared as output then the `TaskRun` controller
  adds a step to copy each output resource to the directory path
  `/pvc/task_name/resource_name`.

- If an input resource includes `from` condition then the `TaskRun` controller
  adds a step to copy from PVC directory path:
  `/pvc/previous_task/resource_name`.

If neither of these conditions are met, the PVC will not be created nor will the
GCS storage / S3 buckets be used.

Another alternative is to use a GCS storage or S3 bucket to share the artifacts.
This can be configured using a ConfigMap with the name `config-artifact-bucket`.

See the
[installation docs](../install.md#how-are-resources-shared-between-tasks) for
configuration details.

Both options provide the same functionality to the pipeline. The choice is based
on the infrastructure used, for example in some Kubernetes platforms, the
creation of a persistent volume could be slower than uploading/downloading files
to a bucket, or if the the cluster is running in multiple zones, the access to
the persistent volume can fail.

## How inputs are handled

Input resources, like source code (git) or artifacts, are dumped at path
`/workspace/task_resource_name`.

- If input resource is declared as below, then resource will be copied to
  `/workspace/task_resource_name` directory `from` depended task PVC directory
  `/pvc/previous_task/resource_name`.

  ```yaml
  kind: Task
  metadata:
    name: get-gcs-task
    namespace: default
  spec:
    resources:
      inputs:
        - name: gcs-workspace
          type: storage
  ```

- Resource definition in task can have custom target directory. If `targetPath`
  is mentioned in task input resource as below then resource will be copied to
  `/workspace/outputstuff` directory `from` depended task PVC directory
  `/pvc/previous_task/resource_name`.

  ```yaml
  kind: Task
  metadata:
    name: get-gcs-task
    namespace: default
  spec:
    resources:
      inputs:
        - name: gcs-workspace
          type: storage
          targetPath: /workspace/outputstuff
  ```

## How outputs are handled

Output resources, like source code (git) or artifacts (storage resource), are
expected in directory path `/workspace/output/resource_name`.

- If resource has an output "action" like upload to blob storage, then the
  container step is added for this action.
- If there is PVC volume present (TaskRun holds owner reference to PipelineRun)
  then copy step is added as well.

- If the output resource is declared then the copy step includes resource being
  copied to PVC to path `/pvc/task_name/resource_name` from
  `/workspace/output/resource_name` like the following example.

  ```yaml
  kind: Task
  metadata:
    name: get-gcs-task
    namespace: default
  spec:
    resources:
      outputs:
        - name: gcs-workspace
          type: storage
  ```

- Same as input, if the output resource is declared with `TargetPath` then the
  copy step includes resource being copied to PVC to path
  `/pvc/task_name/resource_name` from `/workspace/outputstuff` like the
  following example.

  ```yaml
  kind: Task
  metadata:
    name: get-gcs-task
    namespace: default
  spec:
    resources:
      outputs:
        - name: gcs-workspace
          type: storage
          targetPath: /workspace/outputstuff
  ```