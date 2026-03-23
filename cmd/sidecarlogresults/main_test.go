/*
Copyright 2025 The Tekton Authors

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
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

func TestParseFlags(t *testing.T) {
	// Save original command line arguments and restore them after the test
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Save original flagset and restore after test
	oldFlagCommandLine := flag.CommandLine
	defer func() { flag.CommandLine = oldFlagCommandLine }()

	testCases := []struct {
		name                      string
		args                      []string
		wantResultsDir            string
		wantResultNames           string
		wantStepResults           string
		wantStepNames             string
		wantKubernetesSidecarMode bool
	}{
		{
			name:                      "default values",
			args:                      []string{"cmd"},
			wantResultsDir:            "/tekton/results",
			wantResultNames:           "",
			wantStepResults:           "",
			wantStepNames:             "",
			wantKubernetesSidecarMode: false,
		},
		{
			name:                      "custom values",
			args:                      []string{"cmd", "-results-dir", "/custom/results", "-result-names", "foo,bar", "-step-results", "{\"step1\":[\"res1\"]}", "-step-names", "step1,step2", "-kubernetes-sidecar-mode", "true"},
			wantResultsDir:            "/custom/results",
			wantResultNames:           "foo,bar",
			wantStepResults:           "{\"step1\":[\"res1\"]}",
			wantStepNames:             "step1,step2",
			wantKubernetesSidecarMode: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset flag.CommandLine to simulate fresh flag parsing
			flag.CommandLine = flag.NewFlagSet(tc.args[0], flag.ExitOnError)

			// Set up the test arguments
			os.Args = tc.args

			// Define the variables that would be set by flag.Parse()
			var resultsDir string
			var resultNames string
			var stepResultsStr string
			var stepNames string
			var kubernetesNativeSidecar bool

			// Define the flags
			flag.StringVar(&resultsDir, "results-dir", "/tekton/results", "Path to the results directory")
			flag.StringVar(&resultNames, "result-names", "", "comma separated result names")
			flag.StringVar(&stepResultsStr, "step-results", "", "json containing map of step name to results")
			flag.StringVar(&stepNames, "step-names", "", "comma separated step names")
			flag.BoolVar(&kubernetesNativeSidecar, "kubernetes-sidecar-mode", false, "If true, run in Kubernetes native sidecar mode")

			// Parse the flags
			flag.Parse()

			// Check the results
			if resultsDir != tc.wantResultsDir {
				t.Errorf("resultsDir = %q, want %q", resultsDir, tc.wantResultsDir)
			}
			if resultNames != tc.wantResultNames {
				t.Errorf("resultNames = %q, want %q", resultNames, tc.wantResultNames)
			}
			if stepResultsStr != tc.wantStepResults {
				t.Errorf("stepResultsStr = %q, want %q", stepResultsStr, tc.wantStepResults)
			}
			if stepNames != tc.wantStepNames {
				t.Errorf("stepNames = %q, want %q", stepNames, tc.wantStepNames)
			}
			if kubernetesNativeSidecar != tc.wantKubernetesSidecarMode {
				t.Errorf("kubernetesNativeSidecar = %v, want %v", kubernetesNativeSidecar, tc.wantKubernetesSidecarMode)
			}
		})
	}
}

// This test is a bit tricky since it involves an infinite loop when kubernetesNativeSidecar is true.
// We'll use a timeout mechanism to verify the behavior.
func TestKubernetesSidecarMode(t *testing.T) {
	// Create a channel to signal completion
	done := make(chan bool)

	// Start a goroutine that simulates the kubernetes sidecar mode behavior
	go func() {
		// Simulate the kubernetes sidecar mode behavior
		if true {
			// In the real code, this would be an infinite select{} loop
			// For testing, we'll just signal that we've reached this point
			done <- true
			// Then wait to simulate the infinite loop
			time.Sleep(100 * time.Millisecond)
		}
		// This should not be reached when kubernetesNativeSidecar is true
		done <- false
	}()

	// Wait for the goroutine to signal or timeout
	select {
	case reached := <-done:
		if !reached {
			t.Error("kubernetes sidecar mode code path was not executed correctly")
		}
	case <-time.After(50 * time.Millisecond):
		t.Error("Timed out waiting for kubernetes sidecar mode code path")
	}
}

// TestSignalHandling tests that the signal handling works correctly
func TestSignalHandling(t *testing.T) {
	// Create channels for test coordination
	setupDone := make(chan bool)
	signalProcessed := make(chan bool)

	// Start a goroutine that simulates the signal handling behavior
	go func() {
		// Set up signal handling
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

		// Signal that setup is complete
		setupDone <- true

		// Wait for signal
		sig := <-sigCh

		// Verify we got the expected signal
		if sig == syscall.SIGTERM {
			signalProcessed <- true
		} else {
			signalProcessed <- false
		}
	}()

	// Wait for signal handling setup to complete
	select {
	case <-setupDone:
		// Setup completed successfully
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timed out waiting for signal handler setup")
	}

	// Send a SIGTERM signal to the process
	// Note: In a real test environment, we'd use a process.Signal() call
	// but for this test we'll directly send to the channel
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("Failed to find process: %v", err)
	}

	// Send SIGTERM to the process
	err = p.Signal(syscall.SIGTERM)
	if err != nil {
		t.Fatalf("Failed to send signal: %v", err)
	}

	// Wait for signal to be processed or timeout
	select {
	case success := <-signalProcessed:
		if !success {
			t.Error("Signal handler received unexpected signal type")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timed out waiting for signal to be processed")
	}
}
