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

## Overview

A mechanism to define and execute Pipelines in Pipelines, alongside Tasks and Custom Tasks, for a more in-depth background and inspiration, refer to the proposal [TEP-0056](https://github.com/tektoncd/community/blob/main/teps/0056-pipelines-in-pipelines.md "Proposal").

> :seedling: **Pipelines in Pipelines is an  [alpha](additional-configs.md#alpha-features) feature.**
> The `enable-api-fields` feature flag must be set to `"alpha"` to specify `pipelineRef` or `pipelineSpec` in a `pipelineTask`.
> This feature is in Preview Only mode and not yet supported/implemented.

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
