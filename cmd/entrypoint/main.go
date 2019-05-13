/*
Copyright 2019 The Knative Authors.

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
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/tektoncd/pipeline/pkg/entrypoint"
)

var (
	ep       = flag.String("entrypoint", "", "Original specified entrypoint to execute")
	waitFile = flag.String("wait_file", "", "If specified, file to wait for")
	postFile = flag.String("post_file", "", "If specified, file to write upon completion")
)

func main() {
	flag.Parse()

	e := entrypoint.Entrypointer{
		Entrypoint: *ep,
		WaitFile:   *waitFile,
		PostFile:   *postFile,
		Args:       flag.Args(),
		Waiter:     &RealWaiter{},
		Runner:     &RealRunner{},
		PostWriter: &RealPostWriter{},
	}
	if err := e.Go(); err != nil {
		switch err.(type) {
		case skipError:
			os.Exit(0)
		case *exec.ExitError:
			// Copied from https://stackoverflow.com/questions/10385551/get-exit-code-go
			// This works on both Unix and Windows. Although
			// package syscall is generally platform dependent,
			// WaitStatus is defined for both Unix and Windows and
			// in both cases has an ExitStatus() method with the
			// same signature.
			if status, ok := err.(*exec.ExitError).Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
			log.Fatalf("Error executing command (ExitError): %v", err)
		default:
			log.Fatalf("Error executing command: %v", err)
		}
	}
}

// TODO(jasonhall): Test that original exit code is propagated and that
// stdout/stderr are collected -- needs e2e tests.

// RealWaiter actually waits for files, by polling.
type RealWaiter struct{}

var _ entrypoint.Waiter = (*RealWaiter)(nil)

func (*RealWaiter) Wait(file string) error {
	if file == "" {
		return nil
	}
	for ; ; time.Sleep(time.Second) {
		// Watch for the post file
		if _, err := os.Stat(file); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("Waiting for %q: %v", file, err)
		}
		// Watch for the post error file
		if _, err := os.Stat(file + ".err"); err == nil {
			return skipError("error file present, bail and skip the step")
		}
	}
}

// RealRunner actually runs commands.
type RealRunner struct{}

var _ entrypoint.Runner = (*RealRunner)(nil)

func (*RealRunner) Run(args ...string) error {
	if len(args) == 0 {
		return nil
	}
	name, args := args[0], args[1:]

	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// RealPostWriter actually writes files.
type RealPostWriter struct{}

var _ entrypoint.PostWriter = (*RealPostWriter)(nil)

func (*RealPostWriter) Write(file string) {
	if file == "" {
		return
	}
	if _, err := os.Create(file); err != nil {
		log.Fatalf("Creating %q: %v", file, err)
	}
}

type skipError string

func (e skipError) Error() string {
	return string(e)
}
