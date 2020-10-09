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
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/tektoncd/pipeline/pkg/credentials"
	"github.com/tektoncd/pipeline/pkg/credentials/dockercreds"
	"github.com/tektoncd/pipeline/pkg/credentials/gitcreds"
	"github.com/tektoncd/pipeline/pkg/entrypoint"
	"github.com/tektoncd/pipeline/pkg/termination"
)

var (
	ep                  = flag.String("entrypoint", "", "Original specified entrypoint to execute")
	waitFiles           = flag.String("wait_file", "", "Comma-separated list of paths to wait for")
	waitFileContent     = flag.Bool("wait_file_content", false, "If specified, expect wait_file to have content")
	postFile            = flag.String("post_file", "", "If specified, file to write upon completion")
	terminationPath     = flag.String("termination_path", "/tekton/termination", "If specified, file to write upon termination")
	results             = flag.String("results", "", "If specified, list of file names that might contain task results")
	waitPollingInterval = time.Second
	timeout             = flag.Duration("timeout", time.Duration(0), "If specified, sets timeout for step")
)

func cp(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	// Owner has permission to write and execute, and anybody has
	// permission to execute.
	d, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE, 0311)
	if err != nil {
		return err
	}
	defer d.Close()

	_, err = io.Copy(d, s)
	return err
}

func main() {
	// Add credential flags originally used in creds-init.
	gitcreds.AddFlags(flag.CommandLine)
	dockercreds.AddFlags(flag.CommandLine)

	flag.Parse()

	// If invoked in "cp mode" (`entrypoint cp <src> <dst>`), simply copy
	// the src path to the dst path. This is used to place the entrypoint
	// binary in the tools directory, without requiring the cp command to
	// exist in the base image.
	if len(flag.Args()) == 3 && flag.Args()[0] == "cp" {
		src, dst := flag.Args()[1], flag.Args()[2]
		if err := cp(src, dst); err != nil {
			log.Fatal(err)
		}
		log.Println("Copied", src, "to", dst)
		return
	}

	// Copy creds-init credentials from secret volume mounts to /tekton/creds
	// This is done to support the expansion of a variable, $(credentials.path), that
	// resolves to a single place with all the stored credentials.
	builders := []credentials.Builder{dockercreds.NewBuilder(), gitcreds.NewBuilder()}
	for _, c := range builders {
		if err := c.Write("/tekton/creds"); err != nil {
			log.Printf("Error initializing credentials: %s", err)
		}
	}

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
		Timeout:         timeout,
	}

	// Copy any creds injected by the controller into the $HOME directory of the current
	// user so that they're discoverable by git / ssh.
	if err := credentials.CopyCredsToHome(credentials.CredsInitCredentials); err != nil {
		log.Printf("non-fatal error copying credentials: %q", err)
	}

	if err := e.Go(); err != nil {
		switch t := err.(type) {
		case skipError:
			log.Print("Skipping step because a previous step failed")
			os.Exit(1)
		case termination.MessageLengthError:
			log.Print(err.Error())
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
