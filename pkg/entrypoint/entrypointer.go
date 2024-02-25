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
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tektoncd/pipeline/internal/artifactref"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/internal/resultref"
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

// ScriptDir for testing
var ScriptDir = pipeline.ScriptDir

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
	if err := os.MkdirAll(filepath.Join(e.StepMetadataDir, "artifacts"), os.ModePerm); err != nil {
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
		err = errors.New("negative timeout specified")
	}
	ctx := context.Background()
	var cancel context.CancelFunc
	if err == nil {
		if err := e.applyStepResultSubstitutions(pipeline.StepsDir); err != nil {
			logger.Error("Error while substituting step results: ", err)
		}
		if err := e.applyStepArtifactSubstitutions(pipeline.StepsDir); err != nil {
			logger.Error("Error while substituting step artifacts: ", err)
		}

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

	if e.ResultExtractionMethod == config.ResultExtractionMethodTerminationMessage {
		fp := filepath.Join(e.StepMetadataDir, "artifacts", "provenance.json")

		artifacts, err := readArtifacts(fp)
		if err != nil {
			logger.Fatalf("Error while handling artifacts: %s", err)
		}
		output = append(output, artifacts...)
	}

	return err
}

func readArtifacts(fp string) ([]result.RunResult, error) {
	file, err := os.ReadFile(fp)
	if os.IsNotExist(err) {
		return []result.RunResult{}, nil
	}
	if err != nil {
		return nil, err
	}
	return []result.RunResult{{Key: fp, Value: string(file), ResultType: result.StepArtifactsResultType}}, nil
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
		postFile += ".err"
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

// loadStepResult reads the step result file and returns the string, array or object result value.
func loadStepResult(stepDir string, stepName string, resultName string) (v1.ResultValue, error) {
	v := v1.ResultValue{}
	fp := getStepResultPath(stepDir, pod.GetContainerName(stepName), resultName)
	fileContents, err := os.ReadFile(fp)
	if err != nil {
		return v, err
	}
	err = v.UnmarshalJSON(fileContents)
	if err != nil {
		return v, err
	}
	return v, nil
}

// getStepResultPath gets the path to the step result
func getStepResultPath(stepDir string, stepName string, resultName string) string {
	return filepath.Join(stepDir, stepName, "results", resultName)
}

// findReplacement looks for any usage of step results in an input string.
// If found, it loads the results from the previous steps and provides the replacement value.
func findReplacement(stepDir string, s string) (string, []string, error) {
	value := strings.TrimSuffix(strings.TrimPrefix(s, "$("), ")")
	pr, err := resultref.ParseStepExpression(value)
	if err != nil {
		return "", nil, err
	}
	result, err := loadStepResult(stepDir, pr.ResourceName, pr.ResultName)
	if err != nil {
		return "", nil, err
	}
	replaceWithArray := []string{}
	replaceWithString := ""

	switch pr.ResultType {
	case "object":
		if pr.ObjectKey != "" {
			replaceWithString = result.ObjectVal[pr.ObjectKey]
		}
	case "array":
		if pr.ArrayIdx != nil {
			replaceWithString = result.ArrayVal[*pr.ArrayIdx]
		} else {
			replaceWithArray = append(replaceWithArray, result.ArrayVal...)
		}
	// "string"
	default:
		replaceWithString = result.StringVal
	}
	return replaceWithString, replaceWithArray, nil
}

// replaceEnv performs replacements for step results in environment variables.
func replaceEnv(stepDir string) error {
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		matches := resultref.StepResultRegex.FindAllStringSubmatch(pair[1], -1)
		v := pair[1]
		for _, m := range matches {
			replaceWith, _, err := findReplacement(stepDir, m[0])
			if err != nil {
				return err
			}
			v = strings.ReplaceAll(v, m[0], replaceWith)
		}
		os.Setenv(pair[0], v)
	}
	return nil
}

// replaceCommandAndArgs performs replacements for step results in e.Command
func replaceCommandAndArgs(command []string, stepDir string) ([]string, error) {
	var newCommand []string
	for _, c := range command {
		matches := resultref.StepResultRegex.FindAllStringSubmatch(c, -1)
		newC := []string{c}
		for _, m := range matches {
			replaceWithString, replaceWithArray, err := findReplacement(stepDir, m[0])
			if err != nil {
				return []string{}, fmt.Errorf("failed to find replacement for %s to replace %s", m[0], c)
			}
			// replaceWithString and replaceWithArray are mutually exclusive
			if len(replaceWithArray) > 0 {
				if c != m[0] {
					// it has to be exact in "$(steps.<step-name>.results.<result-name>[*])" format, without anything else in the original string
					return nil, errors.New("value must be in \"$(steps.<step-name>.results.<result-name>[*])\" format, when using array results")
				}
				newC = replaceWithArray
			} else {
				newC[0] = strings.ReplaceAll(newC[0], m[0], replaceWithString)
			}
		}
		newCommand = append(newCommand, newC...)
	}
	return newCommand, nil
}

// applyStepResultSubstitutions applies the runtime step result substitutions in env, args and command.
func (e *Entrypointer) applyStepResultSubstitutions(stepDir string) error {
	// env
	if err := replaceEnv(stepDir); err != nil {
		return err
	}
	// command + args
	newCommand, err := replaceCommandAndArgs(e.Command, stepDir)
	if err != nil {
		return err
	}
	e.Command = newCommand
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

// getStepArtifactsPath gets the path to the step artifacts
func getStepArtifactsPath(stepDir string, containerName string) string {
	return filepath.Join(stepDir, containerName, "artifacts", "provenance.json")
}

// loadStepArtifacts loads and parses the artifacts file for a specified step.
func loadStepArtifacts(stepDir string, containerName string) (v1.Artifacts, error) {
	v := v1.Artifacts{}
	fp := getStepArtifactsPath(stepDir, containerName)

	fileContents, err := os.ReadFile(fp)
	if err != nil {
		return v, err
	}
	err = json.Unmarshal(fileContents, &v)
	if err != nil {
		return v, err
	}
	return v, nil
}

// getArtifactValues retrieves the values associated with a specified artifact reference.
// It parses the provided artifact template, loads the corresponding step's artifacts, and extracts the relevant values.
// If the artifact name is not specified in the template, the values of the first output are returned.
func getArtifactValues(dir string, template string) (string, error) {
	artifactTemplate, err := parseArtifactTemplate(template)

	if err != nil {
		return "", err
	}

	artifacts, err := loadStepArtifacts(dir, artifactTemplate.ContainerName)
	if err != nil {
		return "", err
	}

	// $(steps.stepName.outputs.artifactName) <- artifacts.Output[artifactName].Values
	// $(steps.stepName.outputs) <- artifacts.Output[0].Values
	var t []v1.Artifact
	if artifactTemplate.Type == "outputs" {
		t = artifacts.Outputs
	} else {
		t = artifacts.Inputs
	}

	if artifactTemplate.ArtifactName == "" {
		marshal, err := json.Marshal(t[0].Values)
		if err != nil {
			return "", err
		}
		return string(marshal), err
	}
	for _, ar := range t {
		if ar.Name == artifactTemplate.ArtifactName {
			marshal, err := json.Marshal(ar.Values)
			if err != nil {
				return "", err
			}
			return string(marshal), err
		}
	}
	return "", fmt.Errorf("values for template %s not found", template)
}

// parseArtifactTemplate parses an artifact template string and extracts relevant information into an ArtifactTemplate struct.
//
// The artifact template is expected to be in the format "$(steps.{step-name}.outputs.{artifact-name})" or "$(steps.{step-name}.outputs)".
func parseArtifactTemplate(template string) (ArtifactTemplate, error) {
	if template == "" {
		return ArtifactTemplate{}, errors.New("template is empty")
	}
	if artifactref.StepArtifactRegex.FindString(template) != template {
		return ArtifactTemplate{}, fmt.Errorf("invalid artifact template %s", template)
	}
	template = strings.TrimSuffix(strings.TrimPrefix(template, "$("), ")")
	split := strings.Split(template, ".")
	at := ArtifactTemplate{
		ContainerName: "step-" + split[1],
		Type:          split[2],
	}
	if len(split) == 4 {
		at.ArtifactName = split[3]
	}
	return at, nil
}

// ArtifactTemplate holds steps artifacts metadata parsed from step artifacts interpolation
type ArtifactTemplate struct {
	ContainerName string
	Type          string // inputs or outputs
	ArtifactName  string
}

// applyStepArtifactSubstitutions replaces artifact references within a step's command and environment variables with their corresponding values.
//
// This function is designed to handle artifact substitutions in a script file, inline command, or environment variables.
//
// Args:
//
//	stepDir: The directory of the executing step.
//
// Returns:
//
//	An error object if any issues occur during substitution.
func (e *Entrypointer) applyStepArtifactSubstitutions(stepDir string) error {
	// Script was re-written into a file, we need to read the file to and substitute the content
	// and re-write the command.
	// While param substitution cannot be used in Script from StepAction, allowing artifact substitution doesn't seem bad as
	// artifacts are unmarshalled, should be safe.
	if len(e.Command) == 1 && filepath.Dir(e.Command[0]) == filepath.Clean(ScriptDir) {
		dataBytes, err := os.ReadFile(e.Command[0])
		if err != nil {
			return err
		}
		fileContent := string(dataBytes)
		v, err := replaceValue(artifactref.StepArtifactRegex, fileContent, stepDir, getArtifactValues)
		if err != nil {
			return err
		}
		if v != fileContent {
			temp, err := writeToTempFile(v)
			if err != nil {
				return err
			}
			e.Command = []string{temp.Name()}
		}
	} else {
		command := e.Command
		var newCmd []string
		for _, c := range command {
			v, err := replaceValue(artifactref.StepArtifactRegex, c, stepDir, getArtifactValues)
			if err != nil {
				return err
			}
			newCmd = append(newCmd, v)
		}
		e.Command = newCmd
	}

	// substitute env
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		v, err := replaceValue(artifactref.StepArtifactRegex, pair[1], stepDir, getArtifactValues)

		if err != nil {
			return err
		}
		os.Setenv(pair[0], v)
	}

	return nil
}

func writeToTempFile(v string) (*os.File, error) {
	tmp, err := os.CreateTemp("", "script-*")
	if err != nil {
		return nil, err
	}
	err = os.Chmod(tmp.Name(), 0o755)
	if err != nil {
		return nil, err
	}
	_, err = tmp.WriteString(v)
	if err != nil {
		return nil, err
	}
	err = tmp.Close()
	if err != nil {
		return nil, err
	}
	return tmp, nil
}

func replaceValue(regex *regexp.Regexp, src string, stepDir string, getValue func(string, string) (string, error)) (string, error) {
	matches := regex.FindAllStringSubmatch(src, -1)
	t := src
	for _, m := range matches {
		v, err := getValue(stepDir, m[0])
		if err != nil {
			return "", err
		}
		t = strings.ReplaceAll(t, m[0], v)
	}
	return t, nil
}
