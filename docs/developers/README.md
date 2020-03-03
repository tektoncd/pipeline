# Developer docs

This document is aimed at helping maintainers/developers of project understand
the complexity.

## How are resources shared between tasks

`PipelineRun` uses PVC to share `PipelineResources` between tasks. PVC volume is
mounted on path `/pvc` by PipelineRun.

- If a resource in a task is declared as output then the `TaskRun` controller
  adds a step to copy each output resource to the directory path
  `/pvc/task_name/resource_name`.

- If an input resource includes `from` condition then the `TaskRun` controller
  adds a step to copy from PVC directory path:
  `/pvc/previous_task/resource_name`.

If neither of these conditions are met, the PVC will not be created nor will
the GCS storage / S3 buckets be used.

Another alternative is to use a GCS storage or S3 bucket to share the artifacts.
This can be configured using a ConfigMap with the name `config-artifact-bucket`.

See the [installation docs](../install.md#how-are-resources-shared-between-tasks) for configuration details.

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

The `/tekton/` directory is reserved on containers for internal usage. Examples
of how this directory is used:

- Task Results are written to `/tekton/results`
- Various tools like the entrypoint are placed in `/tekton/tools`
- The termination log message is written to `/tekton/termination`
- Sequencing step containers is done using both `/tekton/downward/ready`
and numbered files in `/tekton/tools`

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
with updated container image. The nop container image exits immediately
*because it does not provide the command that the sidecar is configured to run*.
The container is considered `Terminated` by Kubernetes and the TaskRun's Pod
stops.

There are known issues with the existing implementation of sidecars:

- When the `nop` image does provide the sidecar's command, the sidecar will continue to
run even after `nop` has been swapped into the sidecar container's image
field. See [the issue tracking this bug](https://github.com/tektoncd/pipeline/issues/1347)
for the issue tracking this bug. Until this issue is resolved the best way to avoid it is to
avoid overriding the `nop` image when deploying the tekton controller, or
ensuring that the overridden `nop` image contains as few commands as possible.

- `kubectl get pods` will show a Completed pod when a sidecar exits successfully
but an Error when the sidecar exits with an error. This is only apparent when
using `kubectl` to get the pods of a TaskRun, not when describing the Pod
using `kubectl describe pod ...` nor when looking at the TaskRun, but can be quite
confusing.

## How task results are defined and outputted by a task

Tasks can define results by adding a result on the task spec.
This is an example:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: Task
metadata:
  name: print-date
  annotations:
    description: |
      A simple task that prints the date to make sure your cluster / Tekton is working properly.
spec:
  results:
    - name: "current-date"
      description: "The current date"
  steps:
    - name: print-date
      image: bash:latest
      args:
        - "-c"
        - |
          date > /tekton/results/current-date
```

The result is added to a file name with the specified result's name into the `/tekton/results` folder. This is then added to the
task run status.
Internally the results are a new argument `-results`to the entrypoint defined for the task. A user can defined more than one result for a
single task.

For this task definition,

```yaml
apiVersion: tekton.dev/v1alpha1
kind: Task
metadata:
  name: print-date
  annotations:
    description: |
      A simple task that prints the date to make sure your cluster / Tekton is working properly.
spec:
  results:
    - name: current-date-unix-timestamp
      description: The current date in unix timestamp format
    - name: current-date-human-readable
      description: The current date in humand readable format
  steps:
    - name: print-date-unix-timestamp
      image: bash:latest
      script: |
        #!/usr/bin/env bash
        date +%s | tee /tekton/results/current-date-unix-timestamp
    - name: print-date-humman-readable
      image: bash:latest
      script: |
        #!/usr/bin/env bash
        date | tee /tekton/results/current-date-human-readable
```

you end up with this task run status:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: TaskRun
...
status:
...
  taskResults:
  - name: current-date-human-readable
    value: |
      Wed Jan 22 19:47:26 UTC 2020
  - name: current-date-unix-timestamp
    value: |
      1579722445
```

Instead of hardcoding the path to the result file, the user can also use a variable. So `/tekton/results/current-date-unix-timestamp` can be replaced with: `$(results.current-date-unix-timestamp.path)`. This is more flexible if the path to result files ever changes.

### Known issues

- Task Results are returned to the TaskRun controller via the container's
termination log. At time of writing this has a capped maximum size of ["2048 bytes or 80 lines, whichever is smaller"](https://kubernetes.io/docs/tasks/debug-application-cluster/determine-reason-pod-failure/#customizing-the-termination-message).

## How task results can be used in pipeline's tasks

Now that we have tasks that can return a result, the user can refer to a task result in a pipeline by using the syntax
`$(tasks.<task name>.results.<result name>)`. This will substitute the task result at the location of the variable.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: Pipeline
metadata:
  name: sum-and-multiply-pipeline
    #...
  tasks:
    - name: sum-inputs
    #...
    - name: multiply-inputs
    #...
- name: sum-and-multiply
      taskRef:
        name: sum
      params:
        - name: a
          value: "$(tasks.multiply-inputs.results.product)$(tasks.sum-inputs.results.sum)"
        - name: b
          value: "$(tasks.multiply-inputs.results.product)$(tasks.sum-inputs.results.sum)"
```

This results in:

```shell
tkn pipeline start sum-and-multiply-pipeline
? Value for param `a` of type `string`? (Default is `1`) 10
? Value for param `b` of type `string`? (Default is `1`) 15
Pipelinerun started: sum-and-multiply-pipeline-run-rgd9j

In order to track the pipelinerun progress run:
tkn pipelinerun logs sum-and-multiply-pipeline-run-rgd9j -f -n default
```

```shell
tkn pipelinerun logs sum-and-multiply-pipeline-run-rgd9j -f -n default
[multiply-inputs : product] 150

[sum-inputs : sum] 25

[sum-and-multiply : sum] 30050
```

As you can see, you can define multiple tasks in the same pipeline and use the result of more than one task inside another task parameter. The substitution is only done inside `pipeline.spec.tasks[].params[]`. For a complete example demonstrating Task Results in a Pipeline, see the [pipelinerun example](../../examples/v1beta1/pipelineruns/task_results_example.yaml).
