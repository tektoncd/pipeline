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

package sidecarlogresults

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/result"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// ErrSizeExceeded indicates that the result exceeded its maximum allowed size
var ErrSizeExceeded = errors.New("results size exceeds configured limit")

// SidecarLogResult holds fields for storing extracted results
type SidecarLogResult struct {
	Name  string
	Value string
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

func waitForStepsToFinish(runDir string) error {
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
			if exists, err = fileExists(fmt.Sprintf("%s.err", stepFile)); exists || err != nil {
				return err
			}
		}
	}
	return nil
}

// LookForResults waits for results to be written out by the steps
// in their results path and prints them in a structured way to its
// stdout so that the reconciler can parse those logs.
func LookForResults(w io.Writer, runDir string, resultsDir string, resultNames []string) error {
	if err := waitForStepsToFinish(runDir); err != nil {
		return fmt.Errorf("error while waiting for the steps to finish  %w", err)
	}
	results := make(chan SidecarLogResult)
	g := new(errgroup.Group)
	for _, resultFile := range resultNames {
		resultFile := resultFile

		g.Go(func() error {
			value, err := os.ReadFile(filepath.Join(resultsDir, resultFile))
			if os.IsNotExist(err) {
				return nil
			} else if err != nil {
				return fmt.Errorf("error reading the results file %w", err)
			}
			newResult := SidecarLogResult{
				Name:  resultFile,
				Value: string(value),
			}
			results <- newResult
			return nil
		})
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

// GetResultsFromSidecarLogs extracts results from the logs of the results sidecar
func GetResultsFromSidecarLogs(ctx context.Context, clientset kubernetes.Interface, namespace string, name string, container string, podPhase corev1.PodPhase) ([]result.RunResult, error) {
	sidecarLogResults := []result.RunResult{}
	if podPhase == corev1.PodPending {
		return sidecarLogResults, nil
	}
	podLogOpts := corev1.PodLogOptions{Container: container}
	req := clientset.CoreV1().Pods(namespace).GetLogs(name, &podLogOpts)
	sidecarLogs, err := req.Stream(ctx)
	if err != nil {
		return sidecarLogResults, err
	}
	defer sidecarLogs.Close()
	maxResultLimit := config.FromContextOrDefaults(ctx).FeatureFlags.MaxResultSize
	return extractResultsFromLogs(sidecarLogs, sidecarLogResults, maxResultLimit)
}

func extractResultsFromLogs(logs io.Reader, sidecarLogResults []result.RunResult, maxResultLimit int) ([]result.RunResult, error) {
	scanner := bufio.NewScanner(logs)
	buf := make([]byte, maxResultLimit)
	scanner.Buffer(buf, maxResultLimit)
	for scanner.Scan() {
		result, err := parseResults(scanner.Bytes(), maxResultLimit)
		if err != nil {
			return nil, err
		}
		sidecarLogResults = append(sidecarLogResults, result)
	}

	if scanner.Err() != nil {
		if errors.Is(scanner.Err(), bufio.ErrTooLong) {
			return sidecarLogResults, ErrSizeExceeded
		}
		return nil, scanner.Err()
	}
	return sidecarLogResults, nil
}

func parseResults(resultBytes []byte, maxResultLimit int) (result.RunResult, error) {
	runResult := result.RunResult{}
	if len(resultBytes) > maxResultLimit {
		return runResult, ErrSizeExceeded
	}

	var res SidecarLogResult
	if err := json.Unmarshal(resultBytes, &res); err != nil {
		return runResult, fmt.Errorf("Invalid result %w", err)
	}
	runResult = result.RunResult{
		Key:        res.Name,
		Value:      res.Value,
		ResultType: result.TaskRunResultType,
	}
	return runResult, nil
}
