<!--
---
linkTitle: "Tekton Controllers flags"
weight: 105
---
-->

# Tekton Controllers flags

The different controllers `tektoncd/pipeline` ships comes with a set of flags
that can be changed (in the `yaml` payloads) for advanced use cases. This page
is documenting them.

## Common set of flags

The following flags are available on all "controllers", aka `controller`, `webhook`, `events` and `resolvers`.

```
  -add_dir_header
        If true, adds the file directory to the header of the log messages
  -alsologtostderr
        log to standard error as well as files (no effect when -logtostderr=true)
  -cluster string
        Defaults to the current cluster in kubeconfig.
  -disable-ha
        Whether to disable high-availability functionality for this component.  This flag will be deprecated and removed when we have promoted this feature to stable, so do not pass it without filing an issue upstream!
  -kube-api-burst int
        Maximum burst for throttle.
  -kube-api-qps float
        Maximum QPS to the server from the client.
  -kubeconfig string
        Path to a kubeconfig. Only required if out-of-cluster.
  -log_backtrace_at value
        when logging hits line file:N, emit a stack trace
  -log_dir string
        If non-empty, write log files in this directory (no effect when -logtostderr=true)
  -log_file string
        If non-empty, use this log file (no effect when -logtostderr=true)
  -log_file_max_size uint
        Defines the maximum size a log file can grow to (no effect when -logtostderr=true). Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
  -logtostderr
        log to standard error instead of files (default true)
  -one_output
        If true, only write logs to their native severity level (vs also writing to each lower severity level; no effect when -logtostderr=true)
  -server string
        The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.
  -skip_headers
        If true, avoid header prefixes in the log messages
  -skip_log_headers
        If true, avoid headers when opening log files (no effect when -logtostderr=true)
  -stderrthreshold value
        logs at or above this threshold go to stderr when writing to files and stderr (no effect when -logtostderr=true or -alsologtostderr=false) (default 2)
  -v value
        number for the log level verbosity
  -vmodule value
        comma-separated list of pattern=N settings for file-filtered logging
```

## `controller`

The main controller binary has additional flags to configure its behavior.

```
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
