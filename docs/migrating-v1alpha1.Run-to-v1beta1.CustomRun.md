<!--
---
linkTitle: "Migrating from v1alpha1.Run to v1beta1.CustomRun"
weight: 4000
---
-->

# Migrating from v1alpha1.Run to v1beta1.CustomRun

This document describes how to migrate from `v1alpha1.Run` and `v1beta1.CustomRun`
- [Changes to fields](#changes-to-fields)
  - [Changes to the specification](#changes-to-the-specification)
  - [Changes to the reference](#changes-to-the-reference)
  - [Support `PodTemplate` in Custom Task Spec](#support-podtemplate-in-custom-task-spec)
- [Changes to implementation requirements](#changes-to-implementation-instructions)
  - [Status Reporting](#status-reporting)
  - [Cancellation](#cancellation)
  - [Timeout](#timeout)
  - [Retries & RetriesStatus](#4-retries--retriesstatus)
- [New feature flag `custom-task-version` for migration](#new-feature-flag-custom-task-version)

## Changes to fields

Comparing `v1alpha1.Run` with `v1beta1.CustomRun`, the following fields have been changed:

| Old field              | New field         |
| ---------------------- | ------------------|
| `spec.spec`            | [`spec.customSpec`](#changes-to-the-specification) |
| `spec.ref`             | [`spec.customRef`](#changes-to-the-reference) |
| `spec.podTemplate`     | removed, see [this section](#support-podtemplate-in-custom-task-spec) if you still want to support it|


### Changes to the specification

In `v1beta1.CustomRun`, the specification is renamed from `spec.spec` to `spec.customSpec`.

```yaml
# before (v1alpha1.Run)
apiVersion: tekton.dev/v1alpha1
kind: Run
metadata:
  name: run-with-spec
spec:
  spec:
    apiVersion: example.dev/v1alpha1
    kind: Example
    spec:
      field1: value1
      field2: value2
# after (v1beta1.CustomRun)
apiVersion: tekton.dev/v1beta1
kind: CustomRun
metadata:
  name: customrun-with-spec
spec:
  customSpec:
    apiVersion: example.dev/v1beta1
    kind: Example
    spec:
      field1: value1
      field2: value2
```

### Changes to the reference

In `v1beta1.CustomRun`, the specification is renamed from `spec.ref` to `spec.customRef`.

```yaml
# before (v1alpha1.Run)
apiVersion: tekton.dev/v1alpha1
kind: Run
metadata:
  name: run-with-reference
spec:
  ref:
    apiVersion: example.dev/v1alpha1
    kind: Example
    name: my-example
---
# after (v1beta1.CustomRun)
apiVersion: tekton.dev/v1beta1
kind: CustomRun
metadata:
  name: customrun-with-reference
spec:
  customRef:
    apiVersion: example.dev/v1beta1
    kind: Example
    name: my-customtask
```

### Support `PodTemplate` in Custom Task Spec

`spec.podTemplate` is removed in `v1beta1.CustomRun`. You can support that field in your own custom task spec if you want to. For example:

```yaml
apiVersion: tekton.dev/v1beta1
kind: CustomRun
metadata:
  name: customrun-with-podtemplate
spec:
  customSpec:
    apiVersion: example.dev/v1beta1
    kind: Example
    spec:
      podTemplate: 
        securityContext:
        runAsUser: 1001
```

## Changes to implementation instructions
We've changed four implementation instructions. Note that `status reporting` is the only required instruction to follow, others are recommendations.

### Status Reporting

You **MUST** report `v1beta1.CustomRun` as `Done` (set its `Conditions.Succeeded` status as `True` or `False`) **ONLY** when you want Pipeline Controller consider it as finished.

For example, if the `CustomRun` failed, while it still has remaining `retries`. If you want pipeline controller to continue watching its status changing, you **MUST NOT** mark it as `Done`. Otherwise, the `PipelineRun` controller may consider it as finished.

Here are statuses that indicating the `CustomRun` is done.
```yaml
Type: Succeeded
Status: False
Reason: TimedOut
```
```yaml
Type: Succeeded
Status: True
Reason: Succeeded
```

### Cancellation

Custom Task implementors are responsible for implementing `cancellation` to **support pipelineRun level timeouts and cancellation**. If a Custom Task implementor does not support cancellation via `customRun.spec.status`, `PipelineRun` can not timeout within the specified interval/duration and can not be cancelled as expected upon request.

It is recommended to update the `CustomRun` status  as following, once noticing `customRun.Spec.Status` is updated to `RunCancelled`

```yaml
Type: Succeeded
Status: False
Reason: CustomRunCancelled
```

### Timeout

We recommend `customRun.Timeout` to be set for each retry attempt instead of all retries. That's said, if one `CustomRun` execution fails on `Timeout` and it has remaining retries, the `CustomRun` controller SHOULD NOT set the status of that `CustomRun` as `False`. Instead, it SHOULD initiate another round of execution.

### 4. Retries & RetriesStatus

We recommend you use `customRun.Spec.Retries` if you want to implement the `retry` logic for your `Custom Task`, and archive history `customRun.Status` in `customRun.Status.RetriesStatus`.

Say you started a `CustomRun` by setting the following condition:
```yaml
Status:
  Conditions:
  - Type: Succeeded
    Status: Unknown
    Reason: Running
```
Now it failed for some reasons but has remaining retries, instead of setting Conditions as failed on TimedOut:
```yaml
Status:
  Conditions:
  - Type: Succeeded
    Status: False
    Reason: Failed
```
We **recommend** you to do archive the failure status to `customRun.Status.retriesStatus`, and keep setting `customRun.Status` as `Unknown`:
```yaml
Status:
  Conditions:
  - Type: Succeeded
    Status: Unknown
    Reason: "xxx"
  RetriesStatus:
  - Conditions:
    - Type: Succeeded
      Status: False
      Reason: Failed
```


## New feature flag `custom-task-version` for migration

You can use `custom-task-version` to control whether `v1alpha1.Run` or `v1beta1.CustomRun` should be created when a `Custom Task` is specified in a `Pipeline`. The feature flag currently supports two values: `v1alpha1` and `v1beta1`.

We'll change its default value per the following timeline:
- v0.43.*: default to `v1alpha1`.
- v0.44.*: switch the default to `v1beta1`
- v0.47.*: remove the feature flag, stop supporting creating `v1alpha1.Run`


[cancel-pr]: https://github.com/tektoncd/pipeline/blob/main/docs/pipelineruns.md#cancelling-a-pipelinerun
[gracefully-cancel-pr]: (https://github.com/tektoncd/pipeline/blob/main/docs/pipelineruns.md#gracefully-cancelling-a-pipelinerun)