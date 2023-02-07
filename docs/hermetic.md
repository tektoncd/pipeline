<!--
---
linkTitle: "Hermetic"
weight: 410
---
-->

# Hermetic Execution Mode
A Hermetic Build is a release engineering best practice for increasing the reliability and consistency of software builds.
They are self-contained, and do not depend on anything outside of the build environment.
This means they do not have network access, and cannot fetch dependencies at runtime.

When hermetic execution mode is enabled, all TaskRun steps will be run without access to a network.
_Note: hermetic execution mode does NOT apply to sidecar containers_ 

Hermetic execution mode is currently an alpha experimental feature. 

## Enabling Hermetic Execution Mode
To enable hermetic execution mode:
1. Make sure `enable-api-fields` is set to `"alpha"` in the `feature-flags` configmap, see [`install.md`](./install.md#customizing-the-pipelines-controller-behavior) for details
1. Set the following annotation on any TaskRun you want to run hermetically:

```yaml
experimental.tekton.dev/execution-mode: hermetic
```

## Sample Hermetic TaskRun
This example TaskRun demonstrates running a container in a hermetic environment.

The Step attempts to install curl, but this step **SHOULD FAIL** if the hermetic environment is working as expected.

```yaml
kind: TaskRun
apiVersion: tekton.dev/v1beta1
metadata:
  generateName: hermetic-should-fail
  annotations:
    experimental.tekton.dev/execution-mode: hermetic
spec:
  timeout: 60s
  taskSpec:
    steps:
    - name: hermetic
      image: ubuntu
      script: |
        #!/usr/bin/env bash
        apt-get update
        apt-get install -y curl
```

## Further Details
To learn more about hermetic execution mode, check out the [TEP](https://github.com/tektoncd/community/blob/main/teps/0025-hermekton.md).
