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
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/pod"
	"github.com/tektoncd/pipeline/pkg/result"
	"github.com/tektoncd/pipeline/pkg/spire"
	"github.com/tektoncd/pipeline/pkg/termination"
	"go.uber.org/zap"
)

// RFC3339 with millisecond
const (
	timeFormat      = "2006-01-02T15:04:05.000Z07:00"
	ContinueOnError = "continue"
	FailOnError     = "stopAndFail"
)

// ContextError context error type
type ContextError string

// Error implements error interface
func (e ContextError) Error() string {
	return string(e)
}

type SkipError string

func (e SkipError) Error() string {
	return string(e)
}

var (
	// ErrContextDeadlineExceeded is the error returned when the context deadline is exceeded
	ErrContextDeadlineExceeded = ContextError(context.DeadlineExceeded.Error())
	// ErrContextCanceled is the error returned when the context is canceled
	ErrContextCanceled = ContextError(context.Canceled.Error())
	// ErrSkipPreviousStepFailed is the error returned when the step is skipped due to previous step error
	ErrSkipPreviousStepFailed = SkipError("error file present, bail and skip the step")
)

// IsContextDeadlineError determine whether the error is context deadline
func IsContextDeadlineError(err error) bool {
	return errors.Is(err, ErrContextDeadlineExceeded)
}

// IsContextCanceledError determine whether the error is context canceled
func IsContextCanceledError(err error) bool {
	return errors.Is(err, ErrContextCanceled)
}

// Entrypointer holds fields for running commands with redirected
// entrypoints.
type Entrypointer struct {
	// Command is the original specified command and args.
	Command []string

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

	// StepResults is the set of files that might contain step results
	StepResults []string
	// Results is the set of files that might contain task results
	Results []string
	// Timeout is an optional user-specified duration within which the Step must complete
	Timeout *time.Duration
	// BreakpointOnFailure helps determine if entrypoint execution needs to adapt debugging requirements
	BreakpointOnFailure bool
	// OnError defines exiting behavior of the entrypoint
	// set it to "stopAndFail" to indicate the entrypoint to exit the taskRun if the container exits with non zero exit code
	// set it to "continue" to indicate the entrypoint to continue executing the rest of the steps irrespective of the container exit code
	OnError string
	// StepMetadataDir is the directory for a step where the step related metadata can be stored
	StepMetadataDir string
	// SpireWorkloadAPI connects to spire and does obtains SVID based on taskrun
	SpireWorkloadAPI spire.EntrypointerAPIClient
	// ResultsDirectory is the directory to find results, defaults to pipeline.DefaultResultPath
	ResultsDirectory string
	// ResultExtractionMethod is the method using which the controller extracts the results from the task pod.
	ResultExtractionMethod string
}

// Waiter encapsulates waiting for files to exist.
type Waiter interface {
	// Wait blocks until the specified file exists or the context is done.
	Wait(ctx context.Context, file string, expectContent bool, breakpointOnFailure bool) error
}

// Runner encapsulates running commands.
type Runner interface {
	Run(ctx context.Context, args ...string) error
}

// PostWriter encapsulates writing a file when complete.
type PostWriter interface {
	// Write writes to the path when complete.
	Write(file, content string)
}

// Go optionally waits for a file, runs the command, and writes a
// post file.
func (e Entrypointer) Go() error {
	prod, _ := zap.NewProduction()
	logger := prod.Sugar()

	output := []result.RunResult{}
	defer func() {
		if wErr := termination.WriteMessage(e.TerminationPath, output); wErr != nil {
			logger.Fatalf("Error while writing message: %s", wErr)
		}
		_ = logger.Sync()
	}()

	if err := os.MkdirAll(filepath.Join(e.StepMetadataDir, "results"), os.ModePerm); err != nil {
		return err
	}
	for _, f := range e.WaitFiles {
		if err := e.Waiter.Wait(context.Background(), f, e.WaitFileContent, e.BreakpointOnFailure); err != nil {
			// An error happened while waiting, so we bail
			// *but* we write postfile to make next steps bail too.
			// In case of breakpoint on failure do not write post file.
			if !e.BreakpointOnFailure {
				e.WritePostFile(e.PostFile, err)
			}
			output = append(output, result.RunResult{
				Key:        "StartedAt",
				Value:      time.Now().Format(timeFormat),
				ResultType: result.InternalTektonResultType,
			})

			if errors.Is(err, ErrSkipPreviousStepFailed) {
				output = append(output, e.outputRunResult(pod.TerminationReasonSkipped))
			}

			return err
		}
	}

	output = append(output, result.RunResult{
		Key:        "StartedAt",
		Value:      time.Now().Format(timeFormat),
		ResultType: result.InternalTektonResultType,
	})

	var err error
	if e.Timeout != nil && *e.Timeout < time.Duration(0) {
		err = fmt.Errorf("negative timeout specified")
	}
	ctx := context.Background()
	var cancel context.CancelFunc
	if err == nil {
		ctx, cancel = context.WithCancel(ctx)
		if e.Timeout != nil && *e.Timeout > time.Duration(0) {
			ctx, cancel = context.WithTimeout(ctx, *e.Timeout)
		}
		defer cancel()
		// start a goroutine to listen for cancellation file
		go func() {
			if err := e.waitingCancellation(ctx, cancel); err != nil && (!IsContextCanceledError(err) && !IsContextDeadlineError(err)) {
				logger.Error("Error while waiting for cancellation", zap.Error(err))
			}
		}()
		err = e.Runner.Run(ctx, e.Command...)
	}

	var ee *exec.ExitError
	switch {
	case err != nil && errors.Is(err, ErrContextCanceled):
		logger.Info("Step was canceling")
		output = append(output, e.outputRunResult(pod.TerminationReasonCancelled))
		e.WritePostFile(e.PostFile, ErrContextCanceled)
		e.WriteExitCodeFile(e.StepMetadataDir, syscall.SIGKILL.String())
	case errors.Is(err, ErrContextDeadlineExceeded):
		e.WritePostFile(e.PostFile, err)
		output = append(output, e.outputRunResult(pod.TerminationReasonTimeoutExceeded))
	case err != nil && e.BreakpointOnFailure:
		logger.Info("Skipping writing to PostFile")
	case e.OnError == ContinueOnError && errors.As(err, &ee):
		// with continue on error and an ExitError, write non-zero exit code and a post file
		exitCode := strconv.Itoa(ee.ExitCode())
		output = append(output, result.RunResult{
			Key:        "ExitCode",
			Value:      exitCode,
			ResultType: result.InternalTektonResultType,
		})
		e.WritePostFile(e.PostFile, nil)
		e.WriteExitCodeFile(e.StepMetadataDir, exitCode)
	case err == nil:
		// if err is nil, write zero exit code and a post file
		e.WritePostFile(e.PostFile, nil)
		e.WriteExitCodeFile(e.StepMetadataDir, "0")
	default:
		// for a step without continue on error and any error, write a post file with .err
		e.WritePostFile(e.PostFile, err)
	}

	// strings.Split(..) with an empty string returns an array that contains one element, an empty string.
	// This creates an error when trying to open the result folder as a file.
	if len(e.Results) >= 1 && e.Results[0] != "" {
		resultPath := pipeline.DefaultResultPath
		if e.ResultsDirectory != "" {
			resultPath = e.ResultsDirectory
		}
		if err := e.readResultsFromDisk(ctx, resultPath, result.TaskRunResultType); err != nil {
			logger.Fatalf("Error while handling results: %s", err)
		}
	}
	if len(e.StepResults) >= 1 && e.StepResults[0] != "" {
		stepResultPath := filepath.Join(e.StepMetadataDir, "results")
		if e.ResultsDirectory != "" {
			stepResultPath = e.ResultsDirectory
		}
		if err := e.readResultsFromDisk(ctx, stepResultPath, result.StepResultType); err != nil {
			logger.Fatalf("Error while handling step results: %s", err)
		}
	}

	return err
}

func (e Entrypointer) readResultsFromDisk(ctx context.Context, resultDir string, resultType result.ResultType) error {
	output := []result.RunResult{}
	results := e.Results
	if resultType == result.StepResultType {
		results = e.StepResults
	}
	for _, resultFile := range results {
		if resultFile == "" {
			continue
		}
		fileContents, err := os.ReadFile(filepath.Join(resultDir, resultFile))
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		// if the file doesn't exist, ignore it
		output = append(output, result.RunResult{
			Key:        resultFile,
			Value:      string(fileContents),
			ResultType: resultType,
		})
	}

	if e.SpireWorkloadAPI != nil {
		signed, err := e.SpireWorkloadAPI.Sign(ctx, output)
		if err != nil {
			return err
		}
		output = append(output, signed...)
	}

	// push output to termination path
	if e.ResultExtractionMethod == config.ResultExtractionMethodTerminationMessage && len(output) != 0 {
		if err := termination.WriteMessage(e.TerminationPath, output); err != nil {
			return err
		}
	}
	return nil
}

// BreakpointExitCode reads the post file and returns the exit code it contains
func (e Entrypointer) BreakpointExitCode(breakpointExitPostFile string) (int, error) {
	exitCode, err := os.ReadFile(breakpointExitPostFile)
	if os.IsNotExist(err) {
		return 0, fmt.Errorf("breakpoint postfile %s not found", breakpointExitPostFile)
	}
	strExitCode := strings.TrimSuffix(string(exitCode), "\n")
	log.Println("Breakpoint exiting with exit code " + strExitCode)

	return strconv.Atoi(strExitCode)
}

// WritePostFile write the postfile
func (e Entrypointer) WritePostFile(postFile string, err error) {
	if err != nil && postFile != "" {
		postFile = fmt.Sprintf("%s.err", postFile)
	}
	if postFile != "" {
		e.PostWriter.Write(postFile, "")
	}
}

// WriteExitCodeFile write the exitCodeFile
func (e Entrypointer) WriteExitCodeFile(stepPath, content string) {
	exitCodeFile := filepath.Join(stepPath, "exitCode")
	e.PostWriter.Write(exitCodeFile, content)
}

// waitingCancellation waiting cancellation file, if no error occurs, call cancelFunc to cancel the context
func (e Entrypointer) waitingCancellation(ctx context.Context, cancel context.CancelFunc) error {
	if err := e.Waiter.Wait(ctx, pod.DownwardMountCancelFile, true, false); err != nil {
		return err
	}
	cancel()
	return nil
}

// outputRunResult returns the run reason for a termination
func (e Entrypointer) outputRunResult(terminationReason string) result.RunResult {
	return result.RunResult{
		Key:        "Reason",
		Value:      terminationReason,
		ResultType: result.InternalTektonResultType,
	}
}
