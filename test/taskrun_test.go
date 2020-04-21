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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

func TestTaskRunFailure(t *testing.T) {
	c, namespace := setup(t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	taskRunName := "failing-taskrun"

	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	task := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: "failing-task", Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Image:   "busybox",
				Command: []string{"/bin/sh"},
				Args:    []string{"-c", "echo hello"},
			}}, {Container: corev1.Container{
				Image:   "busybox",
				Command: []string{"/bin/sh"},
				Args:    []string{"-c", "exit 1"},
			}}, {Container: corev1.Container{
				Image:   "busybox",
				Command: []string{"/bin/sh"},
				Args:    []string{"-c", "sleep 30s"},
			}}},
		},
	}
	if _, err := c.TaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}
	taskRun := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: taskRunName, Namespace: namespace},
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{Name: "failing-task"},
		},
	}
	if _, err := c.TaskRunClient.Create(taskRun); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to fail", namespace)
	if err := WaitForTaskRunState(c, taskRunName, TaskRunFailed(taskRunName), "TaskRunFailed"); err != nil {
		t.Errorf("Error waiting for TaskRun to finish: %s", err)
	}

	taskrun, err := c.TaskRunClient.Get(taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
	}

	expectedStepState := []v1beta1.StepState{{
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 0,
				Reason:   "Completed",
			},
		},
		Name:          "unnamed-0",
		ContainerName: "step-unnamed-0",
	}, {
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 1,
				Reason:   "Error",
			},
		},
		Name:          "unnamed-1",
		ContainerName: "step-unnamed-1",
	}, {
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 1,
				Reason:   "Error",
			},
		},
		Name:          "unnamed-2",
		ContainerName: "step-unnamed-2",
	}}
	ignoreTerminatedFields := cmpopts.IgnoreFields(corev1.ContainerStateTerminated{}, "StartedAt", "FinishedAt", "ContainerID")
	ignoreStepFields := cmpopts.IgnoreFields(v1beta1.StepState{}, "ImageID")
	if d := cmp.Diff(taskrun.Status.Steps, expectedStepState, ignoreTerminatedFields, ignoreStepFields); d != "" {
		t.Fatalf("-got, +want: %v", d)
	}
}

func TestTaskRunStatus(t *testing.T) {
	c, namespace := setup(t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	taskRunName := "status-taskrun"

	fqImageName := "busybox@sha256:895ab622e92e18d6b461d671081757af7dbaa3b00e3e28e12505af7817f73649"
	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	task := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: "status-task", Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			// This was the digest of the latest tag as of 8/12/2019
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Image:   "busybox@sha256:895ab622e92e18d6b461d671081757af7dbaa3b00e3e28e12505af7817f73649",
				Command: []string{"/bin/sh"},
				Args:    []string{"-c", "echo hello"},
			}}},
		},
	}
	if _, err := c.TaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}
	taskRun := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: taskRunName, Namespace: namespace},
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{Name: "status-task"},
		},
	}
	if _, err := c.TaskRunClient.Create(taskRun); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to fail", namespace)
	if err := WaitForTaskRunState(c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunSucceed"); err != nil {
		t.Errorf("Error waiting for TaskRun to finish: %s", err)
	}

	taskrun, err := c.TaskRunClient.Get(taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
	}

	expectedStepState := []v1beta1.StepState{{
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 0,
				Reason:   "Completed",
			},
		},
		Name:          "unnamed-0",
		ContainerName: "step-unnamed-0",
	}}

	ignoreTerminatedFields := cmpopts.IgnoreFields(corev1.ContainerStateTerminated{}, "StartedAt", "FinishedAt", "ContainerID")
	ignoreStepFields := cmpopts.IgnoreFields(v1beta1.StepState{}, "ImageID")
	if d := cmp.Diff(taskrun.Status.Steps, expectedStepState, ignoreTerminatedFields, ignoreStepFields); d != "" {
		t.Fatalf("-got, +want: %v", d)
	}
	// Note(chmouel): Sometime we have docker-pullable:// or docker.io/library as prefix, so let only compare the suffix
	if !strings.HasSuffix(taskrun.Status.Steps[0].ImageID, fqImageName) {
		t.Fatalf("`ImageID: %s` does not end with `%s`", taskrun.Status.Steps[0].ImageID, fqImageName)
	}

	if d := cmp.Diff(taskrun.Status.TaskSpec, &task.Spec); d != "" {
		t.Fatalf("-got, +want: %v", d)
	}
}

func TestTaskRunClusterTaskWithPermissions(t *testing.T) {
	c, namespace := setup(t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	taskRunName := "taskrun-to-clustertask-with-perms"
	clusterTaskName := "status-cluster-task-with-perms"

	fqImageName := "busybox@sha256:895ab622e92e18d6b461d671081757af7dbaa3b00e3e28e12505af7817f73649"
	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	task := &v1beta1.ClusterTask{
		ObjectMeta: metav1.ObjectMeta{Name: clusterTaskName, Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			// This was the digest of the latest tag as of 8/12/2019
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Image:   "busybox@sha256:895ab622e92e18d6b461d671081757af7dbaa3b00e3e28e12505af7817f73649",
				Command: []string{"/bin/sh"},
				Args:    []string{"-c", "echo hello"},
			}}},
		},
	}
	if _, err := c.ClusterTaskClient.Get(clusterTaskName, metav1.GetOptions{}); err != nil && !kerrors.IsNotFound(err) {
		t.Fatalf("Unexpected error on get of clustertask: %s", err)
	} else if _, err := c.ClusterTaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}
	defer c.ClusterTaskClient.Delete(task.Name, &metav1.DeleteOptions{})

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: "taskrun-to-clustertask", Namespace: namespace},
		Rules: []rbacv1.PolicyRule{{
			APIGroups: []string{"tekton.dev"},
			Resources: []string{"clustertasks"},
			Verbs:     []string{"get", "list", "watch"},
		}},
	}
	if _, err := c.KubeClient.Kube.RbacV1().Roles(namespace).Create(role); err != nil {
		t.Fatalf("error creating clustertask role %s", err)
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "taskrun-to-cluster-task", Namespace: namespace},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "taskrun-to-clustertask",
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "default",
			Namespace: namespace,
		}},
	}
	if _, err := c.KubeClient.Kube.RbacV1().RoleBindings(namespace).Create(roleBinding); err != nil {
		t.Fatalf("error creating clustertask rolebinding %s", err)
	}

	taskRun := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: taskRunName, Namespace: namespace},
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{Name: clusterTaskName, Kind: "ClusterTask"},
		},
	}
	if _, err := c.TaskRunClient.Create(taskRun); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to fail", namespace)
	if err := WaitForTaskRunState(c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunSucceed"); err != nil {
		t.Errorf("Error waiting for TaskRun to finish: %s", err)
	}

	taskrun, err := c.TaskRunClient.Get(taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
	}

	expectedStepState := []v1beta1.StepState{{
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 0,
				Reason:   "Completed",
			},
		},
		Name:          "unnamed-0",
		ContainerName: "step-unnamed-0",
	}}

	ignoreTerminatedFields := cmpopts.IgnoreFields(corev1.ContainerStateTerminated{}, "StartedAt", "FinishedAt", "ContainerID")
	ignoreStepFields := cmpopts.IgnoreFields(v1beta1.StepState{}, "ImageID")
	if d := cmp.Diff(taskrun.Status.Steps, expectedStepState, ignoreTerminatedFields, ignoreStepFields); d != "" {
		t.Fatalf("-got, +want: %v", d)
	}
	// Note(chmouel): Sometime we have docker-pullable:// or docker.io/library as prefix, so let only compare the suffix
	if !strings.HasSuffix(taskrun.Status.Steps[0].ImageID, fqImageName) {
		t.Fatalf("`ImageID: %s` does not end with `%s`", taskrun.Status.Steps[0].ImageID, fqImageName)
	}

}

func TestTaskRunClusterTaskWithoutPermissions(t *testing.T) {
	c, namespace := setup(t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	taskRunName := "taskrun-to-clustertask-without-perms"
	clusterTaskName := "status-cluster-task-without-perms"

	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	task := &v1beta1.ClusterTask{
		ObjectMeta: metav1.ObjectMeta{Name: clusterTaskName, Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			// This was the digest of the latest tag as of 8/12/2019
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Image:   "busybox@sha256:895ab622e92e18d6b461d671081757af7dbaa3b00e3e28e12505af7817f73649",
				Command: []string{"/bin/sh"},
				Args:    []string{"-c", "echo hello"},
			}}},
		},
	}
	if _, err := c.ClusterTaskClient.Get(clusterTaskName, metav1.GetOptions{}); err != nil && !kerrors.IsNotFound(err) {
		t.Fatalf("Unexpected error on get of clustertask: %s", err)
	} else if _, err := c.ClusterTaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}
	defer c.ClusterTaskClient.Delete(task.Name, &metav1.DeleteOptions{})

	taskRun := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: taskRunName, Namespace: namespace},
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{Name: clusterTaskName, Kind: "ClusterTask"},
		},
	}
	if _, err := c.TaskRunClient.Create(taskRun); err == nil {
		t.Fatalf("Should have failed to create TaskRun: %s", err)
	}

	/*t.Logf("Waiting for TaskRun in namespace %s to fail", namespace)
	if err := WaitForTaskRunState(c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunSucceed"); err == nil {
		taskrun, _ := c.TaskRunClient.Get(taskRunName, metav1.GetOptions{})
		t.Fatalf("should have errored out: %#v", taskrun)
	}

	taskrun, err := c.TaskRunClient.Get(taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
	}

	expectedStepState := []v1beta1.StepState{{
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 0,
				Reason:   "Completed",
			},
		},
		Name:          "unnamed-0",
		ContainerName: "step-unnamed-0",
	}}

	ignoreTerminatedFields := cmpopts.IgnoreFields(corev1.ContainerStateTerminated{}, "StartedAt", "FinishedAt", "ContainerID")
	ignoreStepFields := cmpopts.IgnoreFields(v1beta1.StepState{}, "ImageID")
	if d := cmp.Diff(taskrun.Status.Steps, expectedStepState, ignoreTerminatedFields, ignoreStepFields); d != "" {
		t.Fatalf("-got, +want: %v", d)
	}
	// Note(chmouel): Sometime we have docker-pullable:// or docker.io/library as prefix, so let only compare the suffix
	if !strings.HasSuffix(taskrun.Status.Steps[0].ImageID, fqImageName) {
		t.Fatalf("`ImageID: %s` does not end with `%s`", taskrun.Status.Steps[0].ImageID, fqImageName)
	}*/

}
