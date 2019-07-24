/*
Copyright 2019 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
This tool is for executing `shell` command. To override base shell image update `.ko.yaml` file.

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
*/

package main

import (
	"flag"
	"os/exec"
	"strings"

	"github.com/tektoncd/pipeline/pkg/logging"
)

var (
	args = flag.String("args", "", "space separated arguments for shell")
)

func main() {
	flag.Parse()
	logger, _ := logging.NewLogger("", "shell_command")
	defer logger.Sync()
	cmd := exec.Command("sh", "-c", *args)
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		logger.Fatalf("Error executing command %q ; error %s; cmd_output %s", strings.Join(cmd.Args, " "), err.Error(), stdoutStderr)
	}
	logger.Infof("Successfully executed command %q; output %s", strings.Join(cmd.Args, " "), stdoutStderr)
}
