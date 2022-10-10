# Results Lifecycle

This section gives an overview of the results lifecycle in `Tekton Pipelines`.

## Results definition and usage

Tasks can define results by adding a result on the Task spec.
For more information, see ["Emitting Results"](../tasks.md#emitting-results).
Users are encouraged to use variable substitution for Result paths
(i.e. writing to `$(results.name.path)`), but can also write to the `/tekton/results`
directory directly, for example:

```yaml
apiVersion: tekton.dev/v1beta1
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

The result is added to a file name with the specified result's name into the
`/tekton/results` folder. This is then added to the task run status. Internally
the results are a new argument `-results`to the entrypoint defined for the task.

Each `results` entry in the `Task's` YAML corresponds to a
file that the `Task` should stores the result in. 
When a Task specifies `results`, the TaskRun reconciler automatically creates the
`/tekton/results` directory using an emptyDir volume and mounts it on all the `step containers`. When the script is run inside the container, the results will be available accordingly.

Users can also refer to Task results in Pipeline Tasks.
See ["Passing one Task's Results into the Parameters or when expressions of another"](../pipelines.md#passing-one-tasks-results-into-the-parameters-or-when-expressions-of-another)
for more information.

### Known issues

- Task Results are returned to the TaskRun controller via the container's
  termination message. At time of writing this has a capped maximum size of
  ["4096 bytes or 80 lines, whichever is smaller"](https://kubernetes.io/docs/tasks/debug-application-cluster/determine-reason-pod-failure/#customizing-the-termination-message).
  See ["Emitting Results"](../tasks.md#emitting-results) for more information.

Here are the important functions that come into play in the TaskRun reconciler:

1. [Attaching volumes and creating the step containers.](https://github.com/tektoncd/pipeline/blob/59458291bdbe67300a989f190d8d51c3bbac1064/pkg/pod/pod.go#L111)

### Inside the step container
After the execution of the script in the step, the entrypointer `jsonifies` and then copies the results from `/tekton/results` to the termination path `/tekton/termination`. Just before the copy, the 4KB limit is checked and an error is thrown in the container logs if the size exceeds 4 KB. The kubernetes extracts the information from the termination path and provides it to the user as `pod.State.Terminated.Message`.

Here are the important functions that come into play:
1. [Entrypointer copies results to termination path and enforces 4K limit](https://github.com/tektoncd/pipeline/blob/59458291bdbe67300a989f190d8d51c3bbac1064/pkg/entrypoint/entrypointer.go#L104-L221)

### The taskrun reconciler
The taskrun reconciler executes a periodic loop performing multiple actions depending on the status of the taskrun. Once such action is `reconcile`. In this action, the reconciler extracts the termination message from the state of the pods (`State.Terminated.Message`). The message string is then parsed and the results are extracted and attached to the taskrun's `status` from where it can used by future tasks or accessed by the user.

Here are the important functions that come into play:
1. [Reconciler extracts the results from termination message and updates taskrun status](https://github.com/tektoncd/pipeline/blob/59458291bdbe67300a989f190d8d51c3bbac1064/pkg/pod/status.go#L146-L199)
