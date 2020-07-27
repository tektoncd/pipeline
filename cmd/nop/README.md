# `nop` Image

This image is responsible for two internal functions of Tekton:

1. Stopping sidecar containers[#stopping-sidecar-containers]
1. Affinity Assistant StatefulSet[#affinity-assistant-statefulset]

The `nop` image satisfies these two functions with a minimally small image,
both to optimize image pull latency and to present a minimal surface for a
potential attacker.

## Stopping sidecar containers

When all steps in a TaskRun are complete, Tekton attempts to gracefully stop
any running sidecar containers, by replacing their `image` with an image that
exits immediately, regardless of any `args` passed to the container.

When the `nop` image is run with any args (except one unique string, described
[below](#affinity-assistant-statefulset)), it will exit with the exit code zero
immediately.

* **NB:** If the sidecar container has its `command` specified, the `nop`
  binary will not be invoked, and may exit with a non-zero exit code. Tekton
  will not interpret this as a TaskRun failure, but it may result in noisy
  logs/metrics being emitted.

## Affinity Assistant StatefulSet

The Affinity Assistant, which powers [workspaces](docs/workspaces.md), works
by running a
[`StatefulSet`](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/)
with an container that runs indefinitely. This container doesn't need to _do_
anything, it just needs to exist.

When the `nop` image is passed the string `tekton_run_indefinitely` (a unique,
Tekton-identified string), it will run indefinitely until it receives a signal
to terminate. The affinity assistant StatefulSet passes this arg to ensure its
container runs indefinitely.
