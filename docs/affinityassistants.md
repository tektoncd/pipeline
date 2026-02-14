<!--
---
linkTitle: "Affinity Assistants"
weight: 405
---
-->

# Affinity Assistants

Affinity Assistant is a feature to coschedule `PipelineRun` `pods` to the same node
based on [kubernetes pod affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity) so that it possible for the taskruns to execute parallel while sharing volume.
Available Affinity Assistant Modes are **coschedule workspaces**, **coschedule pipelineruns**,
**isolate pipelinerun** and **disabled**.

> :seedling: **coschedule pipelineruns** and **isolate pipelinerun** modes are [**alpha features**](./additional-configs.md#alpha-features).
> **coschedule workspaces** is a **stable feature**

* **coschedule workspaces** - When a `PersistentVolumeClaim` is used as volume source for a `Workspace` in a `PipelineRun`,
all `TaskRun` pods within the `PipelineRun` that share the `Workspace` will be scheduled to the same Node. (**Note:** Only one pvc-backed workspace can be mounted to each TaskRun in this mode.)

* **coschedule pipelineruns** - All `TaskRun` pods within the `PipelineRun` will be scheduled to the same Node.

* **isolate pipelinerun** - All `TaskRun` pods within the `PipelineRun` will be scheduled to the same Node,
and only one PipelineRun is allowed to run on a node at a time.

* **disabled** - The Affinity Assistant is disabled. No pod coscheduling behavior.

This means that Affinity Assistant is incompatible with other affinity rules
configured for the `TaskRun` pods (i.e. other affinity rules specified in custom [PodTemplate](pipelineruns.md#specifying-a-pod-template) will be overwritten by Affinity Assistant).
If the `PipelineRun` has a custom [PodTemplate](pipelineruns.md#specifying-a-pod-template) configured, the `NodeSelector` and `Tolerations` fields will also be set on the Affinity Assistant pod. The Affinity Assistant
is deleted when the `PipelineRun` is completed.

The Affinity Assistant Modes are configured by the `coschedule` feature flag.
Previously, it was also controlled by the `disable-affinity-assistant` feature flag which was deprecated and removed after release `v0.68`.

The following chart summarizes the Affinity Assistant Modes with different combinations of the `disable-affinity-assistant` and `coschedule` feature flags during migration (when both feature flags are present) and after the migration (when only the `coschedule` flag is present).

<table>
    <thead>
        <tr>
            <th>disable-affinity-assistant</th>
            <th>coschedule</th>
            <th>behavior during migration</th>
            <th>behavior after migration</th>
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>false (default)</td>
            <td>disabled</td>
            <td>N/A: invalid</td>
            <td>disabled</td>
        </tr>
        <tr>
            <td>false (default)</td>
            <td>workspaces (default)</td>
            <td>coschedule workspaces</td>
            <td>coschedule workspaces</td>
        </tr>
        <tr>
            <td>false (default)</td>
            <td>pipelineruns</td>
            <td>N/A: invalid</td>
            <td>coschedule pipelineruns</td>
        </tr>
        <tr>
            <td>false (default)</td>
            <td>isolate-pipelinerun</td>
            <td>N/A: invalid</td>
            <td>isolate pipelinerun</td>
        </tr>
        <tr>
            <td>true</td>
            <td>disabled</td>
            <td>disabled</td>
            <td>disabled</td>
        </tr>
        <tr>
            <td>true</td>
            <td>workspaces (default)</td>
            <td>disabled</td>
            <td>coschedule workspaces</td>
        </tr>
        <tr>
            <td>true</td>
            <td>pipelineruns</td>
            <td>coschedule pipelineruns</td>
            <td>coschedule pipelineruns</td>
        </tr>
        <tr>
            <td>true</td>
            <td>isolate-pipelinerun</td>
            <td>isolate pipelinerun</td>
            <td>isolate pipelinerun</td>
        </tr>
    </tbody>
</table>

**Note:** After release `v0.68`, the `disable-affinity-assistant` feature flag is removed and the Affinity Assistant Modes are only controlled by the `coschedule` feature flag.

**Note:** Affinity Assistant use [Inter-pod affinity and anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity)
that require substantial amount of processing which can slow down scheduling in large clusters
significantly. We do not recommend using the affinity assistant in clusters larger than several hundred nodes

**Note:** Pod anti-affinity requires nodes to be consistently labelled, in other words every
node in the cluster must have an appropriate label matching `topologyKey`. If some or all nodes
are missing the specified `topologyKey` label, it can lead to unintended behavior.

## ServiceAccount Configuration

By default, Affinity Assistant pods inherit the `serviceAccountName` from the PipelineRun's
`spec.taskRunTemplate.serviceAccountName`. This ensures the Affinity Assistant has the same
permissions as TaskRun pods, which is particularly important in security-restricted environments
like OpenShift with Security Context Constraints (SCC).

### Default Behavior

When you specify a ServiceAccount in your PipelineRun, the Affinity Assistant automatically inherits it:

```yaml
apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: example-pipelinerun
spec:
  taskRunTemplate:
    serviceAccountName: my-service-account  # Affinity Assistant inherits this
  pipelineSpec:
    # ... pipeline definition
```

### Overriding ServiceAccount

You can override the ServiceAccount for Affinity Assistant pods using the cluster-wide default configuration:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-defaults
  namespace: tekton-pipelines
data:
  default-affinity-assistant-pod-template: |
    serviceAccountName: affinity-assistant-sa
    nodeSelector:
      disktype: ssd
```

**Note:** Any time during the execution of a `pipelineRun`, if the node with a placeholder Affinity Assistant pod and
the `taskRun` pods sharing a `workspace` is `cordoned` or disabled for scheduling anything new (`tainted`), the
`pipelineRun` controller deletes the placeholder pod. The `taskRun` pods on a `cordoned` node continues running
until completion. The deletion of a placeholder pod triggers creating a new placeholder pod on any available node
such that the rest of the `pipelineRun` can continue without any disruption until it finishes.

## PVC Auto-Cleanup for Workspaces Mode

By default, in `coschedule workspaces` mode, PVCs created from `volumeClaimTemplate` workspaces are NOT automatically
deleted when the PipelineRun completes. The PVCs remain in the cluster and can be reused or manually cleaned up.

To enable automatic PVC cleanup on PipelineRun completion, add the `tekton.dev/auto-cleanup-pvc` annotation to your
PipelineRun:

```yaml
apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: example-pipelinerun
  annotations:
    tekton.dev/auto-cleanup-pvc: "true"
spec:
  pipelineSpec:
    workspaces:
    - name: my-workspace
    tasks:
    - name: my-task
      workspaces:
      - name: my-workspace
      taskSpec:
        steps:
        - image: busybox
          script: echo "Hello, World!"
  workspaces:
  - name: my-workspace
    volumeClaimTemplate:
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
```

**Important notes:**

- This annotation only affects `volumeClaimTemplate` workspaces. User-provided `persistentVolumeClaim` workspaces
  are **never** deleted automatically, even with this annotation set.
- The annotation value must be exactly `"true"` to enable auto-cleanup. Any other value (including `"false"`, `"yes"`, etc.)
  will keep the default behavior of not deleting PVCs.
- In `coschedule pipelineruns` and `isolate-pipelinerun` modes, PVCs are always deleted on completion regardless of this annotation.
- In `disabled` mode, no Affinity Assistant is created and PVC lifecycle is managed by Kubernetes garbage collection
  (PVCs with `ownerReference` to the PipelineRun will be deleted when the PipelineRun is deleted).