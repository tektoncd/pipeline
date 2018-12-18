/*
Copyright 2018 The Knative Authors

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
This tool is for executing `gsutil` command
gsutil command is prefixed with `-m` flag(default) for faster download and uploads.
To override gsutil base image update `.ko.yaml` file.

To use it, run
```
image: github.com/build-pipeline/cmd/gsutil
args: ['-args', 'ARGUMENTS_FOR_GSUTIL_COMMAND']
```

For help, run
```
image: github.com/build-pipeline/cmd/gsutil
args: ['-args','--help']
```

For example:
Following example executes gsutil sub-command `cp`

```
image: github.com/build-pipeline/cmd/gsutil
args: ['-args', 'cp', 'gs://fake-bucket/rules.zip', '/workspace/gcs-dir'
```
*/

package main

import (
	"flag"
	"os/exec"
	"strings"

	"github.com/knative/build-pipeline/pkg/logging"
)

var (
	args = flag.String("args", "", "space separated arguments for gsutil")
)

func main() {
	flag.Parse()
	logger, _ := logging.NewLogger("", "gsutil")
	defer logger.Sync()
	gsutilPath, checkgsutilErr := exec.LookPath("gsutil")
	if checkgsutilErr != nil {
		logger.Fatalf("Error executing command 'gsutil verson'; err %s; output %s", checkgsutilErr.Error(), gsutilPath)
	}

	logger.Infof("gsutil binary at path %s", gsutilPath)

	cmd := exec.Command("gsutil", "-m")
	cmd.Args = append(cmd.Args, strings.Split(*args, " ")...)

	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		logger.Fatalf("Error executing command %q ; error %s; cmd_output %s", strings.Join(cmd.Args, " "), err.Error(), stdoutStderr)
	}
	logger.Infof("Successfully executed command %q; output %s", strings.Join(cmd.Args, " "), stdoutStderr)
}
