<!--
---
linkTitle: "Profiling etcd Usage"
weight: 1000
---
-->
# Understanding and profiling etcd usage

This guide explains how a Tekton workload consumes etcd storage, and how to
measure that consumption for your own pipelines. It is aimed at operators and
platform builders doing capacity planning or chasing etcd pressure.

- [Why etcd cost is more than the object size](#why-etcd-cost-is-more-than-the-object-size)
- [The key primitive: per-object revision count](#the-key-primitive-per-object-revision-count)
- [How Tekton objects are laid out in etcd](#how-tekton-objects-are-laid-out-in-etcd)
- [Profiling a single object](#profiling-a-single-object)
- [Attributing revisions to controllers](#attributing-revisions-to-controllers)
- [Profiling a whole PipelineRun](#profiling-a-whole-pipelinerun)
- [Estimating total etcd cost](#estimating-total-etcd-cost)
- [Caveats](#caveats)

## Why etcd cost is more than the object size

etcd is an MVCC store: every write to a key creates a new revision that holds a
full copy of the value. Between [compaction](https://etcd.io/docs/latest/op-guide/maintenance/#history-compaction)
cycles, all of those revisions occupy storage. A single TaskRun may be written
10-20 times over its lifecycle as multiple controllers update it (the Pipelines
controller, plus Chains, Results, and platform controllers), so its real etcd
footprint is many times the size of its "live" snapshot.

The dominant factor is usually revision count, not object size. A small object
that is rewritten thousands of times can cost far more than a large object
written once. For example, on a quiet single-node cluster the `Node` object is
only ~20 KB but has accumulated 2028 revisions from status heartbeats, while a
13 KB ConfigMap written once sits at a single revision.

## The key primitive: per-object revision count

Every key in etcd carries a `version` field: it starts at 1 when the key is
created and increments on every write. So `version` is the number of revisions
the object has accumulated since creation, which is the quantity you want when
reasoning about etcd cost.

You can read it with `etcdctl`. From a kubeadm control-plane node:

```bash
ETCDCTL_API=3 etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/healthcheck-client.crt \
  --key=/etc/kubernetes/pki/etcd/healthcheck-client.key \
  get /registry/minions/<node-name> -w fields | grep -E 'Version|Revision'
```

```
"CreateRevision" : 6
"ModRevision" : 1761674
"Version" : 2028
```

`CreateRevision`/`ModRevision` are global logical clocks (the same counter for
the whole store); `Version` is per-key and is the one to track.

## How Tekton objects are laid out in etcd

The apiserver stores each object under a `/registry/...` key:

| Object | Group | etcd key |
| --- | --- | --- |
| PipelineRun | `tekton.dev` | `/registry/tekton.dev/pipelineruns/<ns>/<name>` |
| TaskRun | `tekton.dev` | `/registry/tekton.dev/taskruns/<ns>/<name>` |
| Pod | core | `/registry/pods/<ns>/<name>` |
| Event | core | `/registry/events/<ns>/<name>` |

The rule: core-group resources have no group segment
(`/registry/<resource>/<ns>/<name>`); every other API group inserts the group
(`/registry/<group>/<resource>/<ns>/<name>`). A few built-in resources use a
legacy storage name that differs from their API name; the one to watch is
`Node`, stored under `/registry/minions/`.

List the keys for a namespace to see the layout directly:

```bash
etcdctl ... get /registry/tekton.dev/ --prefix --keys-only
```

## Profiling a single object

`version` plus the current value size already tells you most of the story:

```bash
# revision count: writes to the key since it was created
etcdctl ... get /registry/tekton.dev/taskruns/<ns>/<name> -w json | jq '.kvs[0].version'
# current value size: the value is binary protobuf, so measure the raw bytes
etcdctl ... get /registry/tekton.dev/taskruns/<ns>/<name> --print-value-only | wc -c
```

The helper in [`hack/etcd-revision-profile`](../../hack/etcd-revision-profile)
wraps this:

```bash
go run ./hack/etcd-revision-profile -etcd-key /registry/minions/<node-name>
```

```
/registry/minions/<node-name>
  revisions (version): 2028
  current value bytes: 20243
```

## Attributing revisions to controllers

To find *which* controller is driving the writes, look at `managedFields`: each
manager that has written the object appears with the subresource and time of its
last update.

```bash
kubectl get taskrun <name> -n <ns> -o json \
  | jq '[.metadata.managedFields[] | {manager, operation, subresource, time}]'
```

For an exact per-revision attribution (who wrote each individual revision),
enable the apiserver [audit log](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/)
at `RequestResponse` level for `taskruns`/`pipelineruns` and count `update` and
`patch` events grouped by `user.username`. `managedFields` shows the set of
writers and their last write; the audit log shows the full history.

## Profiling a whole PipelineRun

To see the aggregate footprint of a PipelineRun and everything it spawns
(child TaskRuns, their Pods, and associated Events), use the helper with
`-pipelinerun`:

```bash
go run ./hack/etcd-revision-profile -n <ns> -pipelinerun <name>
```

```
etcd revision profile for PipelineRun default/etcd-demo

KIND            COUNT  REVISIONS     CURRENT(B)   EST-REV-BYTES(B)
Event              65         74          41331              46438
PipelineRun         1          6           3887              23322
Pod                 3         39          32366             420758
TaskRun             3         27          10353              93177
------------------------------------------------------------------
TOTAL              72        146          87937             583695

revision amplification (est-rev-bytes / current-bytes): 6.6x
```

The figures above are from a real run of a small three-task sequential
pipeline. Even at that size each TaskRun took about nine revisions and each Pod
about thirteen, so the stored revision bytes run several times the live
snapshot; a production pipeline with tens of TaskRuns multiplies that further.
The helper discovers TaskRuns and Pods by the `tekton.dev/pipelineRun` label,
and Events by their `involvedObject` kind and name, then reads each object's
`version` and size from etcd.

## Estimating total etcd cost

The exact pre-compaction cost is the sum of the sizes of *every* revision, which
requires replaying history with `etcdctl get --rev=<n>`. As a cheap estimate,
the helper reports `EST-REV-BYTES = Σ (version × current_size)`, i.e. it assumes
each revision was about the size of the current one. This over-counts objects
that grew over time and under-counts objects that shrank, but it is a good
first-order signal for "where is my etcd budget going".

The `revision amplification` ratio (estimated revision bytes ÷ current bytes)
summarizes how much MVCC history inflates the live footprint.

## Caveats

- Compaction resets the picture. Revisions older than the last compaction are
  gone; `version` keeps counting but the storage they used has been reclaimed,
  so measure relative to your compaction interval.
- Values may be encrypted at rest. With an
  [EncryptionConfiguration](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/)
  the etcd value is ciphertext, so the size is meaningful but it is not the
  decoded object.
- Some built-in resources use a legacy key. `Node` lives at
  `/registry/minions/`; if a lookup returns "not found", list the prefix with
  `--prefix --keys-only` to find the real key.
- Reading etcd needs root, since the client keys under
  `/etc/kubernetes/pki/etcd/` are root-only. Run `etcdctl` and the helper with
  `sudo` (the helper does so by default; disable with `-sudo=false`).
