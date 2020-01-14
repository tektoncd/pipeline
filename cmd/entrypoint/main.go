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

package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/entrypoint"
)

var (
	ep                  = flag.String("entrypoint", "", "Original specified entrypoint to execute")
	waitFiles           = flag.String("wait_file", "", "Comma-separated list of paths to wait for")
	waitFileContent     = flag.Bool("wait_file_content", false, "If specified, expect wait_file to have content")
	postFile            = flag.String("post_file", "", "If specified, file to write upon completion")
	terminationPath     = flag.String("termination_path", "/tekton/termination", "If specified, file to write upon termination")
	results             = flag.String("results", "", "If specified, list of file names that might contain task results")
	waitPollingInterval = time.Second
)

func main() {
	flag.Parse()

	e := entrypoint.Entrypointer{
		Entrypoint:      *ep,
		WaitFiles:       strings.Split(*waitFiles, ","),
		WaitFileContent: *waitFileContent,
		PostFile:        *postFile,
		TerminationPath: *terminationPath,
		Args:            flag.Args(),
		Waiter:          &realWaiter{},
		Runner:          &realRunner{},
		PostWriter:      &realPostWriter{},
		Results:         strings.Split(*results, ","),
	}
	// strings.Split(..) with an empty string returns an array that contains one element, an empty string.
	// The result folder should only be created if there are actual results to defined for the entrypoint.
	if len(e.Results) >= 1 && e.Results[0] != "" {
		if err := os.MkdirAll(pipeline.DefaultResultPath, 0755); err != nil {
			log.Fatalf("Error creating the results directory: %v", err)
		}
	}
	if err := e.Go(); err != nil {
		switch t := err.(type) {
		case skipError:
			log.Print("Skipping step because a previous step failed")
			os.Exit(1)
		case *exec.ExitError:
			// Copied from https://stackoverflow.com/questions/10385551/get-exit-code-go
			// This works on both Unix and Windows. Although
			// package syscall is generally platform dependent,
			// WaitStatus is defined for both Unix and Windows and
			// in both cases has an ExitStatus() method with the
			// same signature.
			if status, ok := t.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
			log.Fatalf("Error executing command (ExitError): %v", err)
		default:
			log.Fatalf("Error executing command: %v", err)
		}
	}
}
