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
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/knative/build-pipeline/pkg/entrypoint"
)

var (
	ep       = flag.String("entrypoint", "", "Original specified entrypoint to execute")
	waitFile = flag.String("wait_file", "", "If specified, file to wait for")
	postFile = flag.String("post_file", "", "If specified, file to write upon completion")
)

func main() {
	flag.Parse()

	entrypoint.Entrypointer{
		Entrypoint: *ep,
		WaitFile:   *waitFile,
		PostFile:   *postFile,
		Args:       flag.Args(),
		Waiter:     &RealWaiter{},
		Runner:     &RealRunner{},
		PostWriter: &RealPostWriter{},
	}.Go()
}

// TODO(jasonhall): Test that original exit code is propagated and that
// stdout/stderr are collected -- needs e2e tests.

// RealWaiter actually waits for files, by polling.
type RealWaiter struct{ waitFile string }

var _ entrypoint.Waiter = (*RealWaiter)(nil)

func (*RealWaiter) Wait(file string) {
	if file == "" {
		return
	}
	for ; ; time.Sleep(time.Second) {
		if _, err := os.Stat(file); err == nil {
			return
		} else if !os.IsNotExist(err) {
			log.Fatalf("Waiting for %q: %v", file, err)
		}
	}
}

// RealRunner actually runs commands.
type RealRunner struct{}

var _ entrypoint.Runner = (*RealRunner)(nil)

func (*RealRunner) Run(args ...string) {
	if len(args) == 0 {
		return
	}
	name, args := args[0], args[1:]

	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			// Copied from https://stackoverflow.com/questions/10385551/get-exit-code-go
			// This works on both Unix and Windows. Although
			// package syscall is generally platform dependent,
			// WaitStatus is defined for both Unix and Windows and
			// in both cases has an ExitStatus() method with the
			// same signature.
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
			log.Fatalf("Error executing command (ExitError): %v", err)
		}
		log.Fatalf("Error executing command: %v", err)
	}
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
