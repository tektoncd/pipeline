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

package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/tektoncd/pipeline/internal/sidecarlogresults"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/pod"
)

func main() {
	var resultsDir string
	var resultNames string
	var stepResultsStr string
	var stepNames string
	var kubernetesNativeSidecar bool

	flag.StringVar(&resultsDir, "results-dir", pipeline.DefaultResultPath, "Path to the results directory. Default is /tekton/results")
	flag.StringVar(&resultNames, "result-names", "", "comma separated result names to expect from the steps running in the pod. eg. foo,bar,baz")
	flag.StringVar(&stepResultsStr, "step-results", "", "json containing a map of step Name as key and list of result Names. eg. {\"stepName\":[\"foo\",\"bar\",\"baz\"]}")
	flag.StringVar(&stepNames, "step-names", "", "comma separated step names. eg. foo,bar,baz")
	flag.BoolVar(&kubernetesNativeSidecar, "kubernetes-sidecar-mode", false, "If true, wait indefinitely after processing results (for Kubernetes native sidecar support)")
	flag.Parse()

	var done chan bool
	// If kubernetesNativeSidecar is true, wait indefinitely to prevent container from exiting
	// This is needed for Kubernetes native sidecar support
	if kubernetesNativeSidecar {
		// Set up signal handling for graceful shutdown
		// Create a channel to receive OS signals.
		sigCh := make(chan os.Signal, 1)

		// Register the channel to receive notifications for specific signals.
		// In this case, we are interested in SIGINT (Ctrl+C) and SIGTERM.
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

		// Create a channel to signal that the program should exit gracefully.
		done = make(chan bool, 1)

		// Start a goroutine to handle incoming signals.
		go func() {
			<-sigCh      // Block until a signal is received.
			done <- true // Signal that cleanup is done and the program can exit.
		}()
	}

	var expectedResults []string
	// strings.Split returns [""] instead of [] for empty string, we don't want pass [""] to other methods.
	if len(resultNames) > 0 {
		expectedResults = strings.Split(resultNames, ",")
	}
	expectedStepResults := map[string][]string{}
	if err := json.Unmarshal([]byte(stepResultsStr), &expectedStepResults); err != nil {
		log.Fatal(err)
	}
	err := sidecarlogresults.LookForResults(os.Stdout, pod.RunDir, resultsDir, expectedResults, pipeline.StepsDir, expectedStepResults)
	if err != nil {
		log.Fatal(err)
	}

	var names []string
	if len(stepNames) > 0 {
		names = strings.Split(stepNames, ",")
	}
	err = sidecarlogresults.LookForArtifacts(os.Stdout, names, pod.RunDir)
	if err != nil {
		log.Fatal(err)
	}

	if kubernetesNativeSidecar && done != nil {
		// Wait for a signal to be received.
		<-done
	}
}
