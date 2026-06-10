<!--
---
linkTitle: "Security"
weight: 313
---
-->

# Security Best Practices

- [Overview](#overview)
- [Workspace isolation across trust boundaries](#workspace-isolation-across-trust-boundaries)
  - [Trust model](#trust-model)
  - [Recommended pattern: per-run `Workspaces`](#recommended-pattern-per-run-workspaces)
  - [Avoid sharing PVCs across trust boundaries](#avoid-sharing-pvcs-across-trust-boundaries)
  - [`ReadWriteMany` PVCs](#readwritemany-pvcs)
  - [Checklist](#checklist)

## Overview

Tekton Pipelines runs `Tasks` and `Pipelines` as Kubernetes workloads. Security
properties such as namespace isolation, RBAC, admission control, service account
permissions, storage access, and network access are provided by Kubernetes and
by the platform configuration around Tekton.

This page describes security best practices for authoring and operating Tekton
`TaskRuns` and `PipelineRuns` where lower-trust workloads could affect
higher-trust workloads.

## Workspace isolation across trust boundaries

`Workspaces` are a way for `TaskRuns` and `PipelineRuns` to mount Kubernetes
volumes. They can be used to share source, build outputs, caches, credentials,
and other files between `Tasks`. When a workspace is backed by a
`PersistentVolumeClaim`, the security properties of that workspace are the
security properties of the underlying Kubernetes volume.

Treat every shared workspace as part of the trust model for the `Tasks` and
`PipelineRuns` that can mount it. For example, a `PipelineRun` that validates an
untrusted pull request and a `PipelineRun` that builds or publishes a trusted
release should not share the same writeable PVC.

### Trust model

Tekton does not enforce filesystem-level isolation between separate
`PipelineRuns` that intentionally mount the same persistent workspace. If two
`PipelineRuns` bind the same PVC to a workspace, the Pods created for both runs
can access the same files according to the PVC access mode, Kubernetes volume
permissions, and the container security context.

This behavior is by design: workspace sharing is explicit and opt-in. It also
means that isolation between trusted and untrusted workloads is the
responsibility of the pipeline author and cluster operator. Use Kubernetes RBAC,
namespaces, ServiceAccounts, and storage policy to limit which users and
controllers can create `PipelineRuns` that reference sensitive PVCs.

`readOnly` workspace bindings can prevent writes from that mount, but they do
not make existing contents trustworthy. Do not treat a shared volume as safe for
a higher-trust run only because the higher-trust run mounts it read-only.

### Recommended pattern: per-run `Workspaces`

Use `volumeClaimTemplate` when a `PipelineRun` needs a writeable persistent
workspace but should not share files with other runs. A `volumeClaimTemplate`
creates a new PVC for each `PipelineRun` or `TaskRun`, so data written by one run
is not visible through the same workspace binding in another run.

```yaml
apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  generateName: validate-pr-
spec:
  pipelineRef:
    name: validate-pr
  workspaces:
    - name: source
      volumeClaimTemplate:
        spec:
          accessModes:
            - ReadWriteOnce
          resources:
            requests:
              storage: 1Gi
```

PVCs created from `volumeClaimTemplate` are automatically provisioned. Their
cleanup behavior depends on the `PipelineRun` lifecycle and the configured
Affinity Assistant mode. See [PVC Auto-Cleanup for Workspaces Mode](../affinityassistants.md#pvc-auto-cleanup-for-workspaces-mode)
for details on cleanup after completion.

### Avoid sharing PVCs across trust boundaries

Only bind an existing `persistentVolumeClaim` to a workspace when every
`PipelineRun` that can mount that claim belongs to the same trust domain and is
allowed to read or modify the files on that volume.

Avoid patterns like the following when the `PipelineRuns` are triggered by
inputs at different trust levels, such as untrusted pull requests and trusted
release automation:

```yaml
workspaces:
  - name: artifacts
    persistentVolumeClaim:
      claimName: shared-artifacts-rwx
```

A shared PVC can allow one run to read files left by another run, modify files
that a later run will consume, or race with another run that is reading or
writing the same paths. A `subPath` on a shared PVC can help organize data, but
it should not be treated as a security boundary between trust domains.

If persistent data must cross a trust boundary, write it to a controlled storage
or artifact system that performs the appropriate validation, signing, promotion,
or access checks before a trusted pipeline consumes it.

Step-level [Isolated `Workspaces`](../workspaces.md#isolated-workspaces) can limit
which `Steps` and `Sidecars` inside one `Task` receive a workspace. That feature
does not isolate separate `PipelineRuns` that bind the same PVC.

### `ReadWriteMany` PVCs

`ReadWriteMany` is a Kubernetes access mode that allows a volume to be mounted
as read-write by multiple Nodes at the same time, when supported by the storage
provider. Any Pod that is allowed to mount a `ReadWriteMany` PVC can access the
same filesystem concurrently. This is Kubernetes volume behavior, not behavior
specific to Tekton.

Do not use `ReadWriteMany` as an isolation mechanism. It is useful for shared
caches and coordinated workflows inside a single trust domain, but it increases
the impact of accidentally sharing a PVC between unrelated or differently
trusted `PipelineRuns`.

`ReadWriteOnce` is also not a trust boundary. It limits where the volume can be
mounted, but it does not clear previous contents or prevent a later run with
access to the same PVC from reading or modifying those contents.

### Checklist

- Use `volumeClaimTemplate` for writeable workspaces that should be isolated per
  `PipelineRun` or `TaskRun`.
- Do not share an existing PVC between pipelines that run code, inputs, or
  credentials from different trust levels.
- Treat `ReadWriteMany` PVCs as concurrently shared filesystems accessible to
  any Pod that can mount them.
- Do not rely on `subPath`, access mode, or task ordering as a security boundary
  between trusted and untrusted workloads.
- Use separate namespaces, ServiceAccounts, RBAC, and storage classes or claims
  for separate trust domains.
- Promote artifacts across trust boundaries through systems that validate and
  authorize the data before trusted pipelines consume it.
