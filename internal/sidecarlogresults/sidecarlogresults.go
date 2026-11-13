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

// Package sidecarlogresults emits results from files without depending on the
// controller's Kubernetes log reader in pkg/pod.
package sidecarlogresults

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/entrypoint/pipeline"
	"golang.org/x/sync/errgroup"
)

var stepDir = pipeline.StepsDir

type SidecarLogResultType string

const (
	// TaskResultType identifies task results in sidecar logs.
	TaskResultType SidecarLogResultType = "task"
	// StepResultType identifies step results in sidecar logs.
	StepResultType SidecarLogResultType = "step"
	// StepArtifactType identifies step artifacts in sidecar logs.
	StepArtifactType SidecarLogResultType = "stepArtifact"
	// TaskArtifactType identifies task artifacts in sidecar logs.
	TaskArtifactType           SidecarLogResultType = "taskArtifact"
	sidecarResultNameSeparator string               = "."
)

// SidecarLogResult holds fields for storing extracted results
type SidecarLogResult struct {
	Name  string               `json:"name"`
	Value string               `json:"value"`
	Type  SidecarLogResultType `json:"type"`
}

func fileExists(filename string) (bool, error) {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("error checking for file existence %w", err)
	}
	return !info.IsDir(), nil
}

func encode(w io.Writer, v any) error {
	return json.NewEncoder(w).Encode(v)
}

func waitForStepsToFinish(runDir string, sleepInterval time.Duration) error {
	steps := make(map[string]bool)
	files, err := os.ReadDir(runDir)
	if err != nil {
		return fmt.Errorf("error parsing the run dir  %w", err)
	}
	for _, file := range files {
		steps[filepath.Join(runDir, file.Name(), "out")] = true
	}
	for len(steps) > 0 {
		for stepFile := range steps {
			// check if there is a post file without error
			exists, err := fileExists(stepFile)
			if err != nil {
				return fmt.Errorf("error checking for out file's existence %w", err)
			}
			if exists {
				delete(steps, stepFile)
				continue
			}
			// check if there is a post file with error
			// if err is nil then either the out.err file does not exist or it does and there was no issue
			// in either case, existence of out.err marks that the step errored and the following steps will
			// not run. We want the function to break out with nil error in that case so that
			// the existing results can be logged.
			if exists, err = fileExists(stepFile + ".err"); exists || err != nil {
				return err
			}
		}
		if sleepInterval > 0 {
			time.Sleep(sleepInterval)
		}
	}
	return nil
}

func createSidecarResultName(stepName, resultName string) string {
	return fmt.Sprintf("%s%s%s", stepName, sidecarResultNameSeparator, resultName)
}

func readResults(resultsDir, resultFile, stepName string, resultType SidecarLogResultType) (SidecarLogResult, error) {
	value, err := os.ReadFile(filepath.Join(resultsDir, resultFile))
	if os.IsNotExist(err) {
		return SidecarLogResult{}, nil
	} else if err != nil {
		return SidecarLogResult{}, fmt.Errorf("error reading the results file %w", err)
	}
	resultName := resultFile
	if resultType == StepResultType {
		resultName = createSidecarResultName(stepName, resultFile)
	}
	return SidecarLogResult{
		Name:  resultName,
		Value: string(value),
		Type:  resultType,
	}, nil
}

// LookForResults waits for results to be written out by the steps
// in their results path and prints them in a structured way to its
// stdout so that the reconciler can parse those logs.
func LookForResults(w io.Writer, runDir string, resultsDir string, resultNames []string, stepResultsDir string, stepResults map[string][]string) error {
	interval, err := getSidecarLogPollingInterval()
	if err != nil {
		return fmt.Errorf("error getting polling interval: %w", err)
	}
	if err := waitForStepsToFinish(runDir, interval); err != nil {
		return fmt.Errorf("error while waiting for the steps to finish  %w", err)
	}
	results := make(chan SidecarLogResult)
	g := new(errgroup.Group)
	for _, resultFile := range resultNames {
		g.Go(func() error {
			newResult, err := readResults(resultsDir, resultFile, "", TaskResultType)
			if err != nil {
				return err
			}
			if newResult.Name == "" {
				return nil
			}
			results <- newResult
			return nil
		})
	}

	for sName, sresults := range stepResults {
		for _, resultName := range sresults {
			stepResultsDir := filepath.Join(stepResultsDir, sName, "results")

			g.Go(func() error {
				newResult, err := readResults(stepResultsDir, resultName, sName, StepResultType)
				if err != nil {
					return err
				}
				if newResult.Name == "" {
					return nil
				}
				results <- newResult
				return nil
			})
		}
	}

	channelGroup := new(errgroup.Group)
	channelGroup.Go(func() error {
		if err := g.Wait(); err != nil {
			return fmt.Errorf("error parsing results: %w", err)
		}
		close(results)
		return nil
	})

	for result := range results {
		if err := encode(w, result); err != nil {
			return fmt.Errorf("error writing results: %w", err)
		}
	}
	if err := channelGroup.Wait(); err != nil {
		return err
	}
	return nil
}

// LookForArtifacts searches for and processes artifacts within a specified run directory.
// It looks for "provenance.json" files within the "artifacts" subdirectory of each named step.
// If the provenance file exists, the function extracts artifact information, formats it into a
// JSON string, and encodes it for output alongside relevant metadata (step name, artifact type).
func LookForArtifacts(w io.Writer, names []string, runDir string) error {
	interval, err := getSidecarLogPollingInterval()
	if err != nil {
		return fmt.Errorf("error getting polling interval: %w", err)
	}
	if err := waitForStepsToFinish(runDir, interval); err != nil {
		return err
	}

	for _, name := range names {
		p := filepath.Join(stepDir, name, "artifacts", "provenance.json")
		if exist, err := fileExists(p); err != nil {
			return err
		} else if !exist {
			continue
		}
		subRes, err := extractArtifactsFromFile(p)
		if err != nil {
			return err
		}
		values, err := json.Marshal(&subRes)
		if err != nil {
			return err
		}
		if err := encode(w, SidecarLogResult{Name: name, Value: string(values), Type: StepArtifactType}); err != nil {
			return err
		}
	}
	return nil
}

func parseArtifacts(fileContent []byte) (v1.Artifacts, error) {
	var as v1.Artifacts
	if err := json.Unmarshal(fileContent, &as); err != nil {
		return as, fmt.Errorf("invalid artifacts : %w", err)
	}
	return as, nil
}

func extractArtifactsFromFile(filename string) (v1.Artifacts, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return v1.Artifacts{}, fmt.Errorf("error reading the results file %w", err)
	}
	return parseArtifacts(b)
}

// getSidecarLogPollingInterval reads the SIDECAR_LOG_POLLING_INTERVAL environment variable,
// parses it as a time.Duration, and returns the result. If the variable is not set or is invalid,
// it defaults to 100ms.
func getSidecarLogPollingInterval() (time.Duration, error) {
	intervalStr := os.Getenv("SIDECAR_LOG_POLLING_INTERVAL")
	if intervalStr == "" {
		intervalStr = "100ms"
	}
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		return 100 * time.Millisecond, err
	}
	return interval, nil
}
