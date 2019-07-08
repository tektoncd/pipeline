# bash

This tool is for executing `shell` command. To override base shell
image update `.ko.yaml` file. This is currently used in internal
containers, mainly related to `PipelineResource`.

To use it, run
```
image: github.com/tektoncd/pipeline/cmd/bash
args: ['-args', 'ARGUMENTS_FOR_SHELL_COMMAND']
```

For help, run `man sub-command`
```
image: github.com/tektoncd/pipeline/cmd/bash
args: ['-args', `man`,`mkdir`]
```

For example:
Following example executes shell sub-command `mkdir`

```
image: github.com/tektoncd/pipeline/cmd/bash
args:  ['-args', 'mkdir', '-p', '/workspace/gcs-dir']
```
