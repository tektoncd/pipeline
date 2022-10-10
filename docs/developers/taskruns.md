This doc describes how TaskRuns are implemented using pods.

## Entrypoint rewriting and step ordering

Tekton releases include a binary called the "entrypoint", which wraps the
user-provided binary for each `Step` container to manage the execution order of the containers.
The `entrypoint` binary has the following arguments:

- `wait_file` - If specified, file to wait for
- `wait_file_content` - If specified, wait until the file has non-zero size
- `post_file` - If specified, file to write upon completion
- `entrypoint` - The command to run in the image being wrapped

As part of the PodSpec created by `TaskRun` the entrypoint for each `Task` step
is changed to the entrypoint binary with the mentioned arguments and a volume
with the binary and file(s) is mounted.

If the image is a private registry, the service account should include an
[ImagePullSecret](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#add-imagepullsecrets-to-a-service-account)

For more details, see [entrypoint/README.md](../../cmd/entrypoint/README.md)
or the talk ["Russian Doll: Extending Containers with Nested Processes"](https://www.youtube.com/watch?v=iz9_omZ0ctk).

## How to access the exit code of a step from any subsequent step in a task

The entrypoint now allows exiting with an error and continue running rest of the
steps in a task i.e., it is possible for a step to exit with a non-zero exit
code. Now, it is possible to design a task with a step which can take an action
depending on the exit code of any prior steps. The user can access the exit code
of a step by reading the file pointed by the path variable
`$(steps.step-<step-name>.exitCode.path)` or
`$(steps.step-unnamed-<step-index>.exitCode.path)`. For example:

- `$(steps.step-my-awesome-step.exitCode.path)` where the step name is
  `my-awesome-step`.
- `$(steps.step-unnamed-0.exitCode.path)` where the first step in a task has no
  name.

The exit code of a step is stored in a file named `exitCode` under a directory
`/tekton/steps/step-<step-name>/` or `/tekton/steps/step-unnamed-<step-index>/`
which is reserved for any other step specific information in the future.

If you would like to use the tekton internal path, you can access the exit code
by reading the file (which is not recommended though since the path might change
in the future):

```shell
cat /tekton/steps/step-<step-name>/exitCode
```

And, access the step exit code without a step name:

```shell
cat /tekton/steps/step-unnamed-<step-index>/exitCode
```

Or, you can access the step metadata directory via symlink, for example, use
`cat /tekton/steps/0/exitCode` for the first step in a task.

## TaskRun Use of Pod Termination Messages

Tekton Pipelines uses a `Pod's`
[termination message](https://kubernetes.io/docs/tasks/debug-application-cluster/determine-reason-pod-failure/)
to pass data from a Step's container to the Pipelines controller. Examples of
this data include: the time that execution of the user's step began, contents of
task results, contents of pipeline resource results.

The contents and format of the termination message can change. At time of
writing the message takes the form of a serialized JSON blob. Some of the data
from the message is internal to Tekton Pipelines, used for book-keeping, and
some is distributed across a number of fields of the `TaskRun's` `status`. For
example, a `TaskRun's` `status.taskResults` is populated from the termination
message.

## Reserved directories

### /workspace

- `/workspace` - This directory is where volumes for [resources](#resources) and
  [workspaces](#workspaces) are mounted.

### /tekton

The `/tekton/` directory is reserved on containers for internal usage.

Here is an example of a directory layout for a simple Task with 2 script steps:

```
/tekton
|-- bin
    `-- entrypoint
|-- creds
|-- downward
|   |-- ..2021_09_16_18_31_06.270542700
|   |   `-- ready
|   |-- ..data -> ..2021_09_16_18_31_06.270542700
|   `-- ready -> ..data/ready
|-- home
|-- results
|-- run
    `-- 0
        `-- out
        `-- status
            `-- exitCode
|-- scripts
|   |-- script-0-t4jd8
|   `-- script-1-4pjwp
|-- steps
|   |-- 0 -> /tekton/run/0/status
|   |-- 1 -> /tekton/run/1/status
|   |-- step-foo -> /tekton/run/1/status
|   `-- step-unnamed-0 -> /tekton/run/0/status
`-- termination
```

| Path                | Description                                                                                                                                                                                                                                                                                              |
| ------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| /tekton             | Directory used for Tekton specific functionality                                                                                                                                                                                                                                                         |
| /tekton/bin         | Tekton provided binaries / tools                                                                                                                                                                                                                                                                         |
| /tekton/creds       | Location of Tekton mounted secrets. See [Authentication at Run Time](../auth.md) for more details.                                                                                                                                                                                                       |
| /tekton/debug       | Contains [Debug scripts](https://github.com/tektoncd/pipeline/blob/main/docs/debug.md#debug-scripts) used to manage step lifecycle during debugging at a breakpoint and the [Debug Info](https://github.com/tektoncd/pipeline/blob/main/docs/debug.md#mounts) mount used to assist for the same.         |                                                                                                                                               |  
| /tekton/downward    | Location of data mounted via the [Downward API](https://kubernetes.io/docs/tasks/inject-data-application/downward-api-volume-expose-pod-information/#the-downward-api).                                                                                                                                  |
| /tekton/home        | (deprecated - see https://github.com/tektoncd/pipeline/issues/2013) Default home directory for user containers.                                                                                                                                                                                          |
| /tekton/results     | Where [results](#results) are written to (path available to `Task` authors via [`$(results.name.path)`](../variables.md))                                                                                                                                                                                |
| /tekton/run         | Runtime variable data. [Used for coordinating step ordering](#entrypoint-rewriting-and-step-ordering).                                                                                                                                                                                                   |
| /tekton/scripts     | Contains user provided scripts specified in the TaskSpec.                                                                                                                                                                                                                                                |
| /tekton/steps       | Where the `step` exitCodes are written to (path available to `Task` authors via [`$(steps.<stepName>.exitCode.path)`](../variables.md#variables-available-in-a-task))                                                                                                                                    |
| /tekton/termination | where the eventual [termination log message](https://kubernetes.io/docs/tasks/debug-application-cluster/determine-reason-pod-failure/#writing-and-reading-a-termination-message) is written to [Sequencing step containers](#entrypoint-rewriting-and-step-ordering)                                     |

The following directories are covered by the
[Tekton API Compatibility policy](../../api_compatibility_policy.md), and can be
relied on for stability:

- `/tekton/results`

All other files/directories are internal implementation details of Tekton -
**users should not rely on specific paths or behaviors as it may change in the
future**.

## What and Why of `/tekton/run`

`/tekton/run` is a collection of implicit volumes mounted on a pod and created
for storing the step specific information/metadata. Steps can only write
metadata to their own `tekton/run` directory - all other step volumes are mounted as
`readonly`. The `tekton/run` directories are considered internal implementation details
of Tekton and are not bound by the API compatibility policy - the contents and
structure can be safely changed so long as user behavior remains the same.

### `/tekton/steps`

`/tekton/steps` are special subdirectories are created for each step in a task -
each directory is actually a symlink to a directory in the Step's corresponding
`/tekton/run` volume. This is done to ensure that step directories can only be
modified by their own Step. To ensure that these symlinks are not modified, the
entire `/tekton/steps` volume is initially populated by an initContainer, and
mounted `readonly` on all user steps.

These symlinks are created as a part of the `step-init` entrypoint subcommand
initContainer on each Task Pod.

### Entrypoint configuration

The entrypoint is modified to include an additional flag representing the step
specific directory where step metadata should be written:

```
step_metadata_dir - the dir specified in this flag is created to hold a step specific metadata
```

`step_metadata_dir` is set to `/tekton/run/<step #>/status` for the entrypoint
of each step.

### Example

Let's take an example of a task with two steps, each exiting with non-zero exit
code:

```yaml
kind: TaskRun
apiVersion: tekton.dev/v1beta1
metadata:
  generateName: test-taskrun-
spec:
  taskSpec:
    steps:
      - image: alpine
        name: step0
        onError: continue
        script: |
          exit 1
      - image: alpine
        onError: continue
        script: |
          exit 2
```

During `step-step0`, the first container is actively running so none of the
output files are populated yet. The `/tekton/steps` directories are symlinked to
locations that do not yet exist, but will be populated during execution.

```
/tekton
|-- run
|   |-- 0
|   `-- 1
|-- steps
    |-- 0 -> /tekton/run/0/status
    |-- 1 -> /tekton/run/1/status
    |-- step-step0 -> /tekton/run/0/status
    `-- step-unnamed1 -> /tekton/run/1/status
```

During `step-unnamed1`, the first container has now finished. The output files
for the first step are now populated, and the folder pointed to by
`/tekton/steps/0` now exists, and is populated with a file named `exitCode`
which contains the exit code of the first step.

```
/tekton
|-- run
|   |-- 0
|   |   |-- out
|   |   `-- status
|   |       `-- exitCode
|   `-- 1
|-- steps
    |-- 0 -> /tekton/run/0/status
    |-- 1 -> /tekton/run/1/status
    |-- step-step0 -> /tekton/run/0/status
    `-- step-unnamed1 -> /tekton/run/1/status
```

Notice that there are multiple symlinks showing under `/tekton/steps/` pointing
to the same `/tekton/run` location. These symbolic links are created to provide
simplified access to the step metadata directories i.e., instead of referring to
a directory with the step name, access it via the step index. The step index
becomes complex and hard to keep track of in a task with a long list of steps,
for example, a task with 20 steps. Creating the step metadata directory using a
step name and creating a symbolic link using the step index gives the user
flexibility, and an option to choose whatever works best for them.

## Handling of injected sidecars

Tekton has to take some special steps to support sidecars that are injected into
TaskRun Pods. Without intervention sidecars will typically run for the entire
lifetime of a Pod but in Tekton's case it's desirable for the sidecars to run
only as long as Steps take to complete. There's also a need for Tekton to
schedule the sidecars to start before a Task's Steps begin, just in case the
Steps rely on a sidecars behavior, for example to join an Istio service mesh. To
handle all of this, Tekton Pipelines implements the following lifecycle for
sidecar containers:

First, the
[Downward API](https://kubernetes.io/docs/tasks/inject-data-application/downward-api-volume-expose-pod-information/#the-downward-api)
is used to project an annotation on the TaskRun's Pod into the `entrypoint`
container as a file. The annotation starts as an empty string, so the file
projected by the downward API has zero length. The entrypointer spins, waiting
for that file to have non-zero size.

The sidecar containers start up. Once they're all in a ready state, the
annotation is populated with string "READY", which in turn populates the
Downward API projected file. The entrypoint binary recognizes that the projected
file has a non-zero size and allows the Task's steps to begin.

On completion of all steps in a Task the TaskRun reconciler stops any sidecar
containers. The `Image` field of any sidecar containers is swapped to the nop
image. Kubernetes observes the change and relaunches the container with updated
container image. The nop container image exits immediately _because it does not
provide the command that the sidecar is configured to run_. The container is
considered `Terminated` by Kubernetes and the TaskRun's Pod stops.

There are known issues with the existing implementation of sidecars:

- When the `nop` image does provide the sidecar's command, the sidecar will
  continue to run even after `nop` has been swapped into the sidecar container's
  image field. See
  [the issue tracking this bug](https://github.com/tektoncd/pipeline/issues/1347)
  for the issue tracking this bug. Until this issue is resolved the best way to
  avoid it is to avoid overriding the `nop` image when deploying the tekton
  controller, or ensuring that the overridden `nop` image contains as few
  commands as possible.

- `kubectl get pods` will show a Completed pod when a sidecar exits successfully
  but an Error when the sidecar exits with an error. This is only apparent when
  using `kubectl` to get the pods of a TaskRun, not when describing the Pod
  using `kubectl describe pod ...` nor when looking at the TaskRun, but can be
  quite confusing.