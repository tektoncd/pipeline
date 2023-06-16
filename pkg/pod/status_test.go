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

package pod

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/go-multierror"
	"github.com/tektoncd/pipeline/internal/sidecarlogresults"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
)

var ignoreVolatileTime = cmp.Comparer(func(_, _ apis.VolatileTime) bool { return true })

func TestSetTaskRunStatusBasedOnStepStatus(t *testing.T) {
	for _, c := range []struct {
		desc              string
		ContainerStatuses []corev1.ContainerStatus
	}{{
		desc: "test result with large pipeline result",
		ContainerStatuses: []corev1.ContainerStatus{{
			Name: "step-bar-0",
			State: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{
					Message: `[{"key":"resultName","value":"resultValue", "type":1}, {"key":"digest","value":"sha256:1234","resourceName":"source-image"}]`,
				},
			},
		},
			{
				Name: "step-bar1",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"key":"resultName","value":"resultValue", "type":1}, {"key":"digest","value":"sha256:1234` + strings.Repeat("a", 3072) + `","resourceName":"source-image"}]`,
					},
				},
			},
			{
				Name: "step-bar2",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"key":"resultName","value":"resultValue", "type":1}, {"key":"digest","value":"sha256:1234` + strings.Repeat("a", 3072) + `","resourceName":"source-image"}]`,
					},
				},
			}},
	}, {
		desc: "The ExitCode in the result cannot modify the original ExitCode",
		ContainerStatuses: []corev1.ContainerStatus{{
			Name: "step-bar-0",
			State: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{
					ExitCode: 0,
					Message:  `[{"key":"ExitCode","value":"1","type":3}]`,
				},
			},
		}},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			startTime := time.Date(2010, 1, 1, 1, 1, 1, 1, time.UTC)
			tr := v1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "task-run",
					Namespace: "foo",
				},
				Status: v1.TaskRunStatus{
					TaskRunStatusFields: v1.TaskRunStatusFields{
						StartTime: &metav1.Time{Time: startTime},
					},
				},
			}

			logger, _ := logging.NewLogger("", "status")
			kubeclient := fakek8s.NewSimpleClientset()
			originalStatuses := make([]corev1.ContainerStatus, 0, len(c.ContainerStatuses))
			for _, cs := range c.ContainerStatuses {
				originalStatuses = append(originalStatuses, *cs.DeepCopy())
			}
			merr := setTaskRunStatusBasedOnStepStatus(context.Background(), logger, c.ContainerStatuses, &tr, corev1.PodRunning, kubeclient, &v1.TaskSpec{})
			if merr != nil {
				t.Errorf("setTaskRunStatusBasedOnStepStatus: %s", merr)
			}
			if d := cmp.Diff(originalStatuses, c.ContainerStatuses); d != "" {
				t.Errorf("container statuses changed:  %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestSetTaskRunStatusBasedOnStepStatus_sidecar_logs(t *testing.T) {
	for _, c := range []struct {
		desc          string
		maxResultSize int
		wantErr       error
	}{{
		desc:          "test result with sidecar logs too large",
		maxResultSize: 1,
		wantErr:       sidecarlogresults.ErrSizeExceeded,
	}, {
		desc:          "test result with sidecar logs bad format",
		maxResultSize: 4096,
		wantErr:       fmt.Errorf("%s", "Invalid result invalid character 'k' in literal false (expecting 'l')"),
	}} {
		t.Run(c.desc, func(t *testing.T) {
			tr := v1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "task-run",
					Namespace: "foo",
				},
				Status: v1.TaskRunStatus{
					TaskRunStatusFields: v1.TaskRunStatusFields{
						TaskSpec: &v1.TaskSpec{
							Results: []v1.TaskResult{{
								Name: "result1",
							}},
						},
						PodName: "task-run-pod",
					},
				},
			}

			logger, _ := logging.NewLogger("", "status")
			kubeclient := fakek8s.NewSimpleClientset()
			pod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "task-run-pod",
					Namespace: "foo",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "container",
							Image: "image",
						},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			}
			pod, err := kubeclient.CoreV1().Pods(pod.Namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
			if err != nil {
				t.Errorf("Error occurred while creating pod %s: %s", pod.Name, err.Error())
			}
			ctx := config.ToContext(context.Background(), &config.Config{
				FeatureFlags: &config.FeatureFlags{
					ResultExtractionMethod: config.ResultExtractionMethodSidecarLogs,
					MaxResultSize:          c.maxResultSize,
				},
			})
			var wantErr *multierror.Error
			wantErr = multierror.Append(wantErr, c.wantErr)
			merr := setTaskRunStatusBasedOnStepStatus(ctx, logger, []corev1.ContainerStatus{{}}, &tr, pod.Status.Phase, kubeclient, &v1.TaskSpec{})

			if d := cmp.Diff(wantErr.Error(), merr.Error()); d != "" {
				t.Errorf("Got unexpected error  %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestMakeTaskRunStatus(t *testing.T) {
	for _, c := range []struct {
		desc      string
		podStatus corev1.PodStatus
		pod       corev1.Pod
		want      v1.TaskRunStatus
	}{{
		desc:      "empty",
		podStatus: corev1.PodStatus{},

		want: v1.TaskRunStatus{
			Status: statusRunning(),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps:    []v1.StepState{},
				Sidecars: []v1.SidecarState{},
			},
		},
	}, {
		desc: "ignore-creds-init",
		podStatus: corev1.PodStatus{
			InitContainerStatuses: []corev1.ContainerStatus{{
				// creds-init; ignored
				ImageID: "ignored",
			}},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:    "step-state-name",
				ImageID: "",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 123,
					},
				},
			}},
		},
		want: v1.TaskRunStatus{
			Status: statusRunning(),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 123,
						}},
					Name:      "state-name",
					Container: "step-state-name",
				}},
				Sidecars: []v1.SidecarState{},
			},
		},
	}, {
		desc: "ignore-init-containers",
		podStatus: corev1.PodStatus{
			InitContainerStatuses: []corev1.ContainerStatus{{
				// creds-init; ignored.
				ImageID: "ignoreme",
			}, {
				// git-init; ignored.
				ImageID: "ignoreme",
			}},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:    "step-state-name",
				ImageID: "image-id",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 123,
					},
				},
			}},
		},
		want: v1.TaskRunStatus{
			Status: statusRunning(),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 123,
						}},
					Name:      "state-name",
					Container: "step-state-name",
					ImageID:   "image-id",
				}},
				Sidecars: []v1.SidecarState{},
			},
		},
	}, {
		desc: "success",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "step-step-push",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 0,
					},
				},
				ImageID: "image-id",
			}},
		},
		want: v1.TaskRunStatus{
			Status: statusSuccess(),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 0,
						}},
					Name:      "step-push",
					Container: "step-step-push",
					ImageID:   "image-id",
				}},
				Sidecars: []v1.SidecarState{},
				// We don't actually care about the time, just that it's not nil
				CompletionTime: &metav1.Time{Time: time.Now()},
			},
		},
	}, {
		desc: "running",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "step-running-step",
				State: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{},
				},
			}},
		},
		want: v1.TaskRunStatus{
			Status: statusRunning(),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
					Name:      "running-step",
					Container: "step-running-step",
				}},
				Sidecars: []v1.SidecarState{},
			},
		},
	}, {
		desc: "failure-terminated",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodFailed,
			InitContainerStatuses: []corev1.ContainerStatus{{
				// creds-init status; ignored
				ImageID: "ignore-me",
			}},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:    "step-failure",
				ImageID: "image-id",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 123,
					},
				},
			}},
		},
		want: v1.TaskRunStatus{
			Status: statusFailure(v1.TaskRunReasonFailed.String(), "\"step-failure\" exited with code 123 (image: \"image-id\"); for logs run: kubectl -n foo logs pod -c step-failure\n"),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 123,
						}},

					Name:      "failure",
					Container: "step-failure",
					ImageID:   "image-id",
				}},
				Sidecars: []v1.SidecarState{},
				// We don't actually care about the time, just that it's not nil
				CompletionTime: &metav1.Time{Time: time.Now()},
			},
		},
	}, {
		desc: "failure-message",
		podStatus: corev1.PodStatus{
			Phase:   corev1.PodFailed,
			Message: "boom",
		},
		want: v1.TaskRunStatus{
			Status: statusFailure(v1.TaskRunReasonFailed.String(), "boom"),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps:    []v1.StepState{},
				Sidecars: []v1.SidecarState{},
				// We don't actually care about the time, just that it's not nil
				CompletionTime: &metav1.Time{Time: time.Now()},
			},
		},
	}, {
		desc: "failed with OOM",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "step-step-push",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Reason:   "OOMKilled",
						ExitCode: 0,
					},
				},
				ImageID: "image-id",
			}},
		},
		want: v1.TaskRunStatus{
			Status: statusFailure(v1.TaskRunReasonFailed.String(), "OOMKilled"),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Reason:   "OOMKilled",
							ExitCode: 0,
						}},
					Name:      "step-push",
					Container: "step-step-push",
					ImageID:   "image-id",
				}},
				Sidecars: []v1.SidecarState{},
				// We don't actually care about the time, just that it's not nil
				CompletionTime: &metav1.Time{Time: time.Now()},
			},
		},
	}, {
		desc:      "failure-unspecified",
		podStatus: corev1.PodStatus{Phase: corev1.PodFailed},
		want: v1.TaskRunStatus{
			Status: statusFailure(v1.TaskRunReasonFailed.String(), "build failed for unspecified reasons."),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps:    []v1.StepState{},
				Sidecars: []v1.SidecarState{},
				// We don't actually care about the time, just that it's not nil
				CompletionTime: &metav1.Time{Time: time.Now()},
			},
		},
	}, {
		desc: "pending-waiting-message",
		podStatus: corev1.PodStatus{
			Phase:                 corev1.PodPending,
			InitContainerStatuses: []corev1.ContainerStatus{{
				// creds-init status; ignored
			}},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "step-status-name",
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Message: "i'm pending",
					},
				},
			}},
		},
		want: v1.TaskRunStatus{
			Status: statusPending("Pending", `build step "step-status-name" is pending with reason "i'm pending"`),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Message: "i'm pending",
						},
					},
					Name:      "status-name",
					Container: "step-status-name",
				}},
				Sidecars: []v1.SidecarState{},
			},
		},
	}, {
		desc: "pending-pod-condition",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{{
				Status:  corev1.ConditionUnknown,
				Type:    "the type",
				Message: "the message",
			}},
		},
		want: v1.TaskRunStatus{
			Status: statusPending("Pending", `pod status "the type":"Unknown"; message: "the message"`),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps:    []v1.StepState{},
				Sidecars: []v1.SidecarState{},
			},
		},
	}, {
		desc: "pending-message",
		podStatus: corev1.PodStatus{
			Phase:   corev1.PodPending,
			Message: "pod status message",
		},
		want: v1.TaskRunStatus{
			Status: statusPending("Pending", "pod status message"),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps:    []v1.StepState{},
				Sidecars: []v1.SidecarState{},
			},
		},
	}, {
		desc:      "pending-no-message",
		podStatus: corev1.PodStatus{Phase: corev1.PodPending},
		want: v1.TaskRunStatus{
			Status: statusPending("Pending", "Pending"),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps:    []v1.StepState{},
				Sidecars: []v1.SidecarState{},
			},
		},
	}, {
		desc: "pending-not-enough-node-resources",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{{
				Reason:  corev1.PodReasonUnschedulable,
				Message: "0/1 nodes are available: 1 Insufficient cpu.",
			}},
		},
		want: v1.TaskRunStatus{
			Status: statusPending(ReasonExceededNodeResources, "TaskRun Pod exceeded available resources"),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps:    []v1.StepState{},
				Sidecars: []v1.SidecarState{},
			},
		},
	}, {
		desc: "pending-CreateContainerConfigError",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodPending,
			ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason: "CreateContainerConfigError",
					},
				},
			}},
		},
		want: v1.TaskRunStatus{
			Status: statusFailure(ReasonCreateContainerConfigError, "Failed to create pod due to config error"),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps:    []v1.StepState{},
				Sidecars: []v1.SidecarState{},
			},
		},
	}, {
		desc: "with-sidecar-running",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "step-running-step",
				State: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{},
				},
			}, {
				Name:    "sidecar-running",
				ImageID: "image-id",
				State: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{},
				},
				Ready: true,
			}},
		},
		want: v1.TaskRunStatus{
			Status: statusRunning(),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
					Name:      "running-step",
					Container: "step-running-step",
				}},
				Sidecars: []v1.SidecarState{{
					ContainerState: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
					Name:      "running",
					ImageID:   "image-id",
					Container: "sidecar-running",
				}},
			},
		},
	}, {
		desc: "with-sidecar-waiting",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "step-waiting-step",
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason:  "PodInitializing",
						Message: "PodInitializing",
					},
				},
			}, {
				Name:    "sidecar-waiting",
				ImageID: "image-id",
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason:  "PodInitializing",
						Message: "PodInitializing",
					},
				},
				Ready: true,
			}},
		},
		want: v1.TaskRunStatus{
			Status: statusRunning(),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason:  "PodInitializing",
							Message: "PodInitializing",
						},
					},
					Name:      "waiting-step",
					Container: "step-waiting-step",
				}},
				Sidecars: []v1.SidecarState{{
					ContainerState: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason:  "PodInitializing",
							Message: "PodInitializing",
						},
					},
					Name:      "waiting",
					ImageID:   "image-id",
					Container: "sidecar-waiting",
				}},
			},
		},
	}, {
		desc: "with-sidecar-terminated",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "step-running-step",
				State: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{},
				},
			}, {
				Name:    "sidecar-error",
				ImageID: "image-id",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 1,
						Reason:   "Error",
						Message:  "Error",
					},
				},
				Ready: true,
			}},
		},
		want: v1.TaskRunStatus{
			Status: statusRunning(),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
					Name:      "running-step",
					Container: "step-running-step",
				}},
				Sidecars: []v1.SidecarState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
							Reason:   "Error",
							Message:  "Error",
						},
					},
					Name:      "error",
					ImageID:   "image-id",
					Container: "sidecar-error",
				}},
			},
		},
	}, {
		desc: "image resource that should not populate resourcesResult",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "step-foo",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"key":"digest","value":"sha256:12345","resourceName":"source-image"}]`,
					},
				},
			}},
		},
		want: v1.TaskRunStatus{
			Status: statusSuccess(),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Message: `[{"key":"digest","value":"sha256:12345","resourceName":"source-image"}]`,
						}},
					Name:      "foo",
					Container: "step-foo",
				}},
				Sidecars: []v1.SidecarState{},
				// We don't actually care about the time, just that it's not nil
				CompletionTime: &metav1.Time{Time: time.Now()},
			},
		},
	}, {
		desc: "test result with pipeline result",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "step-bar",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"key":"resultName","value":"resultValue", "type":1}, {"key":"digest","value":"sha256:1234","resourceName":"source-image"}]`,
					},
				},
			}},
		},
		want: v1.TaskRunStatus{
			Status: statusSuccess(),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Message: `[{"key":"digest","value":"sha256:1234","resourceName":"source-image"},{"key":"resultName","value":"resultValue","type":1}]`,
						}},
					Name:      "bar",
					Container: "step-bar",
				}},
				Sidecars: []v1.SidecarState{},
				Results: []v1.TaskRunResult{{
					Name:  "resultName",
					Type:  v1.ResultsTypeString,
					Value: *v1.NewStructuredValues("resultValue"),
				}},
				// We don't actually care about the time, just that it's not nil
				CompletionTime: &metav1.Time{Time: time.Now()},
			},
		},
	}, {
		desc: "test result with pipeline result - no result type",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "step-banana",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"key":"resultName","value":"resultValue", "type":1}, {"key":"digest","value":"sha256:1234","resourceName":"source-image"}]`,
					},
				},
			}},
		},
		want: v1.TaskRunStatus{
			Status: statusSuccess(),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Message: `[{"key":"digest","value":"sha256:1234","resourceName":"source-image"},{"key":"resultName","value":"resultValue","type":1}]`,
						}},
					Name:      "banana",
					Container: "step-banana",
				}},
				Sidecars: []v1.SidecarState{},
				Results: []v1.TaskRunResult{{
					Name:  "resultName",
					Type:  v1.ResultsTypeString,
					Value: *v1.NewStructuredValues("resultValue"),
				}},
				// We don't actually care about the time, just that it's not nil
				CompletionTime: &metav1.Time{Time: time.Now()},
			},
		},
	}, {
		desc: "two test results",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "step-one",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"key":"resultNameOne","value":"resultValueOne", "type":1}, {"key":"resultNameTwo","value":"resultValueTwo", "type":1}]`,
					},
				},
			}, {
				Name: "step-two",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"key":"resultNameOne","value":"resultValueThree","type":1},{"key":"resultNameTwo","value":"resultValueTwo","type":1}]`,
					},
				},
			}},
		},
		want: v1.TaskRunStatus{
			Status: statusSuccess(),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Message: `[{"key":"resultNameOne","value":"resultValueOne","type":1},{"key":"resultNameTwo","value":"resultValueTwo","type":1}]`,
						}},
					Name:      "one",
					Container: "step-one",
				}, {
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Message: `[{"key":"resultNameOne","value":"resultValueThree","type":1},{"key":"resultNameTwo","value":"resultValueTwo","type":1}]`,
						}},
					Name:      "two",
					Container: "step-two",
				}},
				Sidecars: []v1.SidecarState{},
				Results: []v1.TaskRunResult{{
					Name:  "resultNameOne",
					Type:  v1.ResultsTypeString,
					Value: *v1.NewStructuredValues("resultValueThree"),
				}, {
					Name:  "resultNameTwo",
					Type:  v1.ResultsTypeString,
					Value: *v1.NewStructuredValues("resultValueTwo"),
				}},
				// We don't actually care about the time, just that it's not nil
				CompletionTime: &metav1.Time{Time: time.Now()},
			},
		},
	}, {
		desc: "the failed task show task results",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodFailed,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "step-task-result",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"key":"resultName","value":"resultValue", "type":1}]`,
					},
				},
			}},
		},
		want: v1.TaskRunStatus{
			Status: statusFailure(v1.TaskRunReasonFailed.String(), "build failed for unspecified reasons."),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Message: `[{"key":"resultName","value":"resultValue","type":1}]`,
						},
					},
					Name:      "task-result",
					Container: "step-task-result",
				}},
				Sidecars:       []v1.SidecarState{},
				CompletionTime: &metav1.Time{Time: time.Now()},
				Results: []v1.TaskRunResult{{
					Name:  "resultName",
					Type:  v1.ResultsTypeString,
					Value: *v1.NewStructuredValues("resultValue"),
				}},
			},
		},
	}, {
		desc: "taskrun status set to failed if task fails",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodFailed,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "step-mango",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{},
				},
			}},
		},
		want: v1.TaskRunStatus{
			Status: statusFailure(v1.TaskRunReasonFailed.String(), "build failed for unspecified reasons."),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{}},
					Name:      "mango",
					Container: "step-mango",
				}},
				Sidecars: []v1.SidecarState{},
				// We don't actually care about the time, just that it's not nil
				CompletionTime: &metav1.Time{Time: time.Now()},
			},
		},
	}, {
		desc: "termination message not adhering to RunResult format is filtered from taskrun termination message",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "step-pineapple",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"invalid":"resultName","invalid":"resultValue"}]`,
					},
				},
			}},
		},
		want: v1.TaskRunStatus{
			Status: statusSuccess(),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{}},
					Name:      "pineapple",
					Container: "step-pineapple",
				}},
				Sidecars: []v1.SidecarState{},
				// We don't actually care about the time, just that it's not nil
				CompletionTime: &metav1.Time{Time: time.Now()},
			},
		},
	}, {
		desc: "filter internaltektonresult",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "step-pear",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"key":"resultNameOne","value":"","type":3}, {"key":"resultNameThree","value":"","type":1}]`},
				},
			}},
		},
		want: v1.TaskRunStatus{
			Status: statusSuccess(),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Message: `[{"key":"resultNameThree","value":"","type":1}]`,
						}},
					Name:      "pear",
					Container: "step-pear",
				}},
				Sidecars: []v1.SidecarState{},
				Results: []v1.TaskRunResult{{
					Name:  "resultNameThree",
					Type:  v1.ResultsTypeString,
					Value: *v1.NewStructuredValues(""),
				}},
				// We don't actually care about the time, just that it's not nil
				CompletionTime: &metav1.Time{Time: time.Now()},
			},
		},
	}, {
		desc: "filter internaltektonresult with `type` as string",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "step-pear",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"key":"resultNameTwo","value":"","type":"InternalTektonResult"}, {"key":"resultNameThree","value":"","type":"TaskRunResult"}]`,
					},
				},
			}},
		},
		want: v1.TaskRunStatus{
			Status: statusSuccess(),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Message: `[{"key":"resultNameThree","value":"","type":1}]`,
						}},
					Name:      "pear",
					Container: "step-pear",
				}},
				Sidecars: []v1.SidecarState{},
				Results: []v1.TaskRunResult{{
					Name:  "resultNameThree",
					Type:  v1.ResultsTypeString,
					Value: *v1.NewStructuredValues(""),
				}},
				// We don't actually care about the time, just that it's not nil
				CompletionTime: &metav1.Time{Time: time.Now()},
			},
		},
	}, {
		desc: "correct TaskRun status step order regardless of pod container status order",
		pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name: "step-first",
				}, {
					Name: "step-second",
				}, {
					Name: "step-third",
				}, {
					Name: "step-",
				}, {
					Name: "step-fourth",
				}},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodSucceeded,
				ContainerStatuses: []corev1.ContainerStatus{{
					Name: "step-second",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{},
					},
				}, {
					Name: "step-fourth",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{},
					},
				}, {
					Name: "step-",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{},
					},
				}, {
					Name: "step-first",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{},
					},
				}, {
					Name: "step-third",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{},
					},
				}},
			},
		},
		want: v1.TaskRunStatus{
			Status: statusSuccess(),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{},
					},
					Name:      "first",
					Container: "step-first",
				}, {
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{}},
					Name:      "second",
					Container: "step-second",
				}, {
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{}},
					Name:      "third",
					Container: "step-third",
				}, {
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{}},
					Name:      "",
					Container: "step-",
				}, {
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{}},
					Name:      "fourth",
					Container: "step-fourth",
				}},
				Sidecars: []v1.SidecarState{},
				// We don't actually care about the time, just that it's not nil
				CompletionTime: &metav1.Time{Time: time.Now()},
			},
		},
	}, {
		desc: "include non zero exit code in a container termination message if entrypoint is set to ignore the error",
		pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name: "step-first",
				}, {
					Name: "step-second",
				}},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodSucceeded,
				ContainerStatuses: []corev1.ContainerStatus{{
					Name: "step-first",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Message: `[{"key":"ExitCode","value":"11","type":"InternalTektonResult"}]`,
						},
					},
				}, {
					Name: "step-second",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{},
					},
				}},
			},
		},
		want: v1.TaskRunStatus{
			Status: statusSuccess(),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 11,
						},
					},
					Name:      "first",
					Container: "step-first",
				}, {
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 0,
						}},
					Name:      "second",
					Container: "step-second",
				}},
				Sidecars: []v1.SidecarState{},
				// We don't actually care about the time, just that it's not nil
				CompletionTime: &metav1.Time{Time: time.Now()},
			},
		},
	}, {
		desc: "when pod is pending because of pulling image then the error should bubble up to taskrun status",
		pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name: "step-first",
				}},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodPending,
				ContainerStatuses: []corev1.ContainerStatus{{
					Name: "step-first",
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason:  "ImagePullBackOff",
							Message: "Back-off pulling image xxxxxxx",
						},
					},
				}},
			},
		},
		want: v1.TaskRunStatus{
			Status: statusPending("PullImageFailed", `build step "step-first" is pending with reason "Back-off pulling image xxxxxxx"`),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason:  "ImagePullBackOff",
							Message: "Back-off pulling image xxxxxxx",
						},
					},
					Name:      "first",
					Container: "step-first",
				}},
				Sidecars: []v1.SidecarState{},
			},
		},
	}, {
		desc: "report init container err message when container from step PodInitializing and init container failed",
		pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod",
				Namespace: "foo",
			},
			Spec: corev1.PodSpec{
				InitContainers: []corev1.Container{{
					Name: "init-A",
				}},
				Containers: []corev1.Container{{
					Name: "step-A",
				}},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodFailed,
				InitContainerStatuses: []corev1.ContainerStatus{{
					Name:    "init-A",
					ImageID: "init-image-id-A",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
						}},
				}},
				ContainerStatuses: []corev1.ContainerStatus{{
					Name:    "step-A",
					ImageID: "image-id-A",
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason: "PodInitializing",
						},
					},
				}},
			},
		},
		want: v1.TaskRunStatus{
			Status: statusFailure(v1.TaskRunReasonFailed.String(), "init container failed, \"init-A\" exited with code 1 (image: \"init-image-id-A\"); for logs run: kubectl -n foo logs pod -c init-A\n"),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason: "PodInitializing",
						},
					},
					Name:      "A",
					Container: "step-A",
					ImageID:   "image-id-A",
				}},
				Sidecars:       []v1.SidecarState{},
				CompletionTime: &metav1.Time{Time: time.Now()},
			},
		},
	}, {
		desc: "report pod evicted err message when pod was evicted and containers have failure messages",
		pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod",
				Namespace: "foo",
			},
			Spec: corev1.PodSpec{
				InitContainers: []corev1.Container{{
					Name: "init-A",
				}},
				Containers: []corev1.Container{{
					Name: "step-A",
				}},
			},
			Status: corev1.PodStatus{
				Phase:   corev1.PodFailed,
				Reason:  "Evicted",
				Message: `Usage of EmptyDir volume "ws-b6dfk" exceeds the limit "10Gi".`,
				ContainerStatuses: []corev1.ContainerStatus{{
					Name: "step-A",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 137,
						},
					},
				},
				}},
		},
		want: v1.TaskRunStatus{
			Status: statusFailure(v1.TaskRunReasonFailed.String(), "Usage of EmptyDir volume \"ws-b6dfk\" exceeds the limit \"10Gi\"."),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 137,
						}},
					Name:      "A",
					Container: "step-A",
				}},
				Sidecars:       []v1.SidecarState{},
				CompletionTime: &metav1.Time{Time: time.Now()},
			},
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			now := metav1.Now()
			if cmp.Diff(c.pod, corev1.Pod{}) == "" {
				c.pod = corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "pod",
						Namespace:         "foo",
						CreationTimestamp: now,
					},
					Status: c.podStatus,
				}
			}

			startTime := time.Date(2010, 1, 1, 1, 1, 1, 1, time.UTC)
			tr := v1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "task-run",
					Namespace: "foo",
				},
				Status: v1.TaskRunStatus{
					TaskRunStatusFields: v1.TaskRunStatusFields{
						StartTime: &metav1.Time{Time: startTime},
					},
				},
			}
			logger, _ := logging.NewLogger("", "status")
			kubeclient := fakek8s.NewSimpleClientset()
			got, err := MakeTaskRunStatus(context.Background(), logger, tr, &c.pod, kubeclient, &v1.TaskSpec{})
			if err != nil {
				t.Errorf("MakeTaskRunResult: %s", err)
			}

			// Common traits, set for test case brevity.
			c.want.PodName = "pod"
			c.want.StartTime = &metav1.Time{Time: startTime}

			ensureTimeNotNil := cmp.Comparer(func(x, y *metav1.Time) bool {
				if x == nil {
					return y == nil
				}
				return y != nil
			})
			if d := cmp.Diff(c.want, got, ignoreVolatileTime, ensureTimeNotNil); d != "" {
				t.Errorf("Diff %s", diff.PrintWantGot(d))
			}
			if tr.Status.StartTime.Time != c.want.StartTime.Time {
				t.Errorf("Expected TaskRun startTime to be unchanged but was %s", tr.Status.StartTime)
			}
		})
	}
}

func TestMakeTaskRunStatusAlpha(t *testing.T) {
	for _, c := range []struct {
		desc      string
		podStatus corev1.PodStatus
		pod       corev1.Pod
		taskSpec  v1.TaskSpec
		want      v1.TaskRunStatus
	}{{
		desc: "test empty result",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "step-bar",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"key":"resultName","value":"", "type":1}, {"key":"digest","value":"sha256:1234","resourceName":"source-image"}]`,
					},
				},
			}},
		},
		taskSpec: v1.TaskSpec{
			Results: []v1.TaskResult{
				{
					Name: "resultName",
					Type: v1.ResultsTypeString,
				},
			},
		},
		want: v1.TaskRunStatus{
			Status: statusSuccess(),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Message: `[{"key":"digest","value":"sha256:1234","resourceName":"source-image"},{"key":"resultName","value":"","type":1}]`,
						}},
					Name:      "bar",
					Container: "step-bar",
				}},
				Sidecars: []v1.SidecarState{},
				Results: []v1.TaskRunResult{{
					Name:  "resultName",
					Type:  v1.ResultsTypeString,
					Value: *v1.NewStructuredValues(""),
				}},
				// We don't actually care about the time, just that it's not nil
				CompletionTime: &metav1.Time{Time: time.Now()},
			},
		},
	}, {
		desc: "test string result",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "step-bar",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"key":"resultName","value":"hello", "type":1}, {"key":"digest","value":"sha256:1234","resourceName":"source-image"}]`,
					},
				},
			}},
		},
		taskSpec: v1.TaskSpec{
			Results: []v1.TaskResult{
				{
					Name: "resultName",
					Type: v1.ResultsTypeString,
				},
			},
		},
		want: v1.TaskRunStatus{
			Status: statusSuccess(),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Message: `[{"key":"digest","value":"sha256:1234","resourceName":"source-image"},{"key":"resultName","value":"hello","type":1}]`,
						}},
					Name:      "bar",
					Container: "step-bar",
				}},
				Sidecars: []v1.SidecarState{},
				Results: []v1.TaskRunResult{{
					Name:  "resultName",
					Type:  v1.ResultsTypeString,
					Value: *v1.NewStructuredValues("hello"),
				}},
				// We don't actually care about the time, just that it's not nil
				CompletionTime: &metav1.Time{Time: time.Now()},
			},
		},
	}, {
		desc: "test array result",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "step-bar",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"key":"resultName","value":"[\"hello\",\"world\"]", "type":1}, {"key":"digest","value":"sha256:1234","resourceName":"source-image"}]`,
					},
				},
			}},
		},
		taskSpec: v1.TaskSpec{
			Results: []v1.TaskResult{
				{
					Name: "resultName",
					Type: v1.ResultsTypeArray,
				},
			},
		},
		want: v1.TaskRunStatus{
			Status: statusSuccess(),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Message: `[{"key":"digest","value":"sha256:1234","resourceName":"source-image"},{"key":"resultName","value":"[\"hello\",\"world\"]","type":1}]`,
						}},
					Name:      "bar",
					Container: "step-bar",
				}},
				Sidecars: []v1.SidecarState{},
				Results: []v1.TaskRunResult{{
					Name:  "resultName",
					Type:  v1.ResultsTypeArray,
					Value: *v1.NewStructuredValues("hello", "world"),
				}},
				// We don't actually care about the time, just that it's not nil
				CompletionTime: &metav1.Time{Time: time.Now()},
			},
		},
	}, {
		desc: "test object result",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "step-bar",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"key":"resultName","value":"{\"hello\":\"world\"}", "type":1}, {"key":"digest","value":"sha256:1234","resourceName":"source-image"}]`,
					},
				},
			}},
		},
		taskSpec: v1.TaskSpec{
			Results: []v1.TaskResult{
				{
					Name: "resultName",
					Type: v1.ResultsTypeObject,
				},
			},
		},
		want: v1.TaskRunStatus{
			Status: statusSuccess(),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Message: `[{"key":"digest","value":"sha256:1234","resourceName":"source-image"},{"key":"resultName","value":"{\"hello\":\"world\"}","type":1}]`,
						}},
					Name:      "bar",
					Container: "step-bar",
				}},
				Sidecars: []v1.SidecarState{},
				Results: []v1.TaskRunResult{{
					Name:  "resultName",
					Type:  v1.ResultsTypeObject,
					Value: *v1.NewObject(map[string]string{"hello": "world"}),
				}},
				// We don't actually care about the time, just that it's not nil
				CompletionTime: &metav1.Time{Time: time.Now()},
			},
		},
	}, {
		desc: "test string result with json emitted results",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "step-bar",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"key":"resultName","value":"{\"hello\":\"world\"}", "type":1}, {"key":"resultName2","value":"[\"hello\",\"world\"]", "type":1}]`,
					},
				},
			}},
		},
		taskSpec: v1.TaskSpec{
			Results: []v1.TaskResult{
				{
					Name: "resultName",
					Type: v1.ResultsTypeString,
				},
				{
					Name: "resultName2",
					Type: v1.ResultsTypeString,
				},
			},
		},
		want: v1.TaskRunStatus{
			Status: statusSuccess(),
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Steps: []v1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Message: `[{"key":"resultName","value":"{\"hello\":\"world\"}","type":1},{"key":"resultName2","value":"[\"hello\",\"world\"]","type":1}]`,
						}},
					Name:      "bar",
					Container: "step-bar",
				}},
				Sidecars: []v1.SidecarState{},
				Results: []v1.TaskRunResult{{
					Name:  "resultName",
					Type:  v1.ResultsTypeString,
					Value: *v1.NewStructuredValues("{\"hello\":\"world\"}"),
				}, {
					Name:  "resultName2",
					Type:  v1.ResultsTypeString,
					Value: *v1.NewStructuredValues("[\"hello\",\"world\"]"),
				}},
				// We don't actually care about the time, just that it's not nil
				CompletionTime: &metav1.Time{Time: time.Now()},
			},
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			now := metav1.Now()
			if cmp.Diff(c.pod, corev1.Pod{}) == "" {
				c.pod = corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "pod",
						Namespace:         "foo",
						CreationTimestamp: now,
					},
					Status: c.podStatus,
				}
			}

			startTime := time.Date(2010, 1, 1, 1, 1, 1, 1, time.UTC)
			tr := v1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "task-run",
					Namespace: "foo",
				},
				Status: v1.TaskRunStatus{
					TaskRunStatusFields: v1.TaskRunStatusFields{
						StartTime: &metav1.Time{Time: startTime},
					},
				},
			}
			logger, _ := logging.NewLogger("", "status")
			kubeclient := fakek8s.NewSimpleClientset()
			got, err := MakeTaskRunStatus(context.Background(), logger, tr, &c.pod, kubeclient, &c.taskSpec)
			if err != nil {
				t.Errorf("MakeTaskRunResult: %s", err)
			}

			// Common traits, set for test case brevity.
			c.want.PodName = "pod"
			c.want.StartTime = &metav1.Time{Time: startTime}

			ensureTimeNotNil := cmp.Comparer(func(x, y *metav1.Time) bool {
				if x == nil {
					return y == nil
				}
				return y != nil
			})
			if d := cmp.Diff(c.want, got, ignoreVolatileTime, ensureTimeNotNil); d != "" {
				t.Errorf("Diff %s", diff.PrintWantGot(d))
			}
			if tr.Status.StartTime.Time != c.want.StartTime.Time {
				t.Errorf("Expected TaskRun startTime to be unchanged but was %s", tr.Status.StartTime)
			}
		})
	}
}

func TestMakeRunStatusJSONError(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod",
			Namespace: "foo",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "step-non-json",
			}, {
				Name: "step-after-non-json",
			}, {
				Name: "step-this-step-might-panic",
			}, {
				Name: "step-foo",
			}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodFailed,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:    "step-this-step-might-panic",
				ImageID: "image",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{},
				},
			}, {
				Name:    "step-foo",
				ImageID: "image",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{},
				},
			}, {
				Name:    "step-non-json",
				ImageID: "image",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 1,
						Message:  "this is a non-json termination message. dont panic!",
					},
				},
			}, {
				Name:    "step-after-non-json",
				ImageID: "image",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{},
				},
			}},
		},
	}
	wantTr := v1.TaskRunStatus{
		Status: statusFailure(v1.TaskRunReasonFailed.String(), "\"step-non-json\" exited with code 1 (image: \"image\"); for logs run: kubectl -n foo logs pod -c step-non-json\n"),
		TaskRunStatusFields: v1.TaskRunStatusFields{
			PodName: "pod",
			Steps: []v1.StepState{{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 1,
						Message:  "this is a non-json termination message. dont panic!",
					}},
				Name:      "non-json",
				Container: "step-non-json",
				ImageID:   "image",
			}, {
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{}},
				Name:      "after-non-json",
				Container: "step-after-non-json",
				ImageID:   "image",
			}, {
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{}},
				Name:      "this-step-might-panic",
				Container: "step-this-step-might-panic",
				ImageID:   "image",
			}, {
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{}},
				Name:      "foo",
				Container: "step-foo",
				ImageID:   "image",
			}},
			Sidecars: []v1.SidecarState{},
			// We don't actually care about the time, just that it's not nil
			CompletionTime: &metav1.Time{Time: time.Now()},
		},
	}
	tr := v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "task-run",
			Namespace: "foo",
		},
	}

	logger, _ := logging.NewLogger("", "status")
	kubeclient := fakek8s.NewSimpleClientset()
	gotTr, err := MakeTaskRunStatus(context.Background(), logger, tr, pod, kubeclient, &v1.TaskSpec{})
	if err == nil {
		t.Error("Expected error, got nil")
	}

	ensureTimeNotNil := cmp.Comparer(func(x, y *metav1.Time) bool {
		if x == nil {
			return y == nil
		}
		return y != nil
	})
	if d := cmp.Diff(wantTr, gotTr, ignoreVolatileTime, ensureTimeNotNil); d != "" {
		t.Errorf("Diff %s", diff.PrintWantGot(d))
	}
}

func TestSidecarsReady(t *testing.T) {
	for _, c := range []struct {
		desc     string
		statuses []corev1.ContainerStatus
		want     bool
	}{{
		desc: "no sidecars",
		statuses: []corev1.ContainerStatus{
			{Name: "step-ignore-me"},
			{Name: "step-ignore-me"},
			{Name: "step-ignore-me"},
		},
		want: true,
	}, {
		desc: "both sidecars ready",
		statuses: []corev1.ContainerStatus{
			{Name: "step-ignore-me"},
			{
				Name:  "sidecar-bar",
				Ready: true,
				State: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{
						StartedAt: metav1.NewTime(time.Now()),
					},
				},
			},
			{Name: "step-ignore-me"},
			{
				Name: "sidecar-stopped-baz",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 99,
					},
				},
			},
			{Name: "step-ignore-me"},
		},
		want: true,
	}, {
		desc: "one sidecar ready, one not running",
		statuses: []corev1.ContainerStatus{
			{Name: "step-ignore-me"},
			{
				Name:  "sidecar-ready",
				Ready: true,
				State: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{
						StartedAt: metav1.NewTime(time.Now()),
					},
				},
			},
			{Name: "step-ignore-me"},
			{
				Name: "sidecar-unready",
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{},
				},
			},
			{Name: "step-ignore-me"},
		},
		want: false,
	}, {
		desc: "one sidecar running but not ready",
		statuses: []corev1.ContainerStatus{
			{Name: "step-ignore-me"},
			{
				Name:  "sidecar-running-not-ready",
				Ready: false, // Not ready.
				State: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{
						StartedAt: metav1.NewTime(time.Now()),
					},
				},
			},
			{Name: "step-ignore-me"},
		},
		want: false,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			got := SidecarsReady(corev1.PodStatus{
				Phase:             corev1.PodRunning,
				ContainerStatuses: c.statuses,
			})
			if got != c.want {
				t.Errorf("SidecarsReady got %t, want %t", got, c.want)
			}
		})
	}
}

func TestMarkStatusRunning(t *testing.T) {
	trs := v1.TaskRunStatus{}
	markStatusRunning(&trs, v1.TaskRunReasonRunning.String(), "Not all Steps in the Task have finished executing")

	expected := &apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionUnknown,
		Reason:  v1.TaskRunReasonRunning.String(),
		Message: "Not all Steps in the Task have finished executing",
	}

	if d := cmp.Diff(expected, trs.GetCondition(apis.ConditionSucceeded), cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime.Inner.Time")); d != "" {
		t.Errorf("Unexpected status: %s", diff.PrintWantGot(d))
	}
}

func TestMarkStatusFailure(t *testing.T) {
	trs := v1.TaskRunStatus{}
	markStatusFailure(&trs, v1.TaskRunReasonFailed.String(), "failure message")

	expected := &apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionFalse,
		Reason:  v1.TaskRunReasonFailed.String(),
		Message: "failure message",
	}

	if d := cmp.Diff(expected, trs.GetCondition(apis.ConditionSucceeded), cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime.Inner.Time")); d != "" {
		t.Errorf("Unexpected status: %s", diff.PrintWantGot(d))
	}
}

func TestMarkStatusSuccess(t *testing.T) {
	trs := v1.TaskRunStatus{}
	markStatusSuccess(&trs)

	expected := &apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionTrue,
		Reason:  v1.TaskRunReasonSuccessful.String(),
		Message: "All Steps have completed executing",
	}

	if d := cmp.Diff(expected, trs.GetCondition(apis.ConditionSucceeded), cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime.Inner.Time")); d != "" {
		t.Errorf("Unexpected status: %s", diff.PrintWantGot(d))
	}
}

func TestIsPodArchived(t *testing.T) {
	for _, tc := range []struct {
		name          string
		podName       string
		retriesStatus []v1.TaskRunStatus
		want          bool
	}{{
		name:          "Pod is not in the empty retriesStatus",
		podName:       "pod",
		retriesStatus: []v1.TaskRunStatus{},
		want:          false,
	}, {
		name:    "Pod is not in the non-empty retriesStatus",
		podName: "pod-retry1",
		retriesStatus: []v1.TaskRunStatus{{
			TaskRunStatusFields: v1.TaskRunStatusFields{
				PodName: "pod",
			},
		}},
		want: false,
	}, {
		name:    "Pod is in the retriesStatus",
		podName: "pod",
		retriesStatus: []v1.TaskRunStatus{{
			TaskRunStatusFields: v1.TaskRunStatusFields{
				PodName: "pod",
			}},
		},
		want: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			trs := v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					PodName:       "pod",
					RetriesStatus: tc.retriesStatus,
				},
			}
			got := IsPodArchived(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: tc.podName}}, &trs)
			if tc.want != got {
				t.Errorf("got: %v, want: %v", got, tc.want)
			}
		})
	}
}

func statusRunning() duckv1.Status {
	var trs v1.TaskRunStatus
	markStatusRunning(&trs, v1.TaskRunReasonRunning.String(), "Not all Steps in the Task have finished executing")
	return trs.Status
}

func statusFailure(reason, message string) duckv1.Status {
	var trs v1.TaskRunStatus
	markStatusFailure(&trs, reason, message)
	return trs.Status
}

func statusSuccess() duckv1.Status {
	var trs v1.TaskRunStatus
	markStatusSuccess(&trs)
	return trs.Status
}

func statusPending(reason, message string) duckv1.Status {
	return duckv1.Status{
		Conditions: []apis.Condition{{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionUnknown,
			Reason:  reason,
			Message: message,
		}},
	}
}

// TestSortPodContainerStatuses checks that a complex list of container statuses is
// sorted in the way that we expect them to be according to their step order. This
// exercises a specific failure mode we observed where the image-digest-exporter
// status was inserted before user step statuses, which is not a valid ordering.
// See github issue https://github.com/tektoncd/pipeline/issues/3677 for the full
// details of the bug.
func TestSortPodContainerStatuses(t *testing.T) {
	containerNames := []string{
		"step-create-dir-notification-g2fjb",
		"step-create-dir-builtgcsfetcherimage-68lnn",
		"step-create-dir-builtpullrequestinitimage-nr6gc",
		"step-create-dir-builtdigestexporterimage-mlj2j",
		"step-create-dir-builtwebhookimage-zldcx",
		"step-create-dir-builtcontrollerimage-4nncs",
		"step-create-dir-builtgitinitimage-jbdwk",
		"step-create-dir-builtcredsinitimage-mtrcp",
		"step-create-dir-builtnopimage-c9k2k",
		"step-create-dir-builtentrypointimage-dd7vw",
		"step-create-dir-bucket-jr2lk",
		"step-git-source-source-fkrcz",
		"step-create-dir-bucket-gtw4k",
		"step-fetch-bucket-llqdh",
		"step-create-ko-yaml",
		"step-link-input-bucket-to-output",
		"step-ensure-release-dir-exists",
		"step-run-ko",
		"step-copy-to-latest-bucket",
		"step-tag-images",
		"step-image-digest-exporter-vcqhc",
		"step-source-mkdir-bucket-ljkcz",
		"step-source-copy-bucket-5mwpq",
		"step-upload-bucket-kt9b4",
	}
	containerStatusNames := []string{
		"step-copy-to-latest-bucket",
		"step-create-dir-bucket-gtw4k",
		"step-create-dir-bucket-jr2lk",
		"step-create-dir-builtcontrollerimage-4nncs",
		"step-create-dir-builtcredsinitimage-mtrcp",
		"step-create-dir-builtdigestexporterimage-mlj2j",
		"step-create-dir-builtentrypointimage-dd7vw",
		"step-create-dir-builtgcsfetcherimage-68lnn",
		"step-create-dir-builtgitinitimage-jbdwk",
		"step-create-dir-builtnopimage-c9k2k",
		"step-create-dir-builtpullrequestinitimage-nr6gc",
		"step-create-dir-builtwebhookimage-zldcx",
		"step-create-dir-notification-g2fjb",
		"step-create-ko-yaml",
		"step-ensure-release-dir-exists",
		"step-fetch-bucket-llqdh",
		"step-git-source-source-fkrcz",
		"step-image-digest-exporter-vcqhc",
		"step-link-input-bucket-to-output",
		"step-run-ko",
		"step-source-copy-bucket-5mwpq",
		"step-source-mkdir-bucket-ljkcz",
		"step-tag-images",
		"step-upload-bucket-kt9b4",
	}
	containers := []corev1.Container{}
	statuses := []corev1.ContainerStatus{}
	for _, s := range containerNames {
		containers = append(containers, corev1.Container{
			Name: s,
		})
	}
	for _, s := range containerStatusNames {
		statuses = append(statuses, corev1.ContainerStatus{
			Name: s,
		})
	}
	sortPodContainerStatuses(statuses, containers)
	for i := range statuses {
		if statuses[i].Name != containers[i].Name {
			t.Errorf("container status out of order: want %q got %q", containers[i].Name, statuses[i].Name)
		}
	}
}
