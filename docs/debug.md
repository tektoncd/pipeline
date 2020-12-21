<!--
---
linkTitle: "Debug"
weight: 11
---
-->
# Debug

- [Overview](#overview)
- [Debugging TaskRuns](#debugging-taskruns)
  - [Adding Breakpoints](#adding-breakpoints)
    - [Breakpoint on Failure](#breakpoint-on-failure)
      - [Failure of a Step](#failure-of-a-step)
      - [Halting a Step on failure](#halting-a-step-on-failure)
      - [Exiting breakpoint](#exiting-breakpoint)
- [Debug Environment](#debug-environment)
  - [Mounts](#mounts)
  - [Debug Scripts](#debug-scripts)


## Overview

`Debug` spec is used for troubleshooting and breakpointing runtime resources. This doc helps understand the inner 
workings of debug in Tekton. Currently only the `TaskRun` resource is supported. 

## Debugging TaskRuns

The following provides explanation on how Debugging TaskRuns is possible through Tekton. To understand how to use 
the debug spec for TaskRuns follow the [TaskRun Debugging Documentation](taskruns.md#debugging-a-taskrun).

### Breakpoint on Failure

Halting a TaskRun execution on Failure of a step.

#### Failure of a Step

The entrypoint binary is used to manage the lifecycle of a step. Steps are aligned beforehand by the TaskRun controller
allowing each step to run in a particular order. This is done using `-wait_file` and the `-post_file` flags. The former 
let's the entrypoint binary know that it has to wait on creation of a particular file before starting execution of the step.
And the latter provides information on the step number and signal the next step on completion of the step.

On success of a step, the `-post-file` is written as is, signalling the next step which would have the same argument given
for `-wait_file` to resume the entrypoint process and move ahead with the step. 

On failure of a step, the `-post_file` is written with appending `.err` to it denoting that the previous step has failed with
and error. The subsequent steps are skipped in this case as well, marking the TaskRun as a failure.

#### Halting a Step on failure

The failed step writes `<step-no>.err` to `/tekton/tools` and stops running completely. To be able to debug a step we would
need it to continue running (not exit), not skip the next steps and signal health of the step. By disabling step skipping, 
stopping write of the `<step-no>.err` file and waiting on a signal by the user to disable the halt, we would be simulating a 
"breakpoint".

In this breakpoint, which is essentially a limbo state the TaskRun finds itself in, the user can interact with the step 
environment using a CLI or an IDE. 

#### Exiting breakpoint

To exit a step which has been paused upon failure, the step would wait on a file similar to `<step-no>.breakpointexit` which 
would unpause and exit the step container. eg: Step 0 fails and is paused. Writing `0.breakpointexit` in `/tekton/tools`
would unpause and exit the step container.

## Debug Environment 

Additional environment augmentations made available to the TaskRun Pod to aid in troubleshooting and managing step lifecycle.

### Mounts

`/tekton/debug/scripts` : Contains scripts which the user can run to mark the step as a success, failure or exit the breakpoint.
Shared between all the containers.

`/tekton/debug/info/<n>` : Contains information about the step. Single EmptyDir shared between all step containers, but renamed 
to reflect step number. eg: Step 0 will have `/tekton/debug/info/0`, Step 1 will have `/tekton/debug/info/1` etc.

### Debug Scripts

`/tekton/debug/scripts/debug-continue` : Mark the step as completed with success by writing to `/tekton/tools`. eg: User wants to exit
breakpoint for failed step 0. Running this script would create `/tekton/tools/0` and `/tekton/tools/0.breakpointexit`.

`/tekton/debug/scripts/debug-fail-continue` : Mark the step as completed with failure by writing to `/tekton/tools`. eg: User wants to exit
breakpoint for failed step 0. Running this script would create `/tekton/tools/0.err` and `/tekton/tools/0.breakpointexit`.