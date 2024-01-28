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

package sidecarlogartifacts

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"log"
	"os"
	"path/filepath"
	"time"
)

func fileExists(filename string) (bool, error) {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("error checking for file existence %w", err)
	}
	return !info.IsDir(), nil
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
			time.Sleep(500 * time.Millisecond)
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
		return v1.Artifacts{}, err
	}
	return parseArtifacts(b)
}

type SidecarArtifacts map[string]v1.Artifacts

func GetArtifactsFromSidecarLogs(ctx context.Context, clientset kubernetes.Interface, namespace string, name string, container string, podPhase corev1.PodPhase) (SidecarArtifacts, error) {
	sidecarArtifacts := SidecarArtifacts{}
	if podPhase == corev1.PodPending {
		return sidecarArtifacts, nil
	}
	podLogOpts := corev1.PodLogOptions{Container: container}
	req := clientset.CoreV1().Pods(namespace).GetLogs(name, &podLogOpts)
	stream, err := req.Stream(ctx)
	if err != nil {
		fmt.Println("failed to retrive logs")
		fmt.Println(err)
		return sidecarArtifacts, err
	}
	err = json.NewDecoder(stream).Decode(&sidecarArtifacts)
	if err != nil {
		fmt.Println("failed to decode")
		fmt.Println(err)
		return sidecarArtifacts, err
	}
	fmt.Println(sidecarArtifacts)

	return sidecarArtifacts, nil
}

func LookForArtifacts(names []string, runDir string) (SidecarArtifacts, error) {
	err := waitForStepsToFinish(runDir)
	if err != nil {
		log.Fatal(err)
	}
	artifacts := SidecarArtifacts{}
	for _, name := range names {
		p := filepath.Join(pipeline.StepsDir, name, "artifacts", "provenance.json")
		if _, err := fileExists(p); err != nil {
			return artifacts, err
		}
		subRes, err := extractArtifactsFromFile(p)
		if err != nil {
			return artifacts, err
		}
		artifacts[name] = v1.Artifacts{Inputs: subRes.Inputs, Outputs: subRes.Outputs}

	}
	return artifacts, nil
}
