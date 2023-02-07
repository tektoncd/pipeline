<!--
---
linkTitle: "Labels and Annotations"
weight: 305
---
-->

# Labels and Annotations

Tekton allows you to use custom [Kubernetes Labels](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/)
to easily mark Tekton entities belonging to the same conceptual execution chain. Tekton also automatically adds select labels
to more easily identify resource relationships. This document describes the label propagation scheme, automatic labeling, and
provides usage examples.

---

- [Label propagation](#label-propagation)
- [Automatic labeling](#automatic-labeling)
- [Usage examples](#usage-examples)

---

## Label propagation

Labels propagate among Tekton entities as follows:

- For `Pipelines` instantiated using a `PipelineRun`, labels propagate
automatically from `Pipelines` to `PipelineRuns` to `TaskRuns`, and then to
the associated `Pods`. If a label is present in both `Pipeline` and
`PipelineRun`, the label in `PipelineRun` takes precedence.

- Labels from `Tasks` referenced by `TaskRuns` within a `PipelineRun` propagate to the corresponding `TaskRuns`,
and then to the associated `Pods`. As for `Pipeline` and `PipelineRun`, if a label is present in both `Task` and
`TaskRun`, the label in `TaskRun` takes precedence.

- For standalone `TaskRuns` (that is, ones not executing as part of a `Pipeline`), labels
propagate from the [referenced `Task`](taskruns.md#specifying-the-target-task), if one exists, to
the corresponding `TaskRun`, and then to the associated `Pod`. The same as above applies.

## Automatic labeling

Tekton automatically adds labels to Tekton entities as described in the following table.

**Note:** `*.tekton.dev` labels are reserved for Tekton's internal use only. Do not add or remove them manually.

<table >
	<tbody>
		<tr>
			<td><b>Label</b></td>
			<td><b>Added To</b></td>
			<td><b>Propagates To</b></td>
			<td><b>Contains</b></td>
		</tr>
		<tr>
			<td><code>tekton.dev/pipeline</code></td>
			<td><code>PipelineRuns</code></td>
			<td><code>TaskRuns, Pods</code></td>
			<td>Name of the <code>Pipeline</code> that the <code>PipelineRun</code> references.</td>
		</tr>
		<tr>
			<td><code>tekton.dev/pipelineRun</code></td>
			<td><code>TaskRuns</code> that are created automatically during the execution of a <code>PipelineRun</code>.</td>
			<td><code>TaskRuns, Pods</code></td>
			<td>Name of the <code>PipelineRun</code> that triggered the creation of the <code>TaskRun</code>.</td>
		</tr>
		<tr>
			<td><code>tekton.dev/task</code></td>
			<td><code>TaskRuns</code> that <a href="taskruns.md#specifying-the-target-task">reference an existing </code>Task</code></a>.</td>
			<td><code>Pods</code></td>
			<td>Name of the <code>Task</code> that the <code>TaskRun</code> references.</td>
		</tr>
		<tr>
			<td><code>tekton.dev/clusterTask</code></td>
			<td><code>TaskRuns</code> that reference an existing <code>ClusterTask</code>.</td>
			<td><code>Pods</code></td>
			<td>Name of the <code>ClusterTask</code> that the <code>TaskRun</code> references.</td>
		</tr>
		<tr>
			<td><code>tekton.dev/taskRun</code></td>
			<td><code>Pods</code></td>
			<td>No propagation.</td>
			<td>Name of the <code>TaskRun</code> that created the <code>Pod</code>.</td>
		</tr>
		<tr>
			<td><code>tekton.dev/memberOf</code></td>
			<td><code>TaskRuns</code> that are created automatically during the execution of a <code>PipelineRun</code>.</td>
			<td><code>TaskRuns, Pods</code></td>
			<td><code>tasks</code> or <code>finally</code> depending on the <code>PipelineTask</code>'s membership in the <code>Pipeline</code>.</td>
		</tr>
		<tr>
			<td><code>app.kubernetes.io/instance</code>, <code>app.kubernetes.io/component</code></td>
			<td><code>Pods</code>, <code>StatefulSets</code> (Affinity Assistant)</td>
			<td>No propagation.</td>
			<td><code>Pod</code> affinity values for <code>TaskRuns</code>.</td>
		</tr>
	</tbody>
</table>

## Usage examples

Below are some examples of using labels:

The following command finds all `Pods` created by a `PipelineRun` named `test-pipelinerun`:

```shell
kubectl get pods --all-namespaces -l tekton.dev/pipelineRun=test-pipelinerun
```

The following command finds all `TaskRuns` that reference a `Task` named `test-task`:

```shell
kubectl get taskruns --all-namespaces -l tekton.dev/task=test-task
```

The following command finds all `TaskRuns` that reference a `ClusterTask` named `test-clustertask`:

```shell
kubectl get taskruns --all-namespaces -l tekton.dev/clusterTask=test-clustertask
```

## Annotations propagation

Annotation propagate among Tekton entities as follows (similar to Labels):

- For `Pipelines` instantiated using a `PipelineRun`, annotations propagate
automatically from `Pipelines` to `PipelineRuns` to `TaskRuns`, and then to
the associated `Pods`. If a annotation is present in both `Pipeline` and
`PipelineRun`, the annotation in `PipelineRun` takes precedence.

- Annotations from `Tasks` referenced by `TaskRuns` within a `PipelineRun` propagate to the corresponding `TaskRuns`,
and then to the associated `Pods`. As for `Pipeline` and `PipelineRun`, if a annotation is present in both `Task` and
`TaskRun`, the annotation in `TaskRun` takes precedence.

- For standalone `TaskRuns` (that is, ones not executing as part of a `Pipeline`), annotations
propagate from the [referenced `Task`](taskruns.md#specifying-the-target-task), if one exists, to
the corresponding `TaskRun`, and then to the associated `Pod`. The same as above applies.
