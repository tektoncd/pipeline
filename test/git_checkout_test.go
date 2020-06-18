// +build e2e

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

package test

import (
	"strings"
	"testing"

	tb "github.com/tektoncd/pipeline/internal/builder/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resources "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

const (
	gitSourceResourceName  = "git-source-resource"
	gitTestTaskName        = "git-check-task"
	gitTestPipelineName    = "git-check-pipeline"
	gitTestPipelineRunName = "git-check-pipeline-run"
)

// TestGitPipelineRun is an integration test that will verify the source code is either fetched or pulled
// successfully under different revision inputs (branch, commitid, tag, ref)
func TestGitPipelineRun(t *testing.T) {
	t.Parallel()

	revisions := []string{"master", "c15aced0e5aaee6456fbe6f7a7e95e0b5b3b2b2f", "release-0.1", "v0.1.0", "refs/pull/347/head"}

	for _, revision := range revisions {

		t.Run(revision, func(t *testing.T) {
			c, namespace := setup(t)
			knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
			defer tearDown(t, c, namespace)

			t.Logf("Creating Git PipelineResource %s", gitSourceResourceName)
			if _, err := c.PipelineResourceClient.Create(getGitPipelineResource(revision, "", "true", "", "", "")); err != nil {
				t.Fatalf("Failed to create Pipeline Resource `%s`: %s", gitSourceResourceName, err)
			}

			t.Logf("Creating Task %s", gitTestTaskName)
			if _, err := c.TaskClient.Create(getGitCheckTask(namespace)); err != nil {
				t.Fatalf("Failed to create Task `%s`: %s", gitTestTaskName, err)
			}

			t.Logf("Creating Pipeline %s", gitTestPipelineName)
			if _, err := c.PipelineClient.Create(getGitCheckPipeline(namespace)); err != nil {
				t.Fatalf("Failed to create Pipeline `%s`: %s", gitTestPipelineName, err)
			}

			t.Logf("Creating PipelineRun %s", gitTestPipelineRunName)
			if _, err := c.PipelineRunClient.Create(getGitCheckPipelineRun(namespace)); err != nil {
				t.Fatalf("Failed to create Pipeline `%s`: %s", gitTestPipelineRunName, err)
			}

			if err := WaitForPipelineRunState(c, gitTestPipelineRunName, timeout, PipelineRunSucceed(gitTestPipelineRunName), "PipelineRunCompleted"); err != nil {
				t.Errorf("Error waiting for PipelineRun %s to finish: %s", gitTestPipelineRunName, err)
				t.Fatalf("PipelineRun execution failed")
			}
		})
	}
}

// Test revision fetching with refspec specified
func TestGitPipelineRunWithRefspec(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		description string
		revision    string
		refspec     string
	}{{
		description: "Fetch refs/tags/v0.1.0 alongside master and checkout the master branch",
		revision:    "master",
		refspec:     "refs/tags/v0.1.0:refs/tags/v0.1.0 refs/heads/master:refs/heads/master",
	}, {
		description: "Checkout specific revision from refs/pull/1009/head's commit chain",
		revision:    "968d5d37a61bfb85426c885dc1090c1cc4b33436",
		refspec:     "refs/pull/1009/head",
	}, {
		description: "Fetch refs/pull/1009/head into a named master branch and then check it out",
		revision:    "master",
		refspec:     "refs/pull/1009/head:refs/heads/master",
	}} {
		t.Run(tc.description, func(t *testing.T) {
			c, namespace := setup(t)
			knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
			defer tearDown(t, c, namespace)

			if _, err := c.PipelineResourceClient.Create(getGitPipelineResource(tc.revision, tc.refspec, "true", "", "", "")); err != nil {
				t.Fatalf("Failed to create Pipeline Resource `%s`: %s", gitSourceResourceName, err)
			}

			if _, err := c.TaskClient.Create(getGitCheckTask(namespace)); err != nil {
				t.Fatalf("Failed to create Task `%s`: %s", gitTestTaskName, err)
			}

			if _, err := c.PipelineClient.Create(getGitCheckPipeline(namespace)); err != nil {
				t.Fatalf("Failed to create Pipeline `%s`: %s", gitTestPipelineName, err)
			}

			if _, err := c.PipelineRunClient.Create(getGitCheckPipelineRun(namespace)); err != nil {
				t.Fatalf("Failed to create Pipeline `%s`: %s", gitTestPipelineRunName, err)
			}

			if err := WaitForPipelineRunState(c, gitTestPipelineRunName, timeout, PipelineRunSucceed(gitTestPipelineRunName), "PipelineRunCompleted"); err != nil {
				t.Errorf("Error waiting for PipelineRun %s to finish: %s", gitTestPipelineRunName, err)
				t.Fatalf("PipelineRun execution failed")
			}

		})
	}
}

// TestGitPipelineRun_Disable_SSLVerify will verify the source code is retrieved even after disabling SSL certificates (sslVerify)
func TestGitPipelineRun_Disable_SSLVerify(t *testing.T) {
	t.Parallel()

	c, namespace := setup(t)
	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	t.Logf("Creating Git PipelineResource %s", gitSourceResourceName)
	if _, err := c.PipelineResourceClient.Create(getGitPipelineResource("master", "", "false", "", "", "")); err != nil {
		t.Fatalf("Failed to create Pipeline Resource `%s`: %s", gitSourceResourceName, err)
	}

	t.Logf("Creating Task %s", gitTestTaskName)
	if _, err := c.TaskClient.Create(getGitCheckTask(namespace)); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", gitTestTaskName, err)
	}

	t.Logf("Creating Pipeline %s", gitTestPipelineName)
	if _, err := c.PipelineClient.Create(getGitCheckPipeline(namespace)); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", gitTestPipelineName, err)
	}

	t.Logf("Creating PipelineRun %s", gitTestPipelineRunName)
	if _, err := c.PipelineRunClient.Create(getGitCheckPipelineRun(namespace)); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", gitTestPipelineRunName, err)
	}

	if err := WaitForPipelineRunState(c, gitTestPipelineRunName, timeout, PipelineRunSucceed(gitTestPipelineRunName), "PipelineRunCompleted"); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", gitTestPipelineRunName, err)
		t.Fatalf("PipelineRun execution failed")
	}
}

// TestGitPipelineRunFail is a test to ensure that the code extraction from github fails as expected when
// an invalid revision is passed on the pipelineresource.
func TestGitPipelineRunFail(t *testing.T) {
	t.Parallel()

	c, namespace := setup(t)
	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	t.Logf("Creating Git PipelineResource %s", gitSourceResourceName)
	if _, err := c.PipelineResourceClient.Create(getGitPipelineResource("Idontexistrabbitmonkeydonkey", "", "true", "", "", "")); err != nil {
		t.Fatalf("Failed to create Pipeline Resource `%s`: %s", gitSourceResourceName, err)
	}

	t.Logf("Creating Task %s", gitTestTaskName)
	if _, err := c.TaskClient.Create(getGitCheckTask(namespace)); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", gitTestTaskName, err)
	}

	t.Logf("Creating Pipeline %s", gitTestPipelineName)
	if _, err := c.PipelineClient.Create(getGitCheckPipeline(namespace)); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", gitTestPipelineName, err)
	}

	t.Logf("Creating PipelineRun %s", gitTestPipelineRunName)
	if _, err := c.PipelineRunClient.Create(getGitCheckPipelineRun(namespace)); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", gitTestPipelineRunName, err)
	}

	if err := WaitForPipelineRunState(c, gitTestPipelineRunName, timeout, PipelineRunSucceed(gitTestPipelineRunName), "PipelineRunCompleted"); err != nil {
		taskruns, err := c.TaskRunClient.List(metav1.ListOptions{})
		if err != nil {
			t.Errorf("Error getting TaskRun list for PipelineRun %s %s", gitTestPipelineRunName, err)
		}
		for _, tr := range taskruns.Items {
			if tr.Status.PodName != "" {
				p, err := c.KubeClient.Kube.CoreV1().Pods(namespace).Get(tr.Status.PodName, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("Error getting pod `%s` in namespace `%s`", tr.Status.PodName, namespace)
				}

				for _, stat := range p.Status.ContainerStatuses {
					if strings.HasPrefix(stat.Name, "step-git-source-"+gitSourceResourceName) {
						if stat.State.Terminated != nil {
							req := c.KubeClient.Kube.CoreV1().Pods(namespace).GetLogs(p.Name, &corev1.PodLogOptions{Container: stat.Name})
							logContent, err := req.Do().Raw()
							if err != nil {
								t.Fatalf("Error getting pod logs for pod `%s` and container `%s` in namespace `%s`", tr.Status.PodName, stat.Name, namespace)
							}
							// Check for failure messages from fetch and pull in the log file
							if strings.Contains(strings.ToLower(string(logContent)), "couldn't find remote ref idontexistrabbitmonkeydonkey") {
								t.Logf("Found exepected errors when retrieving non-existent git revision")
							} else {
								t.Logf("Container `%s` log File: %s", stat.Name, logContent)
								t.Fatalf("The git code extraction did not fail as expected.  Expected errors not found in log file.")
							}
						}
					}
				}
			}
		}

	} else {
		t.Fatalf("PipelineRun succeeded when should have failed")
	}
}

// TestGitPipelineRunFail_HTTPS_PROXY is a test to ensure that the code extraction from github fails as expected when
// an invalid HTTPS_PROXY is passed on the pipelineresource.
func TestGitPipelineRunFail_HTTPS_PROXY(t *testing.T) {
	t.Parallel()

	c, namespace := setup(t)
	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	t.Logf("Creating Git PipelineResource %s", gitSourceResourceName)
	if _, err := c.PipelineResourceClient.Create(getGitPipelineResource("master", "", "true", "", "invalid.https.proxy.com", "")); err != nil {
		t.Fatalf("Failed to create Pipeline Resource `%s`: %s", gitSourceResourceName, err)
	}

	t.Logf("Creating Task %s", gitTestTaskName)
	if _, err := c.TaskClient.Create(getGitCheckTask(namespace)); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", gitTestTaskName, err)
	}

	t.Logf("Creating Pipeline %s", gitTestPipelineName)
	if _, err := c.PipelineClient.Create(getGitCheckPipeline(namespace)); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", gitTestPipelineName, err)
	}

	t.Logf("Creating PipelineRun %s", gitTestPipelineRunName)
	if _, err := c.PipelineRunClient.Create(getGitCheckPipelineRun(namespace)); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", gitTestPipelineRunName, err)
	}

	if err := WaitForPipelineRunState(c, gitTestPipelineRunName, timeout, PipelineRunSucceed(gitTestPipelineRunName), "PipelineRunCompleted"); err != nil {
		taskruns, err := c.TaskRunClient.List(metav1.ListOptions{})
		if err != nil {
			t.Errorf("Error getting TaskRun list for PipelineRun %s %s", gitTestPipelineRunName, err)
		}
		for _, tr := range taskruns.Items {
			if tr.Status.PodName != "" {
				p, err := c.KubeClient.Kube.CoreV1().Pods(namespace).Get(tr.Status.PodName, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("Error getting pod `%s` in namespace `%s`", tr.Status.PodName, namespace)
				}

				for _, stat := range p.Status.ContainerStatuses {
					if strings.HasPrefix(stat.Name, "step-git-source-"+gitSourceResourceName) {
						if stat.State.Terminated != nil {
							req := c.KubeClient.Kube.CoreV1().Pods(namespace).GetLogs(p.Name, &corev1.PodLogOptions{Container: stat.Name})
							logContent, err := req.Do().Raw()
							if err != nil {
								t.Fatalf("Error getting pod logs for pod `%s` and container `%s` in namespace `%s`", tr.Status.PodName, stat.Name, namespace)
							}
							// Check for failure messages from fetch and pull in the log file
							if strings.Contains(strings.ToLower(string(logContent)), "could not resolve proxy: invalid.https.proxy.com") {
								t.Logf("Found exepected errors when using non-existent https proxy")
							} else {
								t.Logf("Container `%s` log File: %s", stat.Name, logContent)
								t.Fatalf("The git code extraction did not fail as expected.  Expected errors not found in log file.")
							}
						}
					}
				}
			}
		}

	} else {
		t.Fatalf("PipelineRun succeeded when should have failed")
	}
}

// TestGitPipelineRunWithNonMasterBranch is an integration test that will verify the source code is either fetched or pulled
// successfully under different revision inputs (default branch, branch)
// This test will run on spring-petclinic repository which does not contain a master branch as the default branch
func TestGitPipelineRunWithNonMasterBranch(t *testing.T) {
	t.Parallel()

	revisions := []string{"", "main"}

	for _, revision := range revisions {

		t.Run(revision, func(t *testing.T) {
			c, namespace := setup(t)
			knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
			defer tearDown(t, c, namespace)

			t.Logf("Creating Git PipelineResource %s", gitSourceResourceName)
			if _, err := c.PipelineResourceClient.Create(getGitPipelineResourceSpringPetClinic(revision, "", "true", "", "", "")); err != nil {
				t.Fatalf("Failed to create Pipeline Resource `%s`: %s", gitSourceResourceName, err)
			}

			t.Logf("Creating Task %s", gitTestTaskName)
			if _, err := c.TaskClient.Create(getGitCheckTask(namespace)); err != nil {
				t.Fatalf("Failed to create Task `%s`: %s", gitTestTaskName, err)
			}

			t.Logf("Creating Pipeline %s", gitTestPipelineName)
			if _, err := c.PipelineClient.Create(getGitCheckPipeline(namespace)); err != nil {
				t.Fatalf("Failed to create Pipeline `%s`: %s", gitTestPipelineName, err)
			}

			t.Logf("Creating PipelineRun %s", gitTestPipelineRunName)
			if _, err := c.PipelineRunClient.Create(getGitCheckPipelineRun(namespace)); err != nil {
				t.Fatalf("Failed to create Pipeline `%s`: %s", gitTestPipelineRunName, err)
			}

			if err := WaitForPipelineRunState(c, gitTestPipelineRunName, timeout, PipelineRunSucceed(gitTestPipelineRunName), "PipelineRunCompleted"); err != nil {
				t.Errorf("Error waiting for PipelineRun %s to finish: %s", gitTestPipelineRunName, err)
				t.Fatalf("PipelineRun execution failed")
			}
		})
	}
}

// getGitPipelineResourceSpringPetClinic will help to clone the spring-petclinic repository which does not contains master branch
func getGitPipelineResourceSpringPetClinic(revision, refspec, sslverify, httpproxy, httpsproxy, noproxy string) *v1alpha1.PipelineResource {
	return tb.PipelineResource(gitSourceResourceName, tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeGit,
		tb.PipelineResourceSpecParam("Url", "https://github.com/spring-projects/spring-petclinic"),
		tb.PipelineResourceSpecParam("Revision", revision),
		tb.PipelineResourceSpecParam("Refspec", refspec),
		tb.PipelineResourceSpecParam("sslVerify", sslverify),
		tb.PipelineResourceSpecParam("httpProxy", httpproxy),
		tb.PipelineResourceSpecParam("httpsProxy", httpsproxy),
		tb.PipelineResourceSpecParam("noProxy", noproxy),
	))
}

func getGitPipelineResource(revision, refspec, sslverify, httpproxy, httpsproxy, noproxy string) *v1alpha1.PipelineResource {
	return tb.PipelineResource(gitSourceResourceName, tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeGit,
		tb.PipelineResourceSpecParam("Url", "https://github.com/tektoncd/pipeline"),
		tb.PipelineResourceSpecParam("Revision", revision),
		tb.PipelineResourceSpecParam("Refspec", refspec),
		tb.PipelineResourceSpecParam("sslVerify", sslverify),
		tb.PipelineResourceSpecParam("httpProxy", httpproxy),
		tb.PipelineResourceSpecParam("httpsProxy", httpsproxy),
		tb.PipelineResourceSpecParam("noProxy", noproxy),
	))
}

func getGitCheckTask(namespace string) *v1beta1.Task {
	return &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: gitTestTaskName, Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Resources: &v1beta1.TaskResources{
				Inputs: []v1beta1.TaskResource{{ResourceDeclaration: v1beta1.ResourceDeclaration{
					Name: "gitsource", Type: resources.PipelineResourceTypeGit,
				}}},
			},
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Image: "alpine/git",
				Args:  []string{"--git-dir=/workspace/gitsource/.git", "show"},
			}}},
		},
	}
}

func getGitCheckPipeline(namespace string) *v1beta1.Pipeline {
	return &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: gitTestPipelineName, Namespace: namespace},
		Spec: v1beta1.PipelineSpec{
			Resources: []v1beta1.PipelineDeclaredResource{{
				Name: "git-repo", Type: resources.PipelineResourceTypeGit,
			}},
			Tasks: []v1beta1.PipelineTask{{
				Name:    "git-check",
				TaskRef: &v1beta1.TaskRef{Name: gitTestTaskName},
				Resources: &v1beta1.PipelineTaskResources{
					Inputs: []v1beta1.PipelineTaskInputResource{{
						Name:     "gitsource",
						Resource: "git-repo",
					}},
				},
			}},
		},
	}

}

func getGitCheckPipelineRun(namespace string) *v1beta1.PipelineRun {
	return &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: gitTestPipelineRunName, Namespace: namespace},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: gitTestPipelineName},
			Resources: []v1beta1.PipelineResourceBinding{{
				Name:        "git-repo",
				ResourceRef: &v1beta1.PipelineResourceRef{Name: gitSourceResourceName},
			}},
		},
	}
}
