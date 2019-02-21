# Labels

In order to make it easier to identify objects that are all part of the same
conceptual pipeline, custom
[labels](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/)
set on resources used by Tekton Pipelines are propagated from more general to
more specific resources, and a few labels are automatically added to make it
easier to identify relationships between those resources.

---

- [Propagation Details](#propagation-details)
- [Automatically Added Labels](#automatically-added-labels)
- [Examples](#examples)

---

## Propagation Details

For `Pipelines` executed using a `PipelineRun`, labels are propagated
automatically from `Pipelines` to `PipelineRuns` to `TaskRuns` and then to
`Pods`. Additionally, labels from the `Tasks` referenced by `TaskRuns` are
propagated to the corresponding `TaskRuns` and then to `Pods`.

For `TaskRuns` executed directly, not as part of a `Pipeline`, labels are
propagated from the referenced `Task` (if one exists, see the
[Specifying a `Task`](taskruns.md#specifying-a-task) section of the `TaskRun`
documentation) to the corresponding `TaskRun` and then to the `Pod`.

## Automatically Added Labels

The following labels are added to resources automatically:

- `tekton.dev/pipeline` is added to `PipelineRuns` (and propagated to `TaskRuns`
  and `Pods`), and contains the name of the `Pipeline` that the `PipelineRun`
  references.
- `tekton.dev/pipelineRun` is added to `TaskRuns` (and propagated to `TaskRuns`
  and `Pods`) that are created automatically during the execution of a
  `PipelineRun`, and contains the name of the `PipelineRun` that triggered the
  creation of the `TaskRun`.
- `tekton.dev/task` is added to `TaskRuns` (and propagated to `Pods`) that
  reference an existing `Task` (see the
  [Specifying a `Task`](taskruns.md#specifying-a-task) section of the `TaskRun`
  documentation), and contains the name of the `Task` that the `TaskRun`
  references.
- `tekton.dev/taskRun` is added to `Pods`, and contains the name of the
  `TaskRun` that created the `Pod`.

## Examples

- [Finding Pods for a Specific PipelineRun](#finding-pods-for-a-specific-pipelinerun)
- [Finding TaskRuns for a Specific Task](#finding-taskruns-for-a-specific-task)

### Finding Pods for a Specific PipelineRun

To find all `Pods` created by a `PipelineRun` named test-pipelinerun, you could
use the following command:

```shell
kubectl get pods --all-namespaces -l tekton.dev/pipelineRun=test-pipelinerun
```

### Finding TaskRuns for a Specific Task

To find all `TaskRuns` that reference a `Task` named test-task, you could use
the following command:

```shell
kubectl get taskruns --all-namespaces -l tekton.dev/task=test-task
```
