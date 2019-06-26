/*
Copyright 2019 The Tekton Authors.

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

	"github.com/tektoncd/pipeline/pkg/entrypoint"
	"golang.org/x/xerrors"
)

var (
	ep              = flag.String("entrypoint", "", "Original specified entrypoint to execute")
	waitFile        = flag.String("wait_file", "", "If specified, file to wait for")
	waitFileContent = flag.Bool("wait_file_content", false, "If specified, expect wait_file to have content")
	postFile        = flag.String("post_file", "", "If specified, file to write upon completion")

	waitPollingInterval = time.Second
)

func main() {
	flag.Parse()

	e := entrypoint.Entrypointer{
		Entrypoint:      *ep,
		WaitFile:        *waitFile,
		WaitFileContent: *waitFileContent,
		PostFile:        *postFile,
		Args:            flag.Args(),
		Waiter:          &RealWaiter{},
		Runner:          &RealRunner{},
		PostWriter:      &RealPostWriter{},
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

// Wait watches a file and returns when either a) the file exists and, if
// the expectContent argument is true, the file has non-zero size or b) there
// is an error polling the file.
//
// If the passed-in file is an empty string then this function returns
// immediately.
//
// If a file of the same name with a ".err" extension exists then this Wait
// will end with a skipError.
func (*RealWaiter) Wait(file string, expectContent bool) error {
	if file == "" {
		return nil
	}
	for ; ; time.Sleep(waitPollingInterval) {
		if info, err := os.Stat(file); err == nil {
			if !expectContent || info.Size() > 0 {
				return nil
			}
		} else if !os.IsNotExist(err) {
			return xerrors.Errorf("Waiting for %q: %w", file, err)
		}
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
