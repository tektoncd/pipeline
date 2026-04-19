<!--
---
linkTitle: "Pipelines in Pipelines"
weight: 406
---
-->

# Pipelines in Pipelines

- [Overview](#overview)
- [Specifying `pipelineRef` in `Tasks`](#specifying-pipelineref-in-pipelinetasks)
- [Specifying `pipelineSpec` in `Tasks`](#specifying-pipelinespec-in-pipelinetasks)
- [Specifying `Parameters`](#specifying-parameters)
- [Specifying `Workspaces`](#specifying-workspaces)
- [Known Limitations](#known-limitations)

## Overview

A mechanism to define and execute Pipelines in Pipelines, alongside Tasks and Custom Tasks, for a more in-depth background and inspiration, refer to the proposal [TEP-0056](https://github.com/tektoncd/community/blob/main/teps/0056-pipelines-in-pipelines.md "Proposal").

> :seedling: **Pipelines in Pipelines is an [alpha](additional-configs.md#alpha-features) feature.**
> The `enable-api-fields` feature flag must be set to `"alpha"` to specify `pipelineRef` or `pipelineSpec` in a `pipelineTask`.
> When enabled, a `PipelineTask` that references a child `Pipeline` will produce a child `PipelineRun` owned by the parent `PipelineRun`.
> Recursive `pipelineRef` references are detected by walking the parent `PipelineRun` owner chain and fail fast with a clear error.
> See [Known Limitations](#known-limitations) for the current caveats.

## Specifying `pipelineRef` in `pipelineTasks`

Defining Pipelines in Pipelines at authoring time, by either specifying `PipelineRef` or `PipelineSpec` fields to a `PipelineTask` alongside `TaskRef` and `TaskSpec`.

For example, a Pipeline named security-scans which is run within a Pipeline named clone-scan-notify where the PipelineRef is used:

```
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: security-scans
spec:
  tasks:
    - name: scorecards
      taskRef:
        name: scorecards
    - name: codeql
      taskRef:
        name: codeql
---
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: clone-scan-notify
spec:
  tasks:
    - name: git-clone
      taskRef:
        name: git-clone
    - name: security-scans
      pipelineRef:
        name: security-scans
    - name: notification
      taskRef:
        name: notification
```

## Specifying `pipelineSpec` in `pipelineTasks`

The `pipelineRef` [example](#specifying-pipelineref-in-pipelinetasks) can be modified to use PipelineSpec instead of PipelineRef to instead embed the Pipeline specification:
```
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: clone-scan-notify
spec:
  tasks:
    - name: git-clone
      taskRef:
        name: git-clone
    - name: security-scans
      pipelineSpec:
        tasks:
          - name: scorecards
            taskRef:
              name: scorecards
          - name: codeql
            taskRef:
              name: codeql
    - name: notification
      taskRef:
        name: notification
```

## Specifying `Parameters`

Pipelines in Pipelines consume Parameters in the same way as Tasks in Pipelines
```
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: clone-scan-notify
spec:
  params:
    - name: repo
      value: $(params.repo)
  tasks:
    - name: git-clone
      params:
        - name: repo
          value: $(params.repo)      
      taskRef:
        name: git-clone
    - name: security-scans
      params:
        - name: repo
          value: $(params.repo)
      pipelineRef:
        name: security-scans
    - name: notification
      taskRef:
        name: notification
```

## Specifying `Workspaces`

A `PipelineTask` that references a child `Pipeline` can map parent workspaces to the child's declared workspaces the same way it does for `Tasks`. The bindings declared on the `PipelineTask` are propagated to the child `PipelineRun`, which then forwards them to the child `Pipeline`'s workspaces.

```yaml
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: security-scans
spec:
  workspaces:
    - name: source
  tasks:
    - name: scorecards
      workspaces:
        - name: source
      taskRef:
        name: scorecards
---
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: clone-scan-notify
spec:
  workspaces:
    - name: shared-ws
  tasks:
    - name: git-clone
      workspaces:
        - name: output
          workspace: shared-ws
      taskRef:
        name: git-clone
    - name: security-scans
      runAfter:
        - git-clone
      workspaces:
        - name: source
          workspace: shared-ws
      pipelineRef:
        name: security-scans
```

## Known Limitations

The initial alpha implementation has the following limitations. These are expected to be addressed in follow-up work.

### Results from child Pipelines are not propagated

`Results` produced by a child `Pipeline` are **not** surfaced on the parent `PipelineRun`:

- They are not aggregated into the parent `PipelineRun`'s `status.results`.
- They cannot be consumed by other `PipelineTasks` via `$(tasks.<task-name>.results.<result-name>)` or by `when` expressions in the parent.

To enforce this at authoring time, the pipeline validating webhook rejects any result reference whose target is a `PipelineTask` using `pipelineRef` or `pipelineSpec`, with an error like:

> `result reference to pipelineTask "child" is not supported: referenced task uses pipelineRef or pipelineSpec and result propagation from child Pipelines is not yet implemented`

Until propagation is implemented, pass values into a child `Pipeline` through `params` on the `PipelineTask`, and keep the parent `Pipeline`'s `results` sourced from regular `TaskRun`s.
