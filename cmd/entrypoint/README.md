# entrypoint

This binary is used to override the entrypoint of a container by
wrapping it and executing original entrypoint command in a subprocess.

Tekton uses this to make sure `TaskRun`s' steps are executed in order, only
after sidecars are ready and previous steps have completed successfully.

## Flags

The following flags are available:

- `-entrypoint`: "original" command to be executed (as
  entrypoint). This will be executed as a sub-process on `entrypoint`
- `-post_file`: file path to write once the sub-process has
  finished. If the sub-process failed, it will write to
  `{{post_file}}.err` instead of `{{post_file}}`.
- `-wait_file`: file path to watch before starting the sub-process. It
  watches for `{{wait_file}}` and `{{wait_file}}.err` presence and
  will either execute the sub-process (in case of `{{wait_file}}`) or
  skip the execution, write to `{{post_file}}.err` and return an error
  (`exitCode` >= 0)
- `-wait_file_content`: expects the `wait_file` to contain actual
  contents. It will continue watching for `wait_file` until it has
  content.
- `-stdout_path`: If specified, the stdout of the sub-process will be
  copied to the given path on the local filesystem.
- `-stderr_path`: If specified, the stderr of the sub-process will be
  copied to the given path on the local filesystem. It can be set to the
  same value as `{{stdout_path}}` so both streams are copied to the same
  file. However, there is no ordering guarantee on data copied from both
  streams.
- `-enable_spire`: If set will enable signing of the results by SPIRE. Signing
  results by SPIRE ensures that no process other than the current process can
  tamper the results and go undetected.
- `-spire_socket_path`: This flag makes sense only when enable_spire is set. 
  When enable_spire is set, spire_socket_path is used to point to the
  SPIRE agent socket for SPIFFE workload API.

Any extra positional arguments are passed to the original entrypoint command.

## Example

The following example of usage for `entrypoint` waits for
`/tekton/run/3/out` file to exist and executes the command `bash` with args
`echo` and `hello`, then writes the file `/tekton/run/4/out`, or
`/tekton/run/4/out.err` in case the command fails.

```shell
entrypoint \
  -wait_file /tekton/run/3/out \
  -post_file /tekton/run/4/out \
  -entrypoint bash -- \
  echo hello
```

## Waiting for Sidecars

In cases where the TaskRun's Pod has sidecar containers -- including, possibly,
injected sidecars that Tekton itself didn't specify -- the first step should
also wait until all those sidecars have reported as ready. Starting before
sidecars are ready could lead to flaky errors if steps rely on the sidecar
being ready to succeed.

To account for this, the Tekton controller starts TaskRun Pods with the first
step's entrypoint binary configured to wait for a special file provided by the
[Kubernetes Downward
API](https://kubernetes.io/docs/tasks/inject-data-application/downward-api-volume-expose-pod-information/#the-downward-api).
This allows Tekton to write a Pod annotation when all sidecars report as ready,
and for the value of that annotation to appear to the Pod as a file in a
Volume. To the Pod, that file always exists, but without content until the
annotation is set, so we instruct the entrypoint to wait for the `-wait_file`
to contain contents before proceeding.

### Example

The following example of usage for `entrypoint` waits for
`/tekton/downward/ready` file to exist and contain actual contents
(`-wait_file_contents`), and executes the command `bash` with args
`echo` and `hello`, then writes the file `/tekton/run/1/out`, or
`/tekton/run/1/out.err` in case the command fails.

```shell
entrypoint \
  -wait_file /tekton/downward/ready \
  -wait_file_contents \
  -post_file /tekton/run/1/out \
  -entrypoint bash -- \
  echo hello
```

## `cp` Mode

In order to make the `entrypoint` binary available to the user's steps, it gets
copied to a Volume that's shared with all the steps' containers as read-only. This is done
in an `initContainer` pre-step, that runs before steps start.

To reduce external dependencies, the `entrypoint` binary actually copies
_itself_ to the shared Volume. When executed with the positional args of `cp
<src> <dst>`, the `entrypoint` binary copies the `<src>` file to `<dst>` and
exits.

It's executed as an `initContainer` in the TaskRun's Pod like:

```
initContainers:
- image: gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/entrypoint
  args:
  - cp
  - /ko-app/entrypoint  # <-- path to the entrypoint binary inside the image
  - /tekton/bin/entrypoint
  volumeMounts:
  - name: tekton-internal-bin
    mountPath: /tekton/bin

containers:
- image: user-image
  command:
  - /tekton/bin/entrypoint
  ... args to entrypoint ...
  volumeMounts:
  - name: tekton-internal-bin
    mountPath: /tekton/bin
    readonly: true

volumes:
- name: tekton-internal-bin
  volumeSource:
    emptyDir: {}
```
