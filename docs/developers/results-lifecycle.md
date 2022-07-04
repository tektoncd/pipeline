# Results Lifecycle

This section gives an overview of the results lifecycle in `Tekton Pipelines`.

### User input

A task author writes results from a step `/tekton/results/<result name>` as follows:

```yaml
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  generateName: print-date-
spec:
  taskSpec:
    description: |
      A simple task that prints the date.
    results:
      - name: current-date-unix-timestamp
        description: The current date in unix timestamp format
      - name: current-date-human-readable
        description: The current date in human readable format
    steps:
      - name: print-date-unix-timestamp
        image: bash:latest
        script: |
          #!/usr/bin/env bash
          date +%s | tee /tekton/results/current-date-unix-timestamp
      - name: print-date-human-readable
        image: bash:latest
        script: |
          #!/usr/bin/env bash
          date | tee /tekton/results/current-date-human-readable
```

Here, `/tekton/results` is an `emptydir volume` that is mounted by the task run reconciler on all the `step containers`. When the script is run inside the container, the results will be available accordingly.

Here are the important functions that come into play:

1. [Attaching volumes and creating the step containers.](https://github.com/tektoncd/pipeline/blob/59458291bdbe67300a989f190d8d51c3bbac1064/pkg/pod/pod.go#L111)

### Inside the step container
After the execution of the script in the step, the entrypointer `jsonifies` and then copies the results from `/tekton/results` to the termination path `/tekton/termination`. Just before the copy, the 4KB limit is checked and an error is thrown in the container logs if the size exceeds 4 KB. The kubernetes extracts the information from the termination path and provides it to the user as `pod.State.Terminated.Message`.

Here are the important functions that come into play:
1. [Entrypointer copies results to termination path and enforces 4K limit](https://github.com/tektoncd/pipeline/blob/59458291bdbe67300a989f190d8d51c3bbac1064/pkg/entrypoint/entrypointer.go#L104-L221)

### The taskrun reconciler
The taskrun reconciler executes a periodic loop performing multiple actions depending on the status of the taskrun. Once such action is `reconcile`. In this action, the reconciler extracts the termination message from the state of the pods (`State.Terminated.Message`). The message string is then parsed and the results are extracted and attached to the taskrun's `status` from where it can used by future tasks or accessed by the user.

Here are the important functions that come into play:
1. [Reconciler extracts the results from termination message and updates taskrun status](https://github.com/tektoncd/pipeline/blob/59458291bdbe67300a989f190d8d51c3bbac1064/pkg/pod/status.go#L146-L199)
