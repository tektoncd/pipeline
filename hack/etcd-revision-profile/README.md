# etcd-revision-profile

A profiling helper that reports the etcd MVCC footprint of a PipelineRun: how
many revisions and bytes its PipelineRun, child TaskRuns, their Pods, and
associated Events have accumulated in etcd.

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
`-key` (etcd client TLS), `-sudo` (default true).

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
