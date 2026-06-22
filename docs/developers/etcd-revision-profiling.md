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
  get /registry/minions/<node-name> -w fields | grep -E '^"(CreateRevision|ModRevision|Version)"'
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

The group segment is not simply the API group. Resources served by
CustomResourceDefinitions, Tekton's included, are stored under their group
(`/registry/<group>/<resource>/<ns>/<name>`). Most groups compiled into the
apiserver instead register a bare per-resource prefix, so Deployments live at
`/registry/deployments/<ns>/<name>` and not under `/registry/apps/`, and the
same goes for `leases`, `clusterroles` and the rest. The exceptions run both
ways: `apiextensions.k8s.io` and `apiregistration.k8s.io` are compiled in and
still keep their group, `Node` is stored at `/registry/minions/<name>`, and
Services are split across `/registry/services/specs/` and
`/registry/services/endpoints/`.

So derive keys only for Tekton's own resources, Pods and Events. For anything
else, ask etcd rather than guessing.

List the keys for a namespace to see the layout directly:

```bash
etcdctl ... get /registry/tekton.dev/ --prefix --keys-only
```

## Profiling a single object

`version` plus the current value size already tells you most of the story:

```bash
# revision count: writes to the key since it was created
etcdctl ... get /registry/tekton.dev/taskruns/<ns>/<name> -w json | jq '.kvs[0].version'
# current value size: decode the value out of the JSON, since --print-value-only
# adds a trailing newline that wc would count
etcdctl ... get /registry/tekton.dev/taskruns/<ns>/<name> -w json \
  | jq -r '.kvs[0].value' | base64 -d | wc -c
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

To find *which* controller is driving the writes, look at `managedFields`. It
records who currently manages each field and when they last did so. It is not a
write history: entries change as field ownership moves between managers, and a
manager that has stopped owning a field disappears from it. `kubectl` hides the
section unless asked for it.

```bash
kubectl get taskrun <name> -n <ns> -o json --show-managed-fields \
  | jq '[.metadata.managedFields[] | {manager, operation, subresource, time}]'
```

For per-write attribution (who issued each write), enable the apiserver
[audit log](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/) at
`Metadata` level for `taskruns`/`pipelineruns` and count `create`, `update` and
`patch` events grouped by `user.username`. `Metadata` records the user, verb,
resource and timestamp without the request and response bodies, which is enough
for attribution and avoids logging object contents. `managedFields` shows the
set of writers and their last write; the audit log shows the full history.

Count the create as well as the updates: a key's `version` is 1 the moment it is
created and rises by one per write after that, so the number an audit log has to
account for is `version`, not `version - 1`.

Audit entries are API requests, not confirmed etcd writes: a request can be
rejected by admission, fail on conflict, be a dry run, or be short-circuited by
the reconciler before it reaches storage. Filter to `ResponseComplete` with a
success status, and treat the count as a close upper bound on writes.

## Profiling a whole PipelineRun

To see the aggregate footprint of a PipelineRun, its TaskRuns, their Pods and
the Events for all of those, use the helper with `-pipelinerun`:

```bash
go run ./hack/etcd-revision-profile -n <ns> -pipelinerun <name>
```

```
etcd revision profile for PipelineRun default/etcd-demo-wl52g (discovered keys read at revision 11677152)

KIND           NAME                                          REVISIONS     CURRENT(B) EST-WRITE-BYTES(B)
Event          etcd-demo-wl52g-build-pod.18c7aeb5ac278b09            1            614                614
Event          etcd-demo-wl52g-build-pod.18c7aeb5fa5c96c3            1            580                580
Event          etcd-demo-wl52g-build-pod.18c7aeb60c8f433d            1            835                835
        ... 62 more Event rows omitted from this excerpt ...
PipelineRun    etcd-demo-wl52g                                       6           3029              18174
Pod            etcd-demo-wl52g-build-pod                            13          10725             139425
Pod            etcd-demo-wl52g-package-pod                          13          10729             139477
Pod            etcd-demo-wl52g-test-pod                             13          10721             139373
TaskRun        etcd-demo-wl52g-build                                 9           3326              29934
TaskRun        etcd-demo-wl52g-package                               9           3334              30006
TaskRun        etcd-demo-wl52g-test                                  9           3321              29889

KIND            COUNT  REVISIONS     CURRENT(B) EST-WRITE-BYTES(B)
Event              65         70          41081              44031
PipelineRun         1          6           3029              18174
Pod                 3         39          32175             418275
TaskRun             3         27           9981              89829
------------------------------------------------------------------
TOTAL              72        142          86266             570309

estimated rewrite multiple (est-write-bytes / current-bytes): 6.6x
```

The per-object rows are the point: they say *which* TaskRun or Pod is driving
the writes, which a per-Kind total hides. Only three of the 65 Event rows are
shown above; the tool prints them all.

The figures are from a real three-task sequential pipeline, profiled right after
it finished so its Events were still alive. Each TaskRun was written nine times
and each Pod thirteen, so the bytes written over their lives run to several
times the live snapshot; a production pipeline with tens of TaskRuns multiplies
that further. Events dominate the object count and barely register in bytes:
they are written about once each, which is what pulls the overall multiple down
to 6.6x. Profile the same run an hour later, after the Events expire, and the
remaining objects alone come out around twice that.

The helper does not use labels at all. They are mutable, can be copied onto an
unrelated object, and the UID one only exists from Tekton v0.63, so a child
whose labels were stripped would be missing from a report that still called
itself complete. It lists the namespace's TaskRuns and Pods and keeps those whose
controller owner reference points at this PipelineRun, or at one of its
accepted TaskRuns for Pods. Tekton's own reconciler settles membership the same
way when it rebuilds a run's child references, which is how it leaves out
TaskRuns a custom task created for itself. Owning is also what rejects objects
orphaned by an earlier run that used this name.
An object this run does not own is not one of its children rather than
something withheld from the count, so nothing is reported about it. Events are
found by `involvedObject.uid` on the accepted objects.

The PipelineRun is read first and the store-wide revision that read was served
at is reused for every other key, so every value in the table comes from the same
revision rather than from a rolling read that other controllers can write into
halfway through. That revision is printed in the header. The set of rows is
still whatever the API returned during discovery, which happens first, so an
object created in between exists at that revision without appearing in the row
set. The listing taken after the reads catches those: anything this run owns
that was not read is looked up at the pinned revision, and reported as not
counted if it was already there. One created after that revision is correctly
absent and is not reported. If the PipelineRun's own key is gone, or now holds a
different UID, the helper stops instead of reporting the leftovers.

Once every key has been read, the identity of each object the profile actually
counted is confirmed again through the API. An object whose identity cannot be
confirmed is left out of the rows and the totals rather than reported under the
identity it was discovered as, and if that object is the PipelineRun itself the
run stops, since nothing else in the report could then be attributed to it. Objects already reported as missing
are left out of that check: they are expected to look replaced, and rechecking
them would turn a result `-allow-partial` exists to accept into a hard failure. That listing carries no label
selector, for the same reason discovery does not: labels are mutable and prove
nothing about identity. Both the UID and the
controller owner are compared, so an object that kept its UID but was
re-parented is caught as well. A name that has simply disappeared is fine: it
was measured at the pinned revision and those figures still describe it. This
check does not read stored values, so it is the one that still works where
encryption at rest leaves no readable UID to compare. That also decides what a
vanished name means: with `-verify-uid` on, the stored value already confirmed
the generation and the figures stand, but with it off nothing did, so the run
stops rather than attribute unconfirmed figures.

Objects are read under the apiserver's `--etcd-prefix`, which defaults to
`/registry`. Pass `-etcd-prefix` if the cluster sets a different one.

What the helper does not cover: CustomRuns, child PipelineRuns from
Pipelines-in-Pipelines, PVCs, and Affinity Assistant StatefulSets. It does
report the CustomRuns and child PipelineRuns a run owns, so a pipeline built out
of them is not mistaken for one with no children; they are owned the same way as
TaskRuns and could be profiled the same way if that is worth doing. Whatever a
custom task's own controller creates downstream stays opaque, since nothing ties
it back to the PipelineRun.

## Estimating total etcd cost

Adding up the sizes of *every* revision means replaying the writes themselves,
which is what etcd's watch stream from an uncompacted revision gives you: one
event per PUT, each carrying that write's own value. Point-in-time reads cannot
do it, because `etcdctl get --rev=<n>` returns whatever the key held at global
revision `n`, so stepping `n` forward returns the same value again and again
whenever some other key was what changed. Even a perfect sum of those payloads
is not the backend cost, which also carries MVCC metadata, keys, index and page
overhead. As a cheap estimate,
the helper reports `EST-WRITE-BYTES = Σ (version × current_size)`, i.e. it assumes
each revision was about the size of the current one. This over-counts objects
that grew over time and under-counts objects that shrank, but it is a good
first-order signal for "where is my etcd budget going".

The `estimated rewrite multiple` (estimated write bytes ÷ current bytes) summarizes how many times over each object has been rewritten. Read it
as write volume, not as bytes etcd is holding now: compaction keeps the
current revision of every live key indefinitely and discards the superseded
ones, so with the default 5 minute interval most of the history counted here is
already gone.

## Caveats

- Compaction resets the picture. Revisions older than the last compaction are
  gone; `version` keeps counting but the storage they used has been reclaimed.
  The apiserver compacts every 5 minutes by default
  (`--etcd-compaction-interval`), so for anything but a very short run most of
  the history counted here is no longer stored. Compaction also does not return
  space to the filesystem on its own, which needs a defragmentation. Size the
  cluster from `etcd_mvcc_db_total_size_in_bytes`, the physical backend size
  the space quota and the `NOSPACE` alarm act on;
  `etcd_mvcc_db_total_size_in_use_in_bytes` excludes free pages, so the gap
  between the two is what a defragmentation would give back.
- The profile is a live-key snapshot. Discovery lists what the API server can
  still see, so objects already deleted or garbage collected are absent even
  when their revisions have not been compacted yet. Events matter most here:
  they default to a one hour TTL, so run the helper soon after the PipelineRun
  finishes. Objects that disappear between discovery and the etcd read are
  reported on stderr as skipped, and the profile says how many were lost.
- Values may be encrypted at rest. With an
  [EncryptionConfiguration](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/)
  the etcd value is ciphertext, so the size is meaningful but it is not the
  decoded object.
- Some built-in resources use a legacy key. `Node` lives at
  `/registry/minions/`; if a lookup returns "not found", list the prefix with
  `--prefix --keys-only` to find the real key.
- Reading etcd needs root, since the client keys under
  `/etc/kubernetes/pki/etcd/` are root-only. Run `etcdctl` and the helper with
  `sudo`. The helper wraps its own `etcdctl` calls in `sudo` by default;
  disable that with `-sudo=false` if `etcdctl` can already read the keys.
- One endpoint reaches one etcd cluster. `kube-apiserver` can route individual
  resources elsewhere with `--etcd-servers-overrides`, most often to keep
  Events off the main cluster (`/events#https://etcd-events:2379`). Aggregate
  mode does not support that: it takes a single `-endpoints`, and every object
  of an overridden resource is reported as missing. Running it a second time
  against the other endpoint does not help either, because the PipelineRun it
  anchors on does not live there, and revisions are local to a cluster so the
  two runs could not be combined anyway. Check the apiserver flags first; for an
  overridden resource, inspect individual keys with `-etcd-key` against its own
  endpoint. The override applies only to resources compiled into the apiserver,
  so it never moves Tekton's own objects, only Pods and Events.
- Storage keys hold names, not UIDs, so a name deleted and recreated between
  discovery and the read would otherwise be counted as the object that was
  discovered. The helper compares the UID it discovered against the stored
  value and skips the object when they differ. Encrypted values carry no
  plaintext UID and would all fail that check, so pass `-verify-uid=false`
  on a cluster using encryption at rest.
- Ownership, not labels, decides what belongs to a run, so objects orphaned by
  `kubectl delete pipelinerun <name> --cascade=orphan` are not attributed to a
  later run that reuses the name, and a child whose labels were stripped is
  still counted. It also means a run whose objects straddle a Tekton upgrade is
  found whole, without depending on the `tekton.dev/pipelineRunUID` label that
  only exists from v0.63.
- The pinned revision has to survive the run. The apiserver compacts every 5
  minutes by default, so on a very large PipelineRun the reads can outlast it;
  etcd then rejects the remaining gets and the helper stops rather than
  silently mixing revisions.
