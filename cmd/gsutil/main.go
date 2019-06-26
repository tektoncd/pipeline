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
This tool is for executing `gsutil` and `gcloud` command
To override gsutil base image update `.ko.yaml` file.

`gcloud auth` is used to authenticate with private buckets

To use it, run
```
image: github.com/tektoncd/pipeline/cmd/gsutil
args: ['-args', 'ARGUMENTS_FOR_GSUTIL_COMMAND']

```

For help, run
```
image: github.com/tektoncd/pipeline/cmd/gsutil
args: ['-args','--help']
```

For example:
Following example executes gsutil sub-command `cp`

```
image: github.com/tektoncd/pipeline/cmd/gsutil
args: ['-args', 'cp', 'gs://fake-bucket/rules.zip', '/workspace/gcs-dir'
env:
  - name: GOOGLE_APPLICATION_CREDENTIALS
    value: /var/secret/auth/key.json
```
*/

package main

import (
	"flag"
	"os"
	"os/exec"
	"strings"

	"github.com/tektoncd/pipeline/pkg/logging"
)

var (
	args = flag.String("args", "", "space separated arguments for gsutil")
)

func main() {
	flag.Parse()
	logger, _ := logging.NewLogger("", "gsutil")
	defer logger.Sync()

	authFilePath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if authFilePath != "" {
		_, err := os.Stat(authFilePath)
		if os.IsNotExist(err) {
			logger.Fatalf("File does not exist at path %s; got err %s", authFilePath, err)
		}

		if _, err := exec.LookPath("gcloud"); err != nil {
			logger.Fatalf("Error executing command 'gcloud'; err %s", err)
		}

		if authCmdstdoutStderr, err := exec.Command("gcloud", "auth", "activate-service-account", "--key-file", authFilePath).CombinedOutput(); err != nil {
			logger.Fatalf("Error executing command %s; cmd output: %s", err, authCmdstdoutStderr)
		}
		logger.Info("Successfully authenticated with gcloud")
	}

	if _, err := exec.LookPath("gsutil"); err != nil {
		logger.Fatalf("Error executing command 'gsutil'; err %s", err)
	}

	cmd := exec.Command("gsutil")
	cmd.Args = append(cmd.Args, strings.Split(*args, " ")...)

	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		logger.Fatalf("Error executing command %q ; error %s; cmd_output %s", strings.Join(cmd.Args, " "), err.Error(), stdoutStderr)
	}
	logger.Infof("Successfully executed command %q; output %s", strings.Join(cmd.Args, " "), stdoutStderr)
}
