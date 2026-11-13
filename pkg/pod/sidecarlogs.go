/*
Copyright 2026 The Tekton Authors

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

package pod

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/tektoncd/pipeline/internal/sidecarlogresults"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/result"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// ErrSizeExceeded indicates that the result exceeded its maximum allowed size.
var ErrSizeExceeded = errors.New("results size exceeds configured limit")

// getResultsFromSidecarLogs extracts results from the logs of the results sidecar.
func getResultsFromSidecarLogs(ctx context.Context, clientset kubernetes.Interface, namespace string, name string, container string, podPhase corev1.PodPhase) ([]result.RunResult, error) {
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
	reader := bufio.NewReaderSize(logs, maxResultLimit)
	for {
		line, isPrefix, err := reader.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}

		if isPrefix {
			// Take ownership of the buffer before further ReadLine calls,
			// since ReadLine's returned slice is only valid until the next call.
			line = append([]byte(nil), line...)
		}
		lineLength := len(line)
		for isPrefix {
			more, nextPrefix, err := reader.ReadLine()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return nil, err
			}
			lineLength += len(more)

			if lineLength <= maxResultLimit {
				line = append(line, more...)
			}
			isPrefix = nextPrefix
		}

		if lineLength > maxResultLimit {
			return sidecarLogResults, fmt.Errorf("%d bytes %w of %d bytes", lineLength, ErrSizeExceeded, maxResultLimit)
		}

		result, err := parseResults(line, maxResultLimit)
		if err != nil {
			return nil, err
		}
		sidecarLogResults = append(sidecarLogResults, result)
	}

	return sidecarLogResults, nil
}

func parseResults(resultBytes []byte, maxResultLimit int) (result.RunResult, error) {
	runResult := result.RunResult{}
	var res sidecarlogresults.SidecarLogResult
	if err := json.Unmarshal(resultBytes, &res); err != nil {
		return runResult, fmt.Errorf("invalid result \"%s\": %w", res.Name, err)
	}
	if len(resultBytes) > maxResultLimit {
		return runResult, fmt.Errorf("invalid result \"%s\": %w of %d", res.Name, ErrSizeExceeded, maxResultLimit)
	}
	var resultType result.ResultType
	switch res.Type {
	case sidecarlogresults.TaskResultType:
		resultType = result.TaskRunResultType
	case sidecarlogresults.StepResultType:
		resultType = result.StepResultType
	case sidecarlogresults.StepArtifactType:
		resultType = result.StepArtifactsResultType
	case sidecarlogresults.TaskArtifactType:
		resultType = result.TaskRunArtifactsResultType
	default:
		return result.RunResult{}, fmt.Errorf("invalid sidecar result type %v. Must be %v or %v or %v", res.Type, sidecarlogresults.TaskResultType, sidecarlogresults.StepResultType, sidecarlogresults.StepArtifactType)
	}
	runResult = result.RunResult{
		Key:        res.Name,
		Value:      res.Value,
		ResultType: resultType,
	}
	return runResult, nil
}

// extractStepAndResultFromSidecarResultName splits the result name to extract the step
// and result name from it. It only works if the format is <stepName>.<resultName>.
func extractStepAndResultFromSidecarResultName(sidecarResultName string) (string, string, error) {
	splitString := strings.SplitN(sidecarResultName, ".", 2)
	if len(splitString) != 2 {
		return "", "", fmt.Errorf("invalid string %s : expected somtthing that looks like <stepName>.<resultName>", sidecarResultName)
	}
	return splitString[0], splitString[1], nil
}
