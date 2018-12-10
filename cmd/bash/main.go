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

package main

import (
	"flag"
	"log"
	"os/exec"

	"github.com/knative/build-pipeline/pkg/logging"
)

var (
	args = flag.String("args", "", "space separated arguments for bash")
)

func main() {
	flag.Parse()
	logger, _ := logging.NewLogger("", "bash_command")
	defer logger.Sync()
	cmd := exec.Command("sh", "-c", *args)
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("Error executing command %q with arguments %s", *args, stdoutStderr)
		log.Fatal(err)
	}
	logger.Infof("Successfully executed command %q", *args)
}
