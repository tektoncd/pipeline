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
	Wait(file string)
}

// Runner encapsulates running commands.
type Runner interface {
	Run(args ...string)
}

// PostWriter encapsulates writing a file when complete.
type PostWriter interface {
	// Write writes to the path when complete.
	Write(file string)
}

// Go optionally waits for a file, runs the command, and writes a
// post file.
func (e Entrypointer) Go() {
	if e.WaitFile != "" {
		e.Waiter.Wait(e.WaitFile)
	}

	if e.Entrypoint != "" {
		e.Args = append([]string{e.Entrypoint}, e.Args...)
	}
	e.Runner.Run(e.Args...)

	if e.PostFile != "" {
		e.PostWriter.Write(e.PostFile)
	}
}
