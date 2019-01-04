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

	"github.com/knative/build-pipeline/pkg/apis/pipeline"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	knativetest "github.com/knative/pkg/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
)

var (
	pipelineRunTimeout = 10 * time.Minute
)

func TestPipelineRun(t *testing.T) {
	type tests struct {
		name                   string
		testSetup              func(c *clients, namespace string, index int)
		expectedTaskRuns       []string
		expectedNumberOfEvents int
		pipelineRunFunc        func(int, string) *v1alpha1.PipelineRun
	}

	tds := []tests{{
		name: "fan-in and fan-out",
		testSetup: func(c *clients, namespace string, index int) {
			t.Helper()
			for _, task := range getFanInFanOutTasks(namespace) {
				if _, err := c.TaskClient.Create(task); err != nil {
					t.Fatalf("Failed to create Task `%s`: %s", task.Name, err)
				}
			}
			if _, err := c.KubeClient.Kube.CoreV1().Secrets(namespace).Create(getServiceAccountSecret(namespace)); err != nil {
				t.Fatalf("Failed to create secret `%s`: %s", "service-account-secret", err)
			}
			for _, res := range getFanInFanOutGitResources(namespace) {
				if _, err := c.PipelineResourceClient.Create(res); err != nil {
					t.Fatalf("Failed to create Pipeline Resource `%s`: %s", kanikoResourceName, err)
				}
			}

			if _, err := c.PipelineClient.Create(getFanInFanOutPipeline(index, namespace)); err != nil {
				t.Fatalf("Failed to create Pipeline `%s`: %s", getName(hwPipelineName, index), err)
			}
		},
		pipelineRunFunc:  getFanInFanOutPipelineRun,
		expectedTaskRuns: []string{"create-file-kritis", "create-fan-out-1", "create-fan-out-2", "check-fan-in"},
		// 1 from PipelineRun and 4 from Tasks defined in pipelinerun
		expectedNumberOfEvents: 5,
	}, {
		name: "service account propagation",
		testSetup: func(c *clients, namespace string, index int) {
			t.Helper()
			if _, err := c.KubeClient.Kube.CoreV1().Secrets(namespace).Create(getPipelineRunSecret(index, namespace)); err != nil {
				t.Fatalf("Failed to create secret `%s`: %s", getName(hwSecret, index), err)
			}

			if _, err := c.KubeClient.Kube.CoreV1().ServiceAccounts(namespace).Create(getPipelineRunServiceAccount(index, namespace)); err != nil {
				t.Fatalf("Failed to create SA `%s`: %s", getName(hwSA, index), err)
			}

			if _, err := c.TaskClient.Create(&v1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      getName(hwTaskName, index),
				},
				Spec: v1alpha1.TaskSpec{
					// Reference build: https://github.com/knative/build/tree/master/test/docker-basic
					Steps: []corev1.Container{{
						Name:  "config-docker",
						Image: "gcr.io/cloud-builders/docker",
						// Private docker image for Build CRD testing
						Command: []string{"docker"},
						Args:    []string{"pull", "gcr.io/build-crd-testing/secret-sauce"},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "docker-socket",
							MountPath: "/var/run/docker.sock",
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: "docker-socket",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/var/run/docker.sock",
								Type: newHostPathType(string(corev1.HostPathSocket)),
							},
						},
					}},
				},
			}); err != nil {
				t.Fatalf("Failed to create Task `%s`: %s", getName(hwTaskName, index), err)
			}

			if _, err := c.PipelineClient.Create(getHelloWorldPipelineWithSingularTask(index, namespace)); err != nil {
				t.Fatalf("Failed to create Pipeline `%s`: %s", getName(hwPipelineName, index), err)
			}
		},
		expectedTaskRuns: []string{hwPipelineTaskName1},
		// 1 from PipelineRun and 1 from Tasks defined in pipelinerun
		expectedNumberOfEvents: 2,
		pipelineRunFunc:        getHelloWorldPipelineRun,
	}}

	for i, td := range tds {
		t.Run(td.name, func(t *testing.T) {
			td := td
			t.Parallel()
			// Note that getting a new logger has the side effect of setting the global metrics logger as well,
			// this means that metrics emitted from these tests will have the wrong test name attached. We should
			// revisit this if we ever start using those metrics (maybe use a different metrics gatherer).
			logger := getContextLogger(t.Name())
			c, namespace := setup(t, logger)

			knativetest.CleanupOnInterrupt(func() { tearDown(t, logger, c, namespace) }, logger)
			defer tearDown(t, logger, c, namespace)

			logger.Infof("Setting up test resources for %q test in namespace %s", td.name, namespace)
			td.testSetup(c, namespace, i)

			prName := fmt.Sprintf("%s%d", hwPipelineRunName, i)
			if _, err := c.PipelineRunClient.Create(td.pipelineRunFunc(i, namespace)); err != nil {
				t.Fatalf("Failed to create PipelineRun `%s`: %s", prName, err)
			}

			logger.Infof("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
			if err := WaitForPipelineRunState(c, prName, pipelineRunTimeout, PipelineRunSucceed(prName), "PipelineRunSuccess"); err != nil {
				t.Fatalf("Error waiting for PipelineRun %s to finish: %s", prName, err)
			}

			logger.Infof("Making sure the expected TaskRuns %s were created", td.expectedTaskRuns)

			expectedTaskRunNames := []string{}
			for _, runName := range td.expectedTaskRuns {
				taskRunName := strings.Join([]string{prName, runName}, "-")
				expectedTaskRunNames = append(expectedTaskRunNames, taskRunName)
				r, err := c.TaskRunClient.Get(taskRunName, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
				}
				if !r.Status.GetCondition(duckv1alpha1.ConditionSucceeded).IsTrue() {
					t.Fatalf("Expected TaskRun %s to have succeeded but Status is %v", taskRunName, r.Status)
				}
				for name, key := range map[string]string{
					hwPipelineRunName: pipeline.PipelineRunLabelKey,
					hwPipelineName:    pipeline.PipelineLabelKey,
				} {
					expectedName := getName(name, i)
					lbl := pipeline.GroupName + key
					if val := r.Labels[lbl]; val != expectedName {
						t.Errorf("Expected label %s=%s but got %s", lbl, expectedName, val)
					}
				}
			}

			matchKinds := map[string][]string{"PipelineRun": {prName}, "TaskRun": expectedTaskRunNames}

			logger.Infof("Making sure %d events were created from taskrun and pipelinerun with kinds %v", td.expectedNumberOfEvents, matchKinds)

			events, err := collectMatchingEvents(c.KubeClient, namespace, matchKinds, "Succeeded")
			if err != nil {
				t.Fatalf("Failed to collect matching events: %q", err)
			}
			if len(events) != td.expectedNumberOfEvents {
				t.Fatalf("Expected %d number of successful events from pipelinerun and taskrun but got %d; list of receieved events : %#v", td.expectedNumberOfEvents, len(events), events)
			}
			logger.Infof("Successfully finished test %q", td.name)
		})
	}
}

func getHelloWorldPipelineWithSingularTask(suffix int, namespace string) *v1alpha1.Pipeline {
	return &v1alpha1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      getName(hwPipelineName, suffix),
		},
		Spec: v1alpha1.PipelineSpec{
			Tasks: []v1alpha1.PipelineTask{{
				Name: hwPipelineTaskName1,
				TaskRef: v1alpha1.TaskRef{
					Name: getName(hwTaskName, suffix),
				},
			}},
		},
	}
}

func getFanInFanOutTasks(namespace string) []*v1alpha1.Task {
	return []*v1alpha1.Task{{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "create-file",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{{
					Name:       "workspace",
					Type:       v1alpha1.PipelineResourceTypeGit,
					TargetPath: "brandnewspace",
				}, {
					Name:       "gcsbucket",
					Type:       v1alpha1.PipelineResourceTypeStorage,
					TargetPath: "gcs-workspace",
				}},
			},
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{{
					Name: "workspace",
					Type: v1alpha1.PipelineResourceTypeGit,
				}},
			},
			Steps: []corev1.Container{{
				Name:    "write-data-task-0-step-0",
				Image:   "ubuntu",
				Command: []string{"/bin/bash"},
				Args:    []string{"-c", "echo stuff > /workspace/brandnewspace/stuff"},
			}, {
				Name:    "write-data-task-0-step-1",
				Image:   "ubuntu",
				Command: []string{"/bin/bash"},
				Args:    []string{"-c", "echo other > /workspace/brandnewspace/other"},
			}, {
				Name:    "read-gcs-bucket",
				Image:   "ubuntu",
				Command: []string{"/bin/bash"},
				Args:    []string{"-c", "ls -la /workspace/gcs-workspace/rules_docker-master.zip"},
			}, {
				Name:    "read-secret-env",
				Image:   "ubuntu",
				Command: []string{"/bin/bash"},
				Args:    []string{"-c", "cat $CREDENTIALS"},
				VolumeMounts: []corev1.VolumeMount{{
					Name: "volume-gcs-resource-service-account-secret", // this build should have volume with
					// name volume-(resource_name)-(secret_name) because of storage resource(gcs)
					MountPath: "/var/secret/service-account-secret",
				}},
				Env: []corev1.EnvVar{{
					Name:  "CREDENTIALS",
					Value: "/var/secret/service-account-secret/service_account-key.json",
				}},
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "check-create-files-exists",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{{
					Name: "workspace",
					Type: v1alpha1.PipelineResourceTypeGit,
				}},
			},
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{{
					Name: "workspace",
					Type: v1alpha1.PipelineResourceTypeGit,
				}},
			},
			Steps: []corev1.Container{{
				Name:    "read-from-task-0",
				Image:   "ubuntu",
				Command: []string{"bash"},
				Args:    []string{"-c", "[[ stuff == $(cat /workspace/stuff) ]]"},
			}, {
				Name:    "write-data-task-1",
				Image:   "ubuntu",
				Command: []string{"/bin/bash"},
				Args:    []string{"-c", "echo something > /workspace/something"},
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "check-create-files-exists-2",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{{
					Name: "workspace",
					Type: v1alpha1.PipelineResourceTypeGit,
				}},
			},
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{{
					Name: "workspace",
					Type: v1alpha1.PipelineResourceTypeGit,
				}},
			},
			Steps: []corev1.Container{{
				Name:    "read-from-task-0",
				Image:   "ubuntu",
				Command: []string{"bash"},
				Args:    []string{"-c", "[[ other == $(cat /workspace/other) ]]"},
			}, {
				Name:    "write-data-task-1",
				Image:   "ubuntu",
				Command: []string{"/bin/bash"},
				Args:    []string{"-c", "echo else > /workspace/else"},
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "read-files",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{{
					Name:       "workspace",
					Type:       v1alpha1.PipelineResourceTypeGit,
					TargetPath: "readingspace",
				}},
			},
			Steps: []corev1.Container{{
				Name:    "read-from-task-0",
				Image:   "ubuntu",
				Command: []string{"bash"},
				Args:    []string{"-c", "[[ stuff == $(cat /workspace/readingspace/stuff) ]]"},
			}, {
				Name:    "read-from-task-1",
				Image:   "ubuntu",
				Command: []string{"bash"},
				Args:    []string{"-c", "[[ something == $(cat /workspace/readingspace/something) ]]"},
			}, {
				Name:    "read-from-task-2",
				Image:   "ubuntu",
				Command: []string{"bash"},
				Args:    []string{"-c", "[[ else == $(cat /workspace/readingspace/else) ]]"},
			}},
		},
	}}
}

func getFanInFanOutPipeline(suffix int, namespace string) *v1alpha1.Pipeline {
	return &v1alpha1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      getName(hwPipelineName, suffix),
		},
		Spec: v1alpha1.PipelineSpec{
			Tasks: []v1alpha1.PipelineTask{{
				Name: "create-file-kritis",
				TaskRef: v1alpha1.TaskRef{
					Name: "create-file",
				},
			}, {
				Name: "create-fan-out-1",
				TaskRef: v1alpha1.TaskRef{
					Name: "check-create-files-exists",
				},
				ResourceDependencies: []v1alpha1.ResourceDependency{{
					Name:       "workspace",
					ProvidedBy: []string{"create-file-kritis"},
				}},
			}, {
				Name: "create-fan-out-2",
				TaskRef: v1alpha1.TaskRef{
					Name: "check-create-files-exists-2",
				},
				ResourceDependencies: []v1alpha1.ResourceDependency{{
					Name:       "workspace",
					ProvidedBy: []string{"create-file-kritis"},
				}},
			}, {
				Name: "check-fan-in",
				TaskRef: v1alpha1.TaskRef{
					Name: "read-files",
				},
				ResourceDependencies: []v1alpha1.ResourceDependency{{
					Name:       "workspace",
					ProvidedBy: []string{"create-fan-out-2", "create-fan-out-1"},
				}},
			}},
		},
	}
}

func getFanInFanOutGitResources(namespace string) []*v1alpha1.PipelineResource {
	return []*v1alpha1.PipelineResource{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kritis-resource-git",
			Namespace: namespace,
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: v1alpha1.PipelineResourceTypeGit,
			Params: []v1alpha1.Param{{
				Name:  "Url",
				Value: "https://github.com/grafeas/kritis",
			}, {
				Name:  "Revision",
				Value: "master",
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gcs-resource",
			Namespace: namespace,
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: v1alpha1.PipelineResourceTypeStorage,
			Params: []v1alpha1.Param{{
				Name:  "location",
				Value: "gs://build-crd-tests/rules_docker-master.zip",
			}, {
				Name:  "type",
				Value: "gcs",
			}},
			SecretParams: []v1alpha1.SecretParam{{
				SecretName: "service-account-secret",
				SecretKey:  "service_account-key.json",
				FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
			}},
		},
	}}
}

func getPipelineRunServiceAccount(suffix int, namespace string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      getName(hwSA, suffix),
		},
		Secrets: []corev1.ObjectReference{{
			Name: getName(hwSecret, suffix),
		}},
	}
}
func getFanInFanOutPipelineRun(suffix int, namespace string) *v1alpha1.PipelineRun {
	return &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      getName(hwPipelineRunName, suffix),
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{
				Name: getName(hwPipelineName, suffix),
			},
			PipelineTrigger: v1alpha1.PipelineTrigger{
				Type: v1alpha1.PipelineTriggerTypeManual,
			},
			PipelineTaskResources: []v1alpha1.PipelineTaskResource{
				{
					Name: "create-file-kritis",
					Inputs: []v1alpha1.TaskResourceBinding{{
						Name: "workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "kritis-resource-git",
						},
					}, {
						Name: "gcsbucket",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "gcs-resource",
						},
					}},
					Outputs: []v1alpha1.TaskResourceBinding{{
						Name: "workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "kritis-resource-git",
						},
					}},
				}, {
					Name: "create-fan-out-1",
					Inputs: []v1alpha1.TaskResourceBinding{{
						Name: "workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "kritis-resource-git",
						},
					}},
					Outputs: []v1alpha1.TaskResourceBinding{{
						Name: "workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "kritis-resource-git",
						},
					}},
				}, {
					Name: "create-fan-out-2",
					Inputs: []v1alpha1.TaskResourceBinding{{
						Name: "workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "kritis-resource-git",
						},
					}},
					Outputs: []v1alpha1.TaskResourceBinding{{
						Name: "workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "kritis-resource-git",
						},
					}},
				}, {
					Name: "check-fan-in",
					Inputs: []v1alpha1.TaskResourceBinding{{
						Name: "workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "kritis-resource-git",
						},
					}},
				}},
		},
	}
}

func getServiceAccountSecret(namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "service-account-secret",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{ // Reference: https://cloud.google.com/iam/docs/creating-managing-service-account-keys
			"service_account-key.json": []byte(`{
				"type": "service_account",
				"project_id": "[PROJECT-ID]",
				"private_key_id": "[KEY-ID]",
				"private_key": "-----BEGIN PRIVATE KEY-----\n[PRIVATE-KEY]\n-----END PRIVATE KEY-----\n",
				"client_email": "[SERVICE-ACCOUNT-EMAIL]",
				"client_id": "[CLIENT-ID]",
				"auth_uri": "https://accounts.google.com/o/oauth2/auth",
				"token_uri": "https://accounts.google.com/o/oauth2/token",
				"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
				"client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/[SERVICE-ACCOUNT-EMAIL]"
			}`),
		},
	}
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
			Name:      getName(hwSecret, suffix),
			Annotations: map[string]string{
				"build.knative.dev/docker-0": "https://us.gcr.io",
				"build.knative.dev/docker-1": "https://eu.gcr.io",
				"build.knative.dev/docker-2": "https://asia.gcr.io",
				"build.knative.dev/docker-3": "https://gcr.io",
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
	return &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      getName(hwPipelineRunName, suffix),
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{
				Name: getName(hwPipelineName, suffix),
			},
			PipelineTrigger: v1alpha1.PipelineTrigger{
				Type: v1alpha1.PipelineTriggerTypeManual,
			},
			ServiceAccount: fmt.Sprintf("%s%d", hwSA, suffix),
		},
	}
}

func getName(namespace string, suffix int) string {
	return fmt.Sprintf("%s%d", namespace, suffix)
}

func newHostPathType(pathType string) *corev1.HostPathType {
	hostPathType := new(corev1.HostPathType)
	*hostPathType = corev1.HostPathType(pathType)
	return hostPathType
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
