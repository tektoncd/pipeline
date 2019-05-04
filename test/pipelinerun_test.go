// +build e2e

/*
Copyright 2018 The Knative Authors

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
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tektoncd/pipeline/pkg/artifacts"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/knative/pkg/apis"
	knativetest "github.com/knative/pkg/test"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	pipelineName       = "pipeline"
	pipelineRunName    = "pipelinerun"
	secretName         = "secret"
	saName             = "service-account"
	taskName           = "task"
	task1Name          = "task1"
	pipelineRunTimeout = 10 * time.Minute
)

func TestPipelineRun(t *testing.T) {
	t.Parallel()
	type tests struct {
		name                   string
		testSetup              func(t *testing.T, c *clients, namespace string, index int)
		expectedTaskRuns       []string
		expectedNumberOfEvents int
		pipelineRunFunc        func(int, string) *v1alpha1.PipelineRun
	}

	tds := []tests{{
		name: "fan-in and fan-out",
		testSetup: func(t *testing.T, c *clients, namespace string, index int) {
			t.Helper()
			for _, task := range getFanInFanOutTasks(namespace) {
				if _, err := c.TaskClient.Create(task); err != nil {
					t.Fatalf("Failed to create Task `%s`: %s", task.Name, err)
				}
			}

			for _, res := range getFanInFanOutGitResources(namespace) {
				if _, err := c.PipelineResourceClient.Create(res); err != nil {
					t.Fatalf("Failed to create Pipeline Resource `%s`: %s", kanikoResourceName, err)
				}
			}

			if _, err := c.PipelineClient.Create(getFanInFanOutPipeline(index, namespace)); err != nil {
				t.Fatalf("Failed to create Pipeline `%s`: %s", getName(pipelineName, index), err)
			}
		},
		pipelineRunFunc:  getFanInFanOutPipelineRun,
		expectedTaskRuns: []string{"create-file-kritis", "create-fan-out-1", "create-fan-out-2", "check-fan-in"},
		// 1 from PipelineRun and 4 from Tasks defined in pipelinerun
		expectedNumberOfEvents: 5,
	}, {
		name: "service account propagation",
		testSetup: func(t *testing.T, c *clients, namespace string, index int) {
			t.Helper()
			if _, err := c.KubeClient.Kube.CoreV1().Secrets(namespace).Create(getPipelineRunSecret(index, namespace)); err != nil {
				t.Fatalf("Failed to create secret `%s`: %s", getName(secretName, index), err)
			}

			if _, err := c.KubeClient.Kube.CoreV1().ServiceAccounts(namespace).Create(getPipelineRunServiceAccount(index, namespace)); err != nil {
				t.Fatalf("Failed to create SA `%s`: %s", getName(saName, index), err)
			}

			task := tb.Task(getName(taskName, index), namespace, tb.TaskSpec(
				// Reference build: https://github.com/knative/build/tree/master/test/docker-basic
				tb.Step("config-docker", "quay.io/rhpipeline/skopeo:alpine",
					tb.Command("skopeo"),
					tb.Args("copy", "docker://gcr.io/build-crd-testing/secret-sauce", "dir:///tmp/"),
				),
			))
			if _, err := c.TaskClient.Create(task); err != nil {
				t.Fatalf("Failed to create Task `%s`: %s", getName(taskName, index), err)
			}

			if _, err := c.PipelineClient.Create(getHelloWorldPipelineWithSingularTask(index, namespace)); err != nil {
				t.Fatalf("Failed to create Pipeline `%s`: %s", getName(pipelineName, index), err)
			}
		},
		expectedTaskRuns: []string{task1Name},
		// 1 from PipelineRun and 1 from Tasks defined in pipelinerun
		expectedNumberOfEvents: 2,
		pipelineRunFunc:        getHelloWorldPipelineRun,
	}}

	for i, td := range tds {
		t.Run(td.name, func(t *testing.T) {
			td := td
			t.Parallel()
			c, namespace := setup(t)

			knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
			defer tearDown(t, c, namespace)

			t.Logf("Setting up test resources for %q test in namespace %s", td.name, namespace)
			td.testSetup(t, c, namespace, i)

			prName := fmt.Sprintf("%s%d", pipelineRunName, i)
			pipelineRun, err := c.PipelineRunClient.Create(td.pipelineRunFunc(i, namespace))
			if err != nil {
				t.Fatalf("Failed to create PipelineRun `%s`: %s", prName, err)
			}

			t.Logf("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
			if err := WaitForPipelineRunState(c, prName, pipelineRunTimeout, PipelineRunSucceed(prName), "PipelineRunSuccess"); err != nil {
				t.Fatalf("Error waiting for PipelineRun %s to finish: %s", prName, err)
			}

			t.Logf("Making sure the expected TaskRuns %s were created", td.expectedTaskRuns)
			actualTaskrunList, err := c.TaskRunClient.List(metav1.ListOptions{LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=%s", prName)})
			if err != nil {
				t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", prName, err)
			}
			expectedTaskRunNames := []string{}
			for _, runName := range td.expectedTaskRuns {
				taskRunName := strings.Join([]string{prName, runName}, "-")
				// check the actual task name starting with prName+runName with a random suffix
				for _, actualTaskRunItem := range actualTaskrunList.Items {
					if strings.HasPrefix(actualTaskRunItem.Name, taskRunName) {
						taskRunName = actualTaskRunItem.Name
					}
				}
				expectedTaskRunNames = append(expectedTaskRunNames, taskRunName)
				r, err := c.TaskRunClient.Get(taskRunName, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
				}
				if !r.Status.GetCondition(apis.ConditionSucceeded).IsTrue() {
					t.Fatalf("Expected TaskRun %s to have succeeded but Status is %v", taskRunName, r.Status)
				}

				t.Logf("Checking that labels were propagated correctly for TaskRun %s", r.Name)
				checkLabelPropagation(t, c, namespace, prName, r)
				t.Logf("Checking that annotations were propagated correctly for TaskRun %s", r.Name)
				checkAnnotationPropagation(t, c, namespace, prName, r)
			}

			matchKinds := map[string][]string{"PipelineRun": {prName}, "TaskRun": expectedTaskRunNames}

			t.Logf("Making sure %d events were created from taskrun and pipelinerun with kinds %v", td.expectedNumberOfEvents, matchKinds)

			events, err := collectMatchingEvents(c.KubeClient, namespace, matchKinds, "Succeeded")
			if err != nil {
				t.Fatalf("Failed to collect matching events: %q", err)
			}
			if len(events) != td.expectedNumberOfEvents {
				t.Fatalf("Expected %d number of successful events from pipelinerun and taskrun but got %d; list of receieved events : %#v", td.expectedNumberOfEvents, len(events), events)
			}
			// Check to make sure the PipelineRun's artifact storage PVC has been "deleted" at the end of the run.
			pvc, err := c.KubeClient.Kube.CoreV1().PersistentVolumeClaims(namespace).Get(artifacts.GetPVCName(pipelineRun), metav1.GetOptions{})
			if err != nil {
				if !errors.IsNotFound(err) {
					t.Fatalf("Error looking up PVC %s for PipelineRun %s: %s", artifacts.GetPVCName(pipelineRun), prName, err)
				}
			} else if pvc.DeletionTimestamp == nil {
				t.Fatalf("PVC %s still exists after PipelineRun %s has completed: %v", artifacts.GetPVCName(pipelineRun), prName, pvc)
			}
			t.Logf("Successfully finished test %q", td.name)
		})
	}
}

func getHelloWorldPipelineWithSingularTask(suffix int, namespace string) *v1alpha1.Pipeline {
	return tb.Pipeline(getName(pipelineName, suffix), namespace, tb.PipelineSpec(
		tb.PipelineTask(task1Name, getName(taskName, suffix)),
	))
}

func getFanInFanOutTasks(namespace string) []*v1alpha1.Task {
	inWorkspaceResource := tb.InputsResource("workspace", v1alpha1.PipelineResourceTypeGit)
	outWorkspaceResource := tb.OutputsResource("workspace", v1alpha1.PipelineResourceTypeGit)
	return []*v1alpha1.Task{
		tb.Task("create-file", namespace, tb.TaskSpec(
			tb.TaskInputs(tb.InputsResource("workspace", v1alpha1.PipelineResourceTypeGit,
				tb.ResourceTargetPath("brandnewspace"),
			)),
			tb.TaskOutputs(outWorkspaceResource),
			tb.Step("write-data-task-0-step-0", "ubuntu", tb.Command("/bin/bash"),
				tb.Args("-c", "echo stuff > /workspace/brandnewspace/stuff"),
			),
			tb.Step("write-data-task-0-step-1", "ubuntu", tb.Command("/bin/bash"),
				tb.Args("-c", "echo other > /workspace/brandnewspace/other"),
			),
		)),
		tb.Task("check-create-files-exists", namespace, tb.TaskSpec(
			tb.TaskInputs(inWorkspaceResource),
			tb.TaskOutputs(outWorkspaceResource),
			tb.Step("read-from-task-0", "ubuntu", tb.Command("/bin/bash"),
				tb.Args("-c", "[[ stuff == $(cat /workspace//workspace/stuff) ]]"),
			),
			tb.Step("write-data-task-1", "ubuntu", tb.Command("/bin/bash"),
				tb.Args("-c", "echo something > /workspace/workspace/something"),
			),
		)),
		tb.Task("check-create-files-exists-2", namespace, tb.TaskSpec(
			tb.TaskInputs(inWorkspaceResource),
			tb.TaskOutputs(outWorkspaceResource),
			tb.Step("read-from-task-0", "ubuntu", tb.Command("/bin/bash"),
				tb.Args("-c", "[[ other == $(cat /workspace/workspace/other) ]]"),
			),
			tb.Step("write-data-task-1", "ubuntu", tb.Command("/bin/bash"),
				tb.Args("-c", "echo else > /workspace/workspace/else"),
			),
		)),
		tb.Task("read-files", namespace, tb.TaskSpec(
			tb.TaskInputs(tb.InputsResource("workspace", v1alpha1.PipelineResourceTypeGit,
				tb.ResourceTargetPath("readingspace"),
			)),
			tb.Step("read-from-task-0", "ubuntu", tb.Command("/bin/bash"),
				tb.Args("-c", "[[ stuff == $(cat /workspace/readingspace/stuff) ]]"),
			),
			tb.Step("read-from-task-1", "ubuntu", tb.Command("/bin/bash"),
				tb.Args("-c", "[[ something == $(cat /workspace/readingspace/something) ]]"),
			),
			tb.Step("read-from-task-2", "ubuntu", tb.Command("/bin/bash"),
				tb.Args("-c", "[[ else == $(cat /workspace/readingspace/else) ]]"),
			),
		)),
	}
}

func getFanInFanOutPipeline(suffix int, namespace string) *v1alpha1.Pipeline {
	outGitResource := tb.PipelineTaskOutputResource("workspace", "git-repo")

	return tb.Pipeline(getName(pipelineName, suffix), namespace, tb.PipelineSpec(
		tb.PipelineDeclaredResource("git-repo", "git"),
		tb.PipelineTask("create-file-kritis", "create-file",
			tb.PipelineTaskInputResource("workspace", "git-repo"),
			outGitResource,
		),
		tb.PipelineTask("create-fan-out-1", "check-create-files-exists",
			tb.PipelineTaskInputResource("workspace", "git-repo", tb.From("create-file-kritis")),
			outGitResource,
		),
		tb.PipelineTask("create-fan-out-2", "check-create-files-exists-2",
			tb.PipelineTaskInputResource("workspace", "git-repo", tb.From("create-file-kritis")),
			outGitResource,
		),
		tb.PipelineTask("check-fan-in", "read-files",
			tb.PipelineTaskInputResource("workspace", "git-repo", tb.From("create-fan-out-2", "create-fan-out-1")),
		),
	))
}

func getFanInFanOutGitResources(namespace string) []*v1alpha1.PipelineResource {
	return []*v1alpha1.PipelineResource{
		tb.PipelineResource("kritis-resource-git", namespace, tb.PipelineResourceSpec(
			v1alpha1.PipelineResourceTypeGit,
			tb.PipelineResourceSpecParam("Url", "https://github.com/grafeas/kritis"),
			tb.PipelineResourceSpecParam("Revision", "master"),
		)),
	}
}

func getPipelineRunServiceAccount(suffix int, namespace string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      getName(saName, suffix),
		},
		Secrets: []corev1.ObjectReference{{
			Name: getName(secretName, suffix),
		}},
	}
}
func getFanInFanOutPipelineRun(suffix int, namespace string) *v1alpha1.PipelineRun {
	return tb.PipelineRun(getName(pipelineRunName, suffix), namespace,
		tb.PipelineRunSpec(getName(pipelineName, suffix),
			tb.PipelineRunResourceBinding("git-repo", tb.PipelineResourceBindingRef("kritis-resource-git")),
		))
}

func getPipelineRunSecret(suffix int, namespace string) *corev1.Secret {
	// Generated by:
	//   cat /tmp/key.json | base64 -w 0
	// This service account is JUST a storage reader on gcr.io/build-crd-testing
	encoedDockercred := "ewogICJ0eXBlIjogInNlcnZpY2VfYWNjb3VudCIsCiAgInByb2plY3RfaWQiOiAiYnVpbGQtY3JkLXRlc3RpbmciLAogICJwcml2YXRlX2tleV9pZCI6ICIwNTAyYTQxYTgxMmZiNjRjZTU2YTY4ZWM1ODMyYWIwYmExMWMxMWU2IiwKICAicHJpdmF0ZV9rZXkiOiAiLS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tXG5NSUlFdlFJQkFEQU5CZ2txaGtpRzl3MEJBUUVGQUFTQ0JLY3dnZ1NqQWdFQUFvSUJBUUM5WDRFWU9BUmJ4UU04XG5EMnhYY2FaVGsrZ1k4ZWp1OTh0THFDUXFUckdNVzlSZVQyeE9ZNUF5Z2FsUFArcDd5WEVja3dCRC9IaE0wZ2xJXG43TVRMZGVlS1dyK3JBMUx3SFp5V0ZXN0gwT25mN3duWUhFSExXVW1jM0JDT1JFRHRIUlo3WnJQQmYxSFRBQS8zXG5Nblc1bFpIU045b2p6U1NGdzZBVnU2ajZheGJCSUlKNzU0THJnS2VBWXVyd2ZJUTJSTFR1MjAxazJJcUxZYmhiXG4zbVNWRzVSK3RiS3oxQ3ZNNTNuSENiN0NmdVZlV3NyQThrazd4SHJyTFFLTW1JOXYyc2dSdWd5TUF6d3ovNnpOXG5oNS9pTXh4Z2VxNVc4eGtWeDNKMm5ZOEpKZEhhZi9UNkFHc09ORW80M3B4ZWlRVmpuUmYvS24xMFRDYzJFc0lZXG5TNDlVc1o3QkFnTUJBQUVDZ2dFQUF1cGxkdWtDUVF1RDVVL2dhbUh0N0dnVzNBTVYxOGVxbkhuQ2EyamxhaCtTXG5BZVVHbmhnSmpOdkUrcE1GbFN2NXVmMnAySzRlZC9veEQ2K0NwOVpYRFJqZ3ZmdEl5cWpsemJ3dkZjZ3p3TnVEXG55Z1VrdXA3SGVjRHNEOFR0ZUFvYlQvVnB3cTZ6S01yQndDdk5rdnk2YlZsb0VqNXgzYlhzYXhlOTVETy95cHU2XG53MFc5N3p4d3dESlk2S1FjSVdNamhyR3h2d1g3bmlVQ2VNNGxlV0JEeUd0dzF6ZUpuNGhFYzZOM2FqUWFjWEtjXG4rNFFseGNpYW1ZcVFXYlBudHhXUWhoUXpjSFdMaTJsOWNGYlpENyt1SkxGNGlONnk4bVZOVTNLM0sxYlJZclNEXG5SVXAzYVVWQlhtRmcrWi8ycHVWTCttVTNqM0xMV1l5Qk9rZXZ1T21kZ1FLQmdRRGUzR0lRa3lXSVMxNFRkTU9TXG5CaUtCQ0R5OGg5NmVoTDBIa0RieU9rU3RQS2RGOXB1RXhaeGh5N29qSENJTTVGVnJwUk4yNXA0c0V6d0ZhYyt2XG5KSUZnRXZxN21YZm1YaVhJTmllUG9FUWFDbm54RHhXZ21yMEhVS0VtUzlvTWRnTGNHVStrQ1ZHTnN6N0FPdW0wXG5LcVkzczIyUTlsUTY3Rk95cWl1OFdGUTdRUUtCZ1FEWmlGaFRFWmtQRWNxWmpud0pwVEI1NlpXUDlLVHNsWlA3XG53VTRiemk2eSttZXlmM01KKzRMMlN5SGMzY3BTTWJqdE5PWkN0NDdiOTA4RlVtTFhVR05oY3d1WmpFUXhGZXkwXG5tNDFjUzVlNFA0OWI5bjZ5TEJqQnJCb3FzMldCYWwyZWdkaE5KU3NDV29pWlA4L1pUOGVnWHZoN2I5MWp6b0syXG5xMlBVbUE0RGdRS0JnQVdMMklqdkVJME95eDJTMTFjbi9lM1dKYVRQZ05QVEc5MDNVcGErcW56aE9JeCtNYXFoXG5QRjRXc3VBeTBBb2dHSndnTkpiTjhIdktVc0VUdkE1d3l5TjM5WE43dzBjaGFyRkwzN29zVStXT0F6RGpuamNzXG5BcTVPN0dQR21YdWI2RUJRQlBKaEpQMXd5NHYvSzFmSGcvRjQ3cTRmNDBMQUpPa2FZUkpENUh6QkFvR0JBTlVoXG5uSUJQSnFxNElNdlE2Y0M5ZzhCKzF4WURlYTkvWWsxdytTbVBHdndyRVh5M0dLeDRLN2xLcGJQejdtNFgzM3N4XG5zRVUvK1kyVlFtd1JhMXhRbS81M3JLN1YybDVKZi9ENDAwalJtNlpmU0FPdmdEVHJ0Wm5VR0pNcno5RTd1Tnc3XG5sZ1VIM0pyaXZ5Ri9meE1JOHFzelFid1hQMCt4bnlxQXhFQWdkdUtCQW9HQUlNK1BTTllXQ1pYeERwU0hJMThkXG5qS2tvQWJ3Mk1veXdRSWxrZXVBbjFkWEZhZDF6c1hRR2RUcm1YeXY3TlBQKzhHWEJrbkJMaTNjdnhUaWxKSVN5XG51Y05yQ01pcU5BU24vZHE3Y1dERlVBQmdqWDE2SkgyRE5GWi9sL1VWRjNOREFKalhDczFYN3lJSnlYQjZveC96XG5hU2xxbElNVjM1REJEN3F4Unl1S3Nnaz1cbi0tLS0tRU5EIFBSSVZBVEUgS0VZLS0tLS1cbiIsCiAgImNsaWVudF9lbWFpbCI6ICJwdWxsLXNlY3JldC10ZXN0aW5nQGJ1aWxkLWNyZC10ZXN0aW5nLmlhbS5nc2VydmljZWFjY291bnQuY29tIiwKICAiY2xpZW50X2lkIjogIjEwNzkzNTg2MjAzMzAyNTI1MTM1MiIsCiAgImF1dGhfdXJpIjogImh0dHBzOi8vYWNjb3VudHMuZ29vZ2xlLmNvbS9vL29hdXRoMi9hdXRoIiwKICAidG9rZW5fdXJpIjogImh0dHBzOi8vYWNjb3VudHMuZ29vZ2xlLmNvbS9vL29hdXRoMi90b2tlbiIsCiAgImF1dGhfcHJvdmlkZXJfeDUwOV9jZXJ0X3VybCI6ICJodHRwczovL3d3dy5nb29nbGVhcGlzLmNvbS9vYXV0aDIvdjEvY2VydHMiLAogICJjbGllbnRfeDUwOV9jZXJ0X3VybCI6ICJodHRwczovL3d3dy5nb29nbGVhcGlzLmNvbS9yb2JvdC92MS9tZXRhZGF0YS94NTA5L3B1bGwtc2VjcmV0LXRlc3RpbmclNDBidWlsZC1jcmQtdGVzdGluZy5pYW0uZ3NlcnZpY2VhY2NvdW50LmNvbSIKfQo="

	decoded, err := base64.StdEncoding.DecodeString(encoedDockercred)
	if err != nil {
		return nil
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      getName(secretName, suffix),
			Annotations: map[string]string{
				"tekton.dev/docker-0": "https://us.gcr.io",
				"tekton.dev/docker-1": "https://eu.gcr.io",
				"tekton.dev/docker-2": "https://asia.gcr.io",
				"tekton.dev/docker-3": "https://gcr.io",
			},
		},
		Type: "kubernetes.io/basic-auth",
		Data: map[string][]byte{
			"username": []byte("_json_key"),
			"password": decoded,
		},
	}
}

func getHelloWorldPipelineRun(suffix int, namespace string) *v1alpha1.PipelineRun {
	return tb.PipelineRun(getName(pipelineRunName, suffix), namespace,
		tb.PipelineRunLabel("hello-world-key", "hello-world-value"),
		tb.PipelineRunSpec(getName(pipelineName, suffix),
			tb.PipelineRunServiceAccount(fmt.Sprintf("%s%d", saName, suffix)),
		),
	)
}

func getName(namespace string, suffix int) string {
	return fmt.Sprintf("%s%d", namespace, suffix)
}

// collectMatchingEvents collects list of events under 5 seconds that match
// 1. matchKinds which is a map of Kind of Object with name of objects
// 2. reason which is the expected reason of event
func collectMatchingEvents(kubeClient *knativetest.KubeClient, namespace string, kinds map[string][]string, reason string) ([]*corev1.Event, error) {
	var events []*corev1.Event

	watchEvents, err := kubeClient.Kube.CoreV1().Events(namespace).Watch(metav1.ListOptions{})
	// close watchEvents channel
	defer watchEvents.Stop()
	if err != nil {
		return events, err
	}

	// create timer to not wait for events longer than 5 seconds
	timer := time.NewTimer(5 * time.Second)

	for {
		select {
		case wevent := <-watchEvents.ResultChan():
			event := wevent.Object.(*corev1.Event)
			if val, ok := kinds[event.InvolvedObject.Kind]; ok {
				for _, expectedName := range val {
					if event.InvolvedObject.Name == expectedName && event.Reason == reason {
						events = append(events, event)
					}
				}
			}
		case <-timer.C:
			return events, nil
		}
	}
}

// checkLabelPropagation checks that labels are correctly propagating from
// Pipelines, PipelineRuns, and Tasks to TaskRuns and Pods.
func checkLabelPropagation(t *testing.T, c *clients, namespace string, pipelineRunName string, tr *v1alpha1.TaskRun) {
	// Our controllers add 4 labels automatically. If custom labels are set on
	// the Pipeline, PipelineRun, or Task then the map will have to be resized.
	labels := make(map[string]string, 4)

	// Check label propagation to PipelineRuns.
	pr, err := c.PipelineRunClient.Get(pipelineRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected PipelineRun for %s: %s", tr.Name, err)
	}
	p, err := c.PipelineClient.Get(pr.Spec.PipelineRef.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected Pipeline for %s: %s", pr.Name, err)
	}
	for key, val := range p.ObjectMeta.Labels {
		labels[key] = val
	}
	// This label is added to every PipelineRun by the PipelineRun controller
	labels[pipeline.GroupName+pipeline.PipelineLabelKey] = p.Name
	assertLabelsMatch(t, labels, pr.ObjectMeta.Labels)

	// Check label propagation to TaskRuns.
	for key, val := range pr.ObjectMeta.Labels {
		labels[key] = val
	}
	// This label is added to every TaskRun by the PipelineRun controller
	labels[pipeline.GroupName+pipeline.PipelineRunLabelKey] = pr.Name
	if tr.Spec.TaskRef != nil {
		task, err := c.TaskClient.Get(tr.Spec.TaskRef.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Couldn't get expected Task for %s: %s", tr.Name, err)
		}
		for key, val := range task.ObjectMeta.Labels {
			labels[key] = val
		}
		// This label is added to TaskRuns that reference a Task by the TaskRun controller
		labels[pipeline.GroupName+pipeline.TaskLabelKey] = task.Name
	}
	assertLabelsMatch(t, labels, tr.ObjectMeta.Labels)

	// PodName is "" iff a retry happened and pod is deleted
	// This label is added to every Pod by the TaskRun controller
	if tr.Status.PodName != "" {
		// Check label propagation to Pods.
		pod := getPodForTaskRun(t, c.KubeClient, namespace, tr)
		// This label is added to every Pod by the TaskRun controller
		labels[pipeline.GroupName+pipeline.TaskRunLabelKey] = tr.Name
		assertLabelsMatch(t, labels, pod.ObjectMeta.Labels)
	}
}

// checkAnnotationPropagation checks that annotations are correctly propagating from
// Pipelines, PipelineRuns, and Tasks to TaskRuns and Pods.
func checkAnnotationPropagation(t *testing.T, c *clients, namespace string, pipelineRunName string, tr *v1alpha1.TaskRun) {
	annotations := make(map[string]string)

	// Check annotation propagation to PipelineRuns.
	pr, err := c.PipelineRunClient.Get(pipelineRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected PipelineRun for %s: %s", tr.Name, err)
	}
	p, err := c.PipelineClient.Get(pr.Spec.PipelineRef.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected Pipeline for %s: %s", pr.Name, err)
	}
	for key, val := range p.ObjectMeta.Annotations {
		annotations[key] = val
	}
	assertAnnotationsMatch(t, annotations, pr.ObjectMeta.Annotations)

	// Check annotation propagation to TaskRuns.
	for key, val := range pr.ObjectMeta.Annotations {
		annotations[key] = val
	}
	if tr.Spec.TaskRef != nil {
		task, err := c.TaskClient.Get(tr.Spec.TaskRef.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Couldn't get expected Task for %s: %s", tr.Name, err)
		}
		for key, val := range task.ObjectMeta.Annotations {
			annotations[key] = val
		}
	}
	assertAnnotationsMatch(t, annotations, tr.ObjectMeta.Annotations)

	// Check annotation propagation to Pods.
	pod := getPodForTaskRun(t, c.KubeClient, namespace, tr)
	assertAnnotationsMatch(t, annotations, pod.ObjectMeta.Annotations)
}

func getPodForTaskRun(t *testing.T, kubeClient *knativetest.KubeClient, namespace string, tr *v1alpha1.TaskRun) *corev1.Pod {
	// The Pod name has a random suffix, so we filter by label to find the one we care about.
	pods, err := kubeClient.Kube.CoreV1().Pods(namespace).List(metav1.ListOptions{
		LabelSelector: pipeline.GroupName + pipeline.TaskRunLabelKey + " = " + tr.Name,
	})
	if err != nil {
		t.Fatalf("Couldn't get expected Pod for %s: %s", tr.Name, err)
	}
	if numPods := len(pods.Items); numPods != 1 {
		t.Fatalf("Expected 1 Pod for %s, but got %d Pods", tr.Name, numPods)
	}
	return &pods.Items[0]
}

func assertLabelsMatch(t *testing.T, expectedLabels, actualLabels map[string]string) {
	for key, expectedVal := range expectedLabels {
		if actualVal := actualLabels[key]; actualVal != expectedVal {
			t.Errorf("Expected labels containing %s=%s but labels were %v", key, expectedVal, actualLabels)
		}
	}
}

func assertAnnotationsMatch(t *testing.T, expectedAnnotations, actualAnnotations map[string]string) {
	for key, expectedVal := range expectedAnnotations {
		if actualVal := actualAnnotations[key]; actualVal != expectedVal {
			t.Errorf("Expected annotations containing %s=%s but annotations were %v", key, expectedVal, actualAnnotations)
		}
	}
}
