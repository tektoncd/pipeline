<!--
---
linkTitle: "Tekton Controllers flags"
weight: 105
---
-->

# Tekton Controllers flags

The different controllers `tektoncd/pipeline` ships come with a set of flags
that can be changed (in the `yaml` payloads) for advanced use cases. This page
documents them.

## Flag availability overview

Not all flags are available on every controller. The table below shows which
flags and environment variables are supported by each controller binary.

### Flags from `knative/pkg` (available on all controllers)

These flags are registered by
[`injection.ParseAndGetRESTConfigOrDie()`](https://github.com/knative/pkg/tree/main/injection)
and [`klog`](https://github.com/kubernetes/klog), and are available on **all**
controllers (`controller`, `webhook`, `events`, `resolvers`):

| Flag | Description |
|------|-------------|
| `-cluster` | Defaults to the current cluster in kubeconfig. |
| `-server` | The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster. |
| `-kubeconfig` | Path to a kubeconfig. Only required if out-of-cluster. |
| `-kube-api-burst` | Maximum burst for throttle. |
| `-kube-api-qps` | Maximum QPS to the server from the client. |
| `-v` | Number for the log level verbosity. |
| `-vmodule` | Comma-separated list of `pattern=N` settings for file-filtered logging. |
| `-add_dir_header` | If true, adds the file directory to the header of the log messages. |
| `-alsologtostderr` | Log to standard error as well as files (no effect when `-logtostderr=true`). |
| `-log_backtrace_at` | When logging hits line `file:N`, emit a stack trace. |
| `-log_dir` | If non-empty, write log files in this directory (no effect when `-logtostderr=true`). |
| `-log_file` | If non-empty, use this log file (no effect when `-logtostderr=true`). |
| `-log_file_max_size` | Maximum size a log file can grow to in megabytes (default 1800, 0 = unlimited; no effect when `-logtostderr=true`). |
| `-logtostderr` | Log to standard error instead of files (default true). |
| `-one_output` | If true, only write logs to their native severity level (no effect when `-logtostderr=true`). |
| `-skip_headers` | If true, avoid header prefixes in the log messages. |
| `-skip_log_headers` | If true, avoid headers when opening log files (no effect when `-logtostderr=true`). |
| `-stderrthreshold` | Logs at or above this threshold go to stderr (default 2; no effect when `-logtostderr=true` or `-alsologtostderr=false`). |

### Flags and environment variables that vary by controller

The following flags and environment variables are **not** available on all
controllers. Their availability depends on whether the controller calls
[`sharedmain.MainWithContext()`](https://github.com/knative/pkg/tree/main/injection/sharedmain)
(which registers `--disable-ha` and reads `K_THREADS_PER_CONTROLLER`) or
registers flags directly in its own `main()`.

| Flag / Environment Variable | Source | `controller` | `webhook` | `events` | `resolvers` |
|------------------------------|--------|:---:|:---:|:---:|:---:|
| `--disable-ha` | `sharedmain` / Tekton | Yes | Yes | Yes | No |
| `--threads-per-controller` | Tekton | Yes | No | No | Yes |
| `--namespace` | Tekton | Yes | No | No | No |
| `--resync-period` | Tekton | Yes | No | No | No |
| `THREADS_PER_CONTROLLER` env var | Tekton | Yes | No | No | Yes |
| `K_THREADS_PER_CONTROLLER` env var | `sharedmain` | No | Yes | Yes | No |

> **Note:** `controller` and `resolvers` read the `THREADS_PER_CONTROLLER`
> environment variable (without the `K_` prefix) in their own code before
> parsing flags, while `webhook` and `events` read `K_THREADS_PER_CONTROLLER`
> (with the `K_` prefix) via `sharedmain.MainWithContext()`. Both environment
> variables set the same underlying
> `controller.DefaultThreadsPerController` value; the difference is only in the
> variable name.

## `controller`

The main controller binary
([`cmd/controller/main.go`](https://github.com/tektoncd/pipeline/blob/main/cmd/controller/main.go))
has additional flags to configure its behavior.

```
  -disable-ha
        Whether to disable high-availability functionality for this component.
        This flag will be deprecated and removed when we have promoted this
        feature to stable, so do not pass it without filing an issue upstream!
  -entrypoint-image string
        The container image containing our entrypoint binary.
  -namespace string
        Namespace to restrict informer to. Optional, defaults to all namespaces.
  -nop-image string
        The container image used to stop sidecars
  -resync-period duration
        The period between two resync run (going through all objects) (default 10h0m0s)
  -shell-image string
        The container image containing a shell
  -shell-image-win string
        The container image containing a windows shell
  -sidecarlogresults-image string
        The container image containing the binary for accessing results.
  -threads-per-controller int
        Threads (goroutines) to create per controller (default 2)
  -workingdirinit-image string
        The container image containing our working dir init binary.
```

**Environment variables:** `THREADS_PER_CONTROLLER`

## `webhook`

The webhook binary
([`cmd/webhook/main.go`](https://github.com/tektoncd/pipeline/blob/main/cmd/webhook/main.go))
calls `sharedmain.MainWithContext()`, which provides:

```
  -disable-ha
        Whether to disable high-availability functionality for this component.
        This flag will be deprecated and removed when we have promoted this
        feature to stable, so do not pass it without filing an issue upstream!
```

**Environment variables:** `K_THREADS_PER_CONTROLLER`

## `events`

The events controller binary
([`cmd/events/main.go`](https://github.com/tektoncd/pipeline/blob/main/cmd/events/main.go))
calls `sharedmain.Main()`, which provides:

```
  -disable-ha
        Whether to disable high-availability functionality for this component.
        This flag will be deprecated and removed when we have promoted this
        feature to stable, so do not pass it without filing an issue upstream!
```

**Environment variables:** `K_THREADS_PER_CONTROLLER`

## `resolvers`

The resolvers binary
([`cmd/resolvers/main.go`](https://github.com/tektoncd/pipeline/blob/main/cmd/resolvers/main.go))
has the following additional flag:

```
  -threads-per-controller int
        Threads (goroutines) to create per controller (default 2)
```

**Environment variables:** `THREADS_PER_CONTROLLER`

> **Note:** The `resolvers` binary does **not** support `--disable-ha`. It
> calls `injection.ParseAndGetRESTConfigOrDie()` followed by
> `sharedmain.MainWithConfig()`, which does not register the `--disable-ha`
> flag.
