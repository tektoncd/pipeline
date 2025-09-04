//go:build e2e
// +build e2e

/*
 Copyright 2022 The Tekton Authors

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

package test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun"
	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

const (
	scmTokenSecretBase    = "tekton-e2e-scm-token"
	scmTokenSecretKey     = "token"
	scmRemoteTaskPath     = "tasks/remote-task.yaml"
	scmRemoteOrg          = "test-org"
	scmRemoteRepo         = "test-repo"
	scmRemoteBranch       = "main"
	scmRemoteUser         = "tekton-bot"
	scmRemoteUserPassword = "ab_d1234HIJKL"
	// Defined in git-resolver/gitea.yaml's "gitea" StatefulSet, in the env for the "configure-gitea" init container
	scmGiteaAdminPassword = "giteaPassword1234"
	systemNamespace       = "tekton-pipelines"
)

var (
	defaultSvcRE = regexp.MustCompile(`\.default\.svc\.cluster`)

	hubFeatureFlags = requireAllGates(map[string]string{
		"enable-hub-resolver": "true",
		"enable-api-fields":   "beta",
	})

	gitFeatureFlags = requireAllGates(map[string]string{
		"enable-git-resolver": "true",
		"enable-api-fields":   "beta",
	})

	clusterFeatureFlags = requireAllGates(map[string]string{
		"enable-cluster-resolver": "true",
		"enable-api-fields":       "beta",
	})
)

func TestHubResolver(t *testing.T) {
	ctx := context.Background()
	c, namespace := setup(ctx, t, hubFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	prName := helpers.ObjectNameForTest(t)

	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  workspaces:
    - name: output # this workspace name must be declared in the Pipeline
      volumeClaimTemplate:
        spec:
          accessModes:
            - ReadWriteOnce # access mode may affect how you can use this volume in parallel tasks
          resources:
            requests:
              storage: 1Gi
  pipelineSpec:
    workspaces:
      - name: output
    tasks:
      - name: task1
        workspaces:
          - name: output
        taskRef:
          resolver: hub
          params:
          - name: kind
            value: task
          - name: name
            value: git-clone
          - name: version
            value: "0.10"
        params:
          - name: url
            value: https://github.com/tektoncd/pipeline
          - name: deleteExisting
            value: "true"
`, prName, namespace))

	_, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", prName, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunSucceed(prName), "PipelineRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to finish: %s", prName, err)
	}
}

func TestHubResolver_Failure(t *testing.T) {
	ctx := context.Background()
	c, namespace := setup(ctx, t, hubFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	prName := helpers.ObjectNameForTest(t)

	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  workspaces:
    - name: output # this workspace name must be declared in the Pipeline
      volumeClaimTemplate:
        spec:
          accessModes:
            - ReadWriteOnce # access mode may affect how you can use this volume in parallel tasks
          resources:
            requests:
              storage: 1Gi
  pipelineSpec:
    workspaces:
      - name: output
    tasks:
      - name: task1
        workspaces:
          - name: output
        taskRef:
          resolver: hub
          params:
          - name: kind
            value: task
          - name: name
            value: git-clone-this-does-not-exist
          - name: version
            value: "0.7"
        params:
          - name: url
            value: https://github.com/tektoncd/pipeline
          - name: deleteExisting
            value: "true"
`, prName, namespace))

	_, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", prName, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout,
		Chain(
			FailedWithReason(v1.PipelineRunReasonCouldntGetTask.String(), prName),
			FailedWithMessage("fail to fetch Artifact Hub resource: requested resource 'https://artifacthub.io/api/v1/packages/tekton-task/tekton-catalog-tasks/git-clone-this-does-not-exist' not found on hub", prName),
		), "PipelineRunFailed", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun to finish with expected error: %s", err)
	}
}

func TestGitResolver_Clone(t *testing.T) {
	ctx := context.Background()
	c, namespace := setup(ctx, t, gitFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	prName := helpers.ObjectNameForTest(t)

	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  workspaces:
    - name: output # this workspace name must be declared in the Pipeline
      volumeClaimTemplate:
        spec:
          accessModes:
            - ReadWriteOnce # access mode may affect how you can use this volume in parallel tasks
          resources:
            requests:
              storage: 1Gi
  pipelineSpec:
    workspaces:
      - name: output
    tasks:
      - name: task1
        workspaces:
          - name: output
        taskRef:
          resolver: git
          params:
          - name: url
            value: https://github.com/tektoncd/catalog.git
          - name: pathInRepo
            value: /task/git-clone/0.10/git-clone.yaml
          - name: revision
            value: main
        params:
          - name: url
            value: https://github.com/tektoncd/pipeline
          - name: deleteExisting
            value: "true"
`, prName, namespace))

	_, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", prName, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunSucceed(prName), "PipelineRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to finish: %s", prName, err)
	}
}

func TestGitResolver_Clone_Failure(t *testing.T) {
	defaultURL := "https://github.com/tektoncd/catalog.git"
	defaultPathInRepo := "/task/git-clone/0.10/git-clone.yaml"
	defaultCommit := "783b4fe7d21148f3b1a93bfa49b0024d8c6c2955"

	testCases := []struct {
		name        string
		url         string
		pathInRepo  string
		commit      string
		expectedErr string
	}{
		{
			name:        "repo does not exist",
			url:         "https://github.com/tektoncd/catalog-does-not-exist.git",
			expectedErr: "clone error: authentication required",
		}, {
			name:        "path does not exist",
			pathInRepo:  "/task/banana/55.55/banana.yaml",
			expectedErr: "error opening file \"/task/banana/55.55/banana.yaml\": file does not exist",
		}, {
			name:        "commit does not exist",
			commit:      "abcd0123",
			expectedErr: "git fetch error: fatal: couldn't find remote ref abcd0123: exit status 128",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			expectedErr := tc.expectedErr
			url := tc.url
			if url == "" {
				url = defaultURL
			}
			pathInRepo := tc.pathInRepo
			if pathInRepo == "" {
				pathInRepo = defaultPathInRepo
			}
			commit := tc.commit
			if commit == "" {
				commit = defaultCommit
			}

			ctx := context.Background()
			c, namespace := setup(ctx, t, gitFeatureFlags)

			t.Parallel()

			knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
			defer tearDown(ctx, t, c, namespace)

			prName := helpers.ObjectNameForTest(t)

			pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  workspaces:
    - name: output # this workspace name must be declared in the Pipeline
      volumeClaimTemplate:
        spec:
          accessModes:
            - ReadWriteOnce # access mode may affect how you can use this volume in parallel tasks
          resources:
            requests:
              storage: 1Gi
  pipelineSpec:
    workspaces:
      - name: output
    tasks:
      - name: task1
        workspaces:
          - name: output
        taskRef:
          resolver: git
          params:
          - name: url
            value: %s
          - name: pathInRepo
            value: %s
          - name: revision
            value: %s
        params:
          - name: url
            value: https://github.com/tektoncd/pipeline
          - name: deleteExisting
            value: "true"
`, prName, namespace, url, pathInRepo, commit))

			_, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create PipelineRun `%s`: %s", prName, err)
			}

			t.Logf("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
			if err := WaitForPipelineRunState(ctx, c, prName, timeout,
				Chain(
					FailedWithReason(v1.PipelineRunReasonCouldntGetTask.String(), prName),
					FailedWithMessage(expectedErr, prName),
				), "PipelineRunFailed", v1Version); err != nil {
				t.Fatalf("Error waiting for PipelineRun to finish with expected error: %s", err)
			}
		})
	}
}

func TestClusterResolver(t *testing.T) {
	ctx := context.Background()
	c, namespace := setup(ctx, t, clusterFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	pipelineName := helpers.ObjectNameForTest(t)
	examplePipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: some-pipeline-task
    taskSpec:
      steps:
      - name: echo
        image: mirror.gcr.io/ubuntu
        script: |
          #!/usr/bin/env bash
          # Sleep for 10s
          sleep 10
`, pipelineName, namespace))

	_, err := c.V1PipelineClient.Create(ctx, examplePipeline, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipelineName, err)
	}

	prName := helpers.ObjectNameForTest(t)

	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    resolver: cluster
    params:
    - name: kind
      value: pipeline
    - name: name
      value: %s
    - name: namespace
      value: %s
`, prName, namespace, pipelineName, namespace))

	_, err = c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", prName, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunSucceed(prName), "PipelineRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to finish: %s", prName, err)
	}
}

func TestClusterResolver_Failure(t *testing.T) {
	ctx := context.Background()
	c, namespace := setup(ctx, t, clusterFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	prName := helpers.ObjectNameForTest(t)

	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    resolver: cluster
    params:
    - name: kind
      value: pipeline
    - name: name
      value: does-not-exist
    - name: namespace
      value: %s
`, prName, namespace, namespace))

	_, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", prName, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout,
		Chain(
			FailedWithReason(pipelinerun.ReasonCouldntGetPipeline, prName),
			FailedWithMessage("pipelines.tekton.dev \"does-not-exist\" not found", prName),
		), "PipelineRunFailed", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun to finish with expected error: %s", err)
	}
}
