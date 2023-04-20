<!--
---
linkTitle: "Migrating from Tekton v1beta1"
weight: 4000
---
-->

# Migrating From Tekton `v1beta1` to Tekton `v1`

- [Changes to fields](#changes-to-fields)
- [Upgrading `PipelineRun.Timeout` to `PipelineRun.Timeouts`](#upgrading-pipelinerun.timeout-to-pipelinerun.timeouts)
- [Replacing Resources from Task, TaskRun, Pipeline and PipelineRun](#replacing-resources-from-task,-taskrun,-pipeline-and-pipelinerun)
- [Replacing `taskRef.bundle` and `pipelineRef.bundle` with Bundle Resolver](#replacing-taskRef.bundle-and-pipelineRef.bundle-with-bundle-resolver)
- [Adding `TaskRunTemplate` in `PipelineRun.Spec`](#adding-taskruntemplate-to-pipelinerun.spec)


This document describes the differences between `v1beta1` Tekton entities and their
`v1` counterparts. It also describes the changed fields and the deprecated fields into v1.
## Changes to fields

In Tekton `v1`, the following fields have been changed:

| Old field | Replacement |
| --------- | ----------|
| `pipelineRun.spec.Timeout`| `pipelineRun.spec.timeouts.pipeline` |
| `pipelineRun.spec.taskRunSpecs.taskServiceAccountName` | `pipelineRun.spec.taskRunSpecs.serviceAccountName` |
| `pipelineRun.spec.taskRunSpecs.taskPodTemplate` | `pipelineRun.spec.taskRunSpecs.podTemplate` |
| `taskRun.status.taskResults` | `taskRun.status.results` |
| `pipelineRun.status.pipelineResults` | `pipelineRun.status.results` |
| `taskRun.spec.taskRef.bundle` | `taskRun.spec.taskRef.resolver` |
| `pipelineRun.spec.pipelineRef.bundle` | `pipelineRun.spec.pipelineRef.resolver` |
| `task.spec.resources` | removed from `Task` |
| `taskrun.spec.resources` | removed from `TaskRun` |
| `taskRun.status.cloudEvents` | removed from `TaskRun` |
| `taskRun.status.resourcesResult` | removed from `TaskRun` |
| `pipeline.spec.resources` | removed from `Pipeline` |
| `pipelineRun.spec.resources` | removed from `PipelineRun` |
| `pipelineRun.spec.serviceAccountName` | [`pipelineRun.spec.taskRunTemplate.serviceAccountName`](#adding-taskruntemplate-to-pipelinerun.spec) |
| `pipelineRun.spec.podTemplate` | [`pipelineRun.spec.taskRunTemplate.podTemplate`](#adding-taskruntemplate-to-pipelinerun.spec) |
| `task.spec.steps[].resources` | `task.spec.steps[].computeResources` |
| `task.spec.stepTemplate.resources` | `task.spec.stepTemplate.computeResources` |
| `task.spec.sidecars[].resources` | `task.spec.sidecars[].computeResources` |
| `taskRun.spec.sidecarOverrides`| `taskRun.spec.sidecarSpecs` |
| `taskRun.spec.stepOverrides` | `taskRun.spec.stepSpecs` |
| `taskRun.spec.sidecarSpecs[].resources` | `taskRun.spec.sidecarSpecs[].computeResources` |
| `taskRun.spec.stepSpecs[].resources` | `taskRun.spec.stepSpecs[].computeResources` |

## Replacing `resources` from Task, TaskRun, Pipeline and PipelineRun
`PipelineResources` and the `resources` fields of Task, TaskRun, Pipeline and PipelineRun have been removed. Please use `Tasks` instead. For more information, see [Replacing PipelineResources](https://github.com/tektoncd/pipeline/blob/main/docs/pipelineresources.md)

## Replacing `taskRef.bundle` and `pipelineRef.bundle` with Bundle Resolver
Bundle resolver in remote resolution should be used instead of `taskRun.spec.taskRef.bundle` and `pipelineRun.spec.pipelineRef.bundle`.

The [`enable-bundles-resolver`](https://github.com/tektoncd/pipeline/blob/main/docs/install.md#customizing-the-pipelines-controller-behavior) feature flag must be enabled to use this feature.

```yaml
# Before in v1beta1:
apiVersion: tekton.dev/v1beta1
kind: TaskRun
spec:
  taskRef:
    name: example-task
    bundle: python:3-alpine
---
# After in v1:
apiVersion: tekton.dev/v1
kind: TaskRun
spec:
  taskRef:
    resolver: bundles
    params:
    - name: bundle
      value: python:3-alpine
    - name: name
      value: taskName
    - name: kind
      value: Task
```

## Replacing ClusterTask with Remote Resolution
`ClusterTask` is deprecated. Please use the `cluster` resolver instead.

The [`enable-cluster-resolver`](https://github.com/tektoncd/pipeline/blob/main/docs/install.md#customizing-the-pipelines-controller-behavior) feature flag must be enabled to use this feature.

The `cluster` resolver allows `Pipeline`s, `PipelineRun`s, and `TaskRun`s to refer
to `Pipeline`s and `Task`s defined in other namespaces in the cluster.

```yaml
# Before in v1beta1:
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: cluster-task-reference
spec:
  taskRef:
    name: example-task
    kind: ClusterTask
---
# After in v1:
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: cluster-task-reference
spec:
  taskRef:
    resolver: cluster
    params:
    - name: kind
      value: task
    - name: name
      value: example-task
    - name: namespace
      value: example-namespace
```

For more information, see [Remote resolution](https://github.com/tektoncd/community/blob/main/teps/0060-remote-resource-resolution.md).

## Adding TaskRunTemplate to PipelineRun.Spec
`ServiceAccountName` and `PodTemplate` are moved to `TaskRunTemplate` as `TaskRunTemplate.ServiceAccountName` and `TaskRunTemplate.PodTemplate` so that users can specify common configuration in `TaskRunTemplate` which will apply to all the TaskRuns.

```yaml
# Before in v1beta1:
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: template-pr
spec:
  pipelineRef:
    name: clone-test-build
  serviceAccountName: build
  podTemplate:
    securityContext:
      fsGroup: 65532
---
# After in v1:
apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: template-pr
spec:
  pipelineRef:
    name: clone-test-build
  taskRunTemplate:
    serviceAccountName: build
    podTemplate:
      securityContext:
        fsGroup: 65532
```

For more information, see [TEP-119](https://github.com/tektoncd/community/blob/main/teps/0119-add-taskrun-template-in-pipelinerun.md).
