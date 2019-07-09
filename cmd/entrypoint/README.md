# entrypoint

This binary is used to override the entrypoint of a container by
wrapping it. In `tektoncd/pipeline` this is used to make sure `Task`'s
steps are executed in order, or for sidecars.

The following flags are available :
- `-entrypoint`: "original" command to be executed (as
  entrypoint). This will be executed as a sub-process on `entrypoint`
- `-post_file`: file path to write once the sub-process has
  finished. If the sub-process failed, it will write to
  `{{post_file}}.err` instead of `{{post_file}}`.
- `-wait_file`: file path to watch before starting the sub-process. It
  watches for `{{wait_file}}` and `{{wait_file}}.err` precense and
  will either execute the sub-process (in case of `{{wait_file}}`) or
  skip the execution, write to `{{post_file}}.err` and return an error
  (`exitCode` >= 0)
- `-wait_file_content`: excepts the `wait_file` to add actual
  content. It will continue watching for `wait_file` until it has
  content.

The following example of usage for `entrypoint`, wait's for
`/builder/downward/ready` file to exists and have some content before
executing `/ko-app/bash -- -args mkdir -p /workspace/git-resource`,
and will write to `/builder/tools/0` in casse of succes, or
`/builder/tools/0.err` in case of failure.

```
entrypoint \
	-wait_file /builder/downward/ready \
	-post_file /builder/tools/0" \
	-wait_file_content  \
	-entrypoint /ko-app/bash -- -args mkdir -p /workspace/git-resource
```
