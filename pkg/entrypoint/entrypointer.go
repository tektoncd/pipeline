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

package entrypoint

import (
	"fmt"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/logging"
	"github.com/tektoncd/pipeline/pkg/termination"
)

// Entrypointer holds fields for running commands with redirected
// entrypoints.
type Entrypointer struct {
	// Entrypoint is the original specified entrypoint, if any.
	Entrypoint string
	// Args are the original specified args, if any.
	Args []string
	// WaitFiles is the set of files to wait for. If empty, execution
	// begins immediately.
	WaitFiles []string
	// WaitFileContent indicates the WaitFile should have non-zero size
	// before continuing with execution.
	WaitFileContent bool
	// PostFile is the file to write when complete. If not specified, no
	// file is written.
	PostFile string

	// Termination path is the path of a file to write the starting time of this endpopint
	TerminationPath string

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
	Wait(file string, expectContent bool) error
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
	logger, _ := logging.NewLogger("", "entrypoint")
	defer func() {
		_ = logger.Sync()
	}()
	output := []v1alpha1.PipelineResourceResult{}
	for _, f := range e.WaitFiles {
		if err := e.Waiter.Wait(f, e.WaitFileContent); err != nil {
			// An error happened while waiting, so we bail
			// *but* we write postfile to make next steps bail too.
			e.WritePostFile(e.PostFile, err)
			return err
		}
	}

	if e.Entrypoint != "" {
		e.Args = append([]string{e.Entrypoint}, e.Args...)
	}
	output = append(output, v1alpha1.PipelineResourceResult{
		Key:   "StartedAt",
		Value: time.Now().Format(time.RFC3339),
	})

	err := e.Runner.Run(e.Args...)

	// Write the post file *no matter what*
	e.WritePostFile(e.PostFile, err)

	if wErr := termination.WriteMessage(e.TerminationPath, output); wErr != nil {
		logger.Fatalf("Error while writing message: %s", wErr)
	}
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
