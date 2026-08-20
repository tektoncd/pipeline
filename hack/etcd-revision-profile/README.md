# etcd-revision-profile

A profiling helper that reports how often each of a PipelineRun's keys has been written and how large it is: how
many revisions and bytes its PipelineRun, TaskRuns, their Pods, and the Events
for all of those have accumulated in etcd, per object and summarised by kind.

Membership is decided by the controller owner reference rather than by labels,
which are mutable: a child whose labels were stripped is still found, an object
carrying this run's labels but owned by something else is not counted, and a
PipelineRun reusing a deleted run's name does not inherit its objects.
Every key is read at the revision the PipelineRun's own read was served at, so
every value comes from the same revision. Afterwards each measured object's UID
and controller owner are confirmed again through the API, listing without a
label selector so a same-name replacement cannot hide by dropping its labels.

CustomRuns, child PipelineRuns, PVCs and Affinity Assistant StatefulSets are
not included. Only objects the API server can still list are profiled, so run
it soon after the PipelineRun finishes: Events expire after an hour by default.

It is an operator tool, not part of the Tekton control plane. See
[docs/developers/etcd-revision-profiling.md](../../docs/developers/etcd-revision-profiling.md)
for the concepts and methodology.

## Usage

Run from a control-plane node (etcd client keys are root-only, so `etcdctl`
runs via `sudo` by default):

```bash
# Whole PipelineRun (PipelineRun + TaskRuns + Pods + Events)
go run ./hack/etcd-revision-profile -n <namespace> -pipelinerun <name>

# A single raw etcd key
go run ./hack/etcd-revision-profile -etcd-key /registry/minions/<node-name>
```

Flags: `-kubectl`, `-etcdctl` (binary paths), `-endpoints`, `-cacert`, `-cert`,
`-key` (etcd client TLS), `-sudo` (default true), `-etcd-prefix` (the
apiserver's `--etcd-prefix`, default `/registry`), `-verify-uid` (default true;
turn off when values are encrypted at rest), `-allow-partial`.

Output on stderr is split by what it means. `note:` lines are things worth
knowing that do not affect the numbers, such as CustomRuns or child PipelineRuns
this run owns but the helper does not profile. `missing:` lines are objects that
could not be measured, and the run exits non-zero unless `-allow-partial` is
passed.

## Design

The pure analysis lives in `profile.go`: etcd key layout (`etcdKeyFor`),
`etcdctl ... -w json` parsing (`parseEtcdGetJSON`), and aggregation
(`aggregate`). It is unit-tested in `profile_test.go` and `wiring_test.go` with
no cluster required.

`main.go` is the thin `kubectl`/`etcdctl` shell. It runs the same commands an
operator would run by hand, which keeps the helper free of client libraries.
The exec calls go through a `commandRunner` seam, so the discovery and
argument-building logic is covered by `main_test.go` with a fake runner.

```bash
go test ./hack/etcd-revision-profile/
```
