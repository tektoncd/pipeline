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

package entrypoint

import (
	"fmt"
)

// Entrypointer holds fields for running commands with redirected
// entrypoints.
type Entrypointer struct {
	// Entrypoint is the original specified entrypoint, if any.
	Entrypoint string
	// Args are the original specified args, if any.
	Args []string
	// WaitFile is the file to wait for. If not specified, execution begins
	// immediately.
	WaitFile string
	// PostFile is the file to write when complete. If not specified, no
	// file is written.
	PostFile string

	// Waiter encapsulates waiting for files to exist.
	Waiter Waiter
	// Runner encapsulates running commands.
	Runner Runner
	// PostWriter encapsulates writing files when complete.
	PostWriter PostWriter
}

// Waiter encapsulates waiting for files to exist.
type Waiter interface {
	// Wait blocks until the specified file exists.
	Wait(file string) error
}

// Runner encapsulates running commands.
type Runner interface {
	Run(args ...string) error
}

// PostWriter encapsulates writing a file when complete.
type PostWriter interface {
	// Write writes to the path when complete.
	Write(file string)
}

// Go optionally waits for a file, runs the command, and writes a
// post file.
func (e Entrypointer) Go() error {
	if e.WaitFile != "" {
		if err := e.Waiter.Wait(e.WaitFile); err != nil {
			// An error happened while waiting, so we bail
			// *but* we write postfile to make next steps bail too
			e.WritePostFile(e.PostFile, err)
			return err
		}
	}

	if e.Entrypoint != "" {
		e.Args = append([]string{e.Entrypoint}, e.Args...)
	}

	err := e.Runner.Run(e.Args...)

	// Write the post file *no matter what*
	e.WritePostFile(e.PostFile, err)

	return err
}

func (e Entrypointer) WritePostFile(postFile string, err error) {
	if err != nil && postFile != "" {
		postFile = fmt.Sprintf("%s.err", postFile)
	}
	if postFile != "" {
		e.PostWriter.Write(postFile)
	}
}
