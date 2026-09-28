<!--
---
linkTitle: "Profiling etcd Usage"
weight: 1000
---
-->
# Understanding and profiling etcd usage

This guide explains how a Tekton workload consumes etcd storage and how to
measure that consumption. It is intended for operators and platform builders
doing capacity planning or investigating etcd pressure.

- [Why etcd cost is more than object size](#why-etcd-cost-is-more-than-object-size)
- [The key primitive: per-key version](#the-key-primitive-per-key-version)
- [How Tekton objects are laid out in etcd](#how-tekton-objects-are-laid-out-in-etcd)
- [Profiling one object](#profiling-one-object)
- [Attributing writes to controllers](#attributing-writes-to-controllers)
- [Profiling a PipelineRun](#profiling-a-pipelinerun)
- [Estimating total etcd cost](#estimating-total-etcd-cost)
- [Caveats](#caveats)

## Why etcd cost is more than object size

etcd is an MVCC store: each write creates a new version of a key. Until etcd
compacts old revisions, those versions consume storage. A TaskRun can be
written many times as the Pipelines controller, Chains, Results, and platform
controllers update it, so its write volume can be many times the size of its
current value.

Both factors matter:

- **Object size:** bytes written by each update.
- **Write count:** how often the object is created or updated.

A small object rewritten thousands of times can cost more than a larger object
written once.

## The key primitive: per-key version

Every etcd key has a `version` field. It starts at 1 when the key is created and
increments on every subsequent write to that key. It therefore gives the number
of writes since the key's current lifetime began. Deleting and recreating a key
starts the count again.

From a kubeadm control-plane node, for example:

```bash
ETCDCTL_API=3 etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/healthcheck-client.crt \
  --key=/etc/kubernetes/pki/etcd/healthcheck-client.key \
  get /registry/minions/<node-name> -w fields \
  | grep -E '^"(CreateRevision|ModRevision|Version)"'
```

```text
"CreateRevision" : 6
"ModRevision" : 1761674
"Version" : 2028
```

`CreateRevision` and `ModRevision` are positions in the store-wide revision
sequence. `Version` is the per-key write count used in this guide.

## How Tekton objects are laid out in etcd

The API server normally stores the objects involved in a PipelineRun under
these keys:

| Object | etcd key |
| --- | --- |
| PipelineRun | `/registry/tekton.dev/pipelineruns/<namespace>/<name>` |
| TaskRun | `/registry/tekton.dev/taskruns/<namespace>/<name>` |
| Pod | `/registry/pods/<namespace>/<name>` |
| Event | `/registry/events/<namespace>/<name>` |

Tekton resources are CustomResourceDefinitions and retain their API group in
the key. Many resources compiled into the API server use an ungrouped prefix,
and some have legacy names. For example, Nodes use `/registry/minions/`, and
Services are split between `/registry/services/specs/` and
`/registry/services/endpoints/`.

Do not infer arbitrary storage keys from API resource names. Confirm the layout
by listing a prefix:

```bash
ETCDCTL_API=3 etcdctl ... get /registry/tekton.dev/ --prefix --keys-only
```

The default API server prefix is `/registry`; use the configured
`--etcd-prefix` instead when it differs.

## Profiling one object

Read the key as JSON to obtain its write count:

```bash
KEY=/registry/tekton.dev/taskruns/<namespace>/<name>

ETCDCTL_API=3 etcdctl ... get "$KEY" -w json \
  | jq '.kvs[0].version'
```

The value in etcdctl's JSON output is base64-encoded. Decode it before measuring
its current serialized size:

```bash
ETCDCTL_API=3 etcdctl ... get "$KEY" -w json \
  | jq -r '.kvs[0].value' \
  | base64 -d \
  | wc -c
```

Avoid measuring `--print-value-only` output with `wc -c`, because etcdctl adds a
trailing newline.

## Attributing writes to controllers

`managedFields` identifies managers that currently own fields and when each
manager last changed its entry:

```bash
kubectl get taskrun <name> -n <namespace> -o json --show-managed-fields \
  | jq '[.metadata.managedFields[] | {manager, operation, subresource, time}]'
```

This is not a write history. Entries can change or disappear as field ownership
moves between managers.

For per-request attribution, enable the API server
[audit log](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/) at
`Metadata` level for PipelineRuns and TaskRuns. Count successful `create`,
`update`, and `patch` events at the `ResponseComplete` stage, grouped by
`user.username`. Metadata-level logging records the actor, verb, resource, and
timestamp without logging object bodies.

Count creates as well as updates: the first create gives the key version 1.
Exclude dry-run and unsuccessful requests. Even then, audit events measure API
requests rather than the physical bytes written by etcd, so use them for actor
attribution rather than backend sizing.

## Profiling a PipelineRun

To estimate one execution's write volume, profile the PipelineRun and the
related keys that are in scope:

1. Record the PipelineRun's UID.
2. Select TaskRuns whose controller owner reference has that UID.
3. Select Pods whose controller owner reference has one of those TaskRun UIDs.
4. Select Events whose `involvedObject.uid` matches the PipelineRun, an accepted
   TaskRun, or an accepted Pod.
5. For every selected object, query its etcd key and record `version` and the
   decoded value size.

Use owner UIDs rather than labels or names alone. Labels are mutable, and a
PipelineRun name can be reused after deletion. The following commands print the
kind, name, and UID of the PipelineRun's TaskRuns and one TaskRun's Pods:

```bash
NAMESPACE=<namespace>
PIPELINERUN=<name>
PIPELINERUN_UID="$(kubectl get pipelinerun "$PIPELINERUN" -n "$NAMESPACE" \
  -o jsonpath='{.metadata.uid}')"

kubectl get taskruns -n "$NAMESPACE" -o json \
  | jq -r --arg uid "$PIPELINERUN_UID" '
      .items[]
      | select(any(.metadata.ownerReferences[]?;
          .controller == true and .uid == $uid))
      | [.kind, .metadata.name, .metadata.uid]
      | @tsv'

TASKRUN_UID=<uid-from-the-previous-command>
kubectl get pods -n "$NAMESPACE" -o json \
  | jq -r --arg uid "$TASKRUN_UID" '
      .items[]
      | select(any(.metadata.ownerReferences[]?;
          .controller == true and .uid == $uid))
      | [.kind, .metadata.name, .metadata.uid]
      | @tsv'
```

Repeat the Pod query for each TaskRun UID. Query Events once for each accepted
object UID:

```bash
OBJECT_UID=<pipelinerun-taskrun-or-pod-uid>
kubectl get events -n "$NAMESPACE" \
  --field-selector "involvedObject.uid=$OBJECT_UID" \
  -o custom-columns='KIND:.kind,NAME:.metadata.name,UID:.metadata.uid'
```

Map each kind and name to the storage-key table above, then apply the commands
in [Profiling one object](#profiling-one-object). Record the kind, name,
`version`, and decoded size in a table; sum `version`, size, and
`version × size` by kind and in total.

Decide explicitly whether the profile also includes CustomRuns, child
PipelineRuns, PVCs, and Affinity Assistant resources; they are not part of the
four-object scope above.

For a more consistent snapshot, first read the PipelineRun key and save the
etcd response header revision:

```bash
PIPELINERUN_KEY=/registry/tekton.dev/pipelineruns/<namespace>/<name>
REVISION="$(ETCDCTL_API=3 etcdctl ... get "$PIPELINERUN_KEY" -w json \
  | jq -r '.header.revision')"
```

Then add `--rev="$REVISION"` to every etcd read. This pins the values, but not
the earlier Kubernetes API discovery. Recheck object UIDs and owner references
afterward, and report objects that disappeared or changed identity rather than
silently attributing a replacement to the run.

A three-task sequential PipelineRun measured immediately after completion
produced this aggregate:

```text
KIND            COUNT  WRITES     CURRENT(B) EST-WRITE-BYTES(B)
Event              65      70          41081              44031
PipelineRun         1       6           3029              18174
Pod                 3      39          32175             418275
TaskRun             3      27           9981              89829
----------------------------------------------------------------
TOTAL              72     142          86266             570309
```

Each TaskRun was written nine times and each Pod thirteen times. Events
increased the object count but contributed little estimated write volume
because most were written once. Keep per-object rows in the underlying analysis
so a single hot TaskRun or Pod is not hidden by the aggregate.

## Estimating total etcd cost

For a cheap first-order estimate, calculate this for every key and sum the
results:

```text
estimated write bytes = version × current serialized size
```

The example above has an estimated rewrite multiple of `570309 / 86266`, or
about 6.6x. This estimate assumes every historical value was the same size as
the current value. It overestimates objects that grew and underestimates objects
that shrank.

Exact historical payload volume requires observing or replaying each PUT before
its revision is compacted. Even that is not the physical backend cost: etcd also
stores keys, MVCC metadata, indexes, and page overhead. Treat the estimate as a
way to find expensive object types and write patterns, not as a measurement of
current disk use or quota consumption.

For backend capacity, monitor `etcd_mvcc_db_total_size_in_bytes`, the physical
size used for quota and `NOSPACE` decisions.
`etcd_mvcc_db_total_size_in_use_in_bytes` excludes free pages; the difference
between the two is space that defragmentation can reclaim.

## Caveats

- **Compaction:** A key's `version` keeps increasing after old revisions are
  compacted. `version × current size` estimates cumulative write volume, not
  retained history. Compaction also does not return free pages to the
  filesystem; defragmentation does.
- **Live-object discovery:** Objects already deleted or garbage-collected are
  absent from an API listing even if their old etcd revisions have not yet been
  compacted. Events commonly expire after one hour, so profile soon after the
  PipelineRun finishes.
- **Encryption at rest:** An
  [EncryptionConfiguration](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/)
  makes the stored value ciphertext. Its size is measurable, but it cannot be
  decoded as a Kubernetes object to verify identity.
- **Privileges:** Direct etcd access is highly privileged. Client keys on a
  kubeadm control-plane node are normally root-only. Follow your platform's
  supported access procedure and do not copy credentials off the node.
- **Multiple etcd clusters:** `--etcd-servers-overrides` can place built-in
  resources such as Events in another etcd cluster. Revisions are local to each
  cluster, so query the configured endpoint for each resource and do not treat
  revision numbers from separate clusters as one timeline.
- **Snapshot lifetime:** A pinned revision must remain available until all reads
  finish. If compaction removes it first, restart the measurement rather than
  mixing values from different revisions.
