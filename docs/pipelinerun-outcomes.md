# Author-defined PipelineRun outcomes

This is an experimental proof of concept for
[TEP-0171](https://github.com/tektoncd/community/pull/1313).
The API is under review and may change.

The Pipeline author can select a declared object-valued Pipeline result with
`spec.outcome.result`. After all tasks, including `finally` tasks, complete
successfully, the controller validates that result and copies its `reason` and
optional `message` into `PipelineRun.status.outcome`.

Enable `enable-pipelinerun-outcomes: "true"` in the `feature-flags` ConfigMap for
both the webhook and controller. It is an alpha feature, disabled by default.
The existing requirements for object results still apply. A Pipeline without an
outcome selector keeps its existing behavior.

## Try the example

Install a controller and webhook built from this branch, enable the flag, and
create the [example PipelineRun](../examples/v1/pipelineruns/pipelinerun-outcome-neutral.yaml).
Both tasks exit successfully, and the reporting `finally` task emits a neutral
report. The expected terminal status includes:

```yaml
conditions:
  - type: Succeeded
    status: "True"
    reason: Succeeded
outcome:
  reason: Neutral
  message: Review the finding before releasing.
```

Change the report's `reason` to `Succeeded` to report success instead.
The selected result may contain only `reason` and `message`. The reason must be
`Succeeded` or `Neutral`; the optional message is limited to 4,096 UTF-8 bytes.
A missing or invalid report makes an otherwise successful run fail with reason
`InvalidPipelineOutcome`. Existing result-processing errors retain their reason.
Execution failures, cancellation, and timeouts take precedence over the report.

The report and terminal execution condition are published in the same status
update. The existing `Succeeded` condition and its reason, message, and timestamps
are preserved for valid reports, including successful runs with skipped tasks.

## Consumers and limits

Consumers read execution status first and then the outcome. PaC, CLI, and
dashboard integrations must adopt the new field explicitly; this branch does not
change their presentation or provider conclusions. Existing consumers continue
to show execution success for a neutral run.

This PoC does not add author-selected failure outcomes, TaskRun outcomes, or
pause/resume. It does not change `onError`, retries, or task scheduling. The
controller uses the existing result collection path, so reports from abandoned
retry attempts are not selected independently.

Before downgrading to a controller without this API, remove outcome selectors
and finish opted-in runs.
