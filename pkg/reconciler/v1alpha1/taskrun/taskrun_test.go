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

package taskrun_test

import (
	"context"
	"testing"

	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/test"
	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

var simpleTask = &v1alpha1.Task{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test-task",
		Namespace: "foo",
	},
	Spec: v1alpha1.TaskSpec{
		BuildSpec: &buildv1alpha1.BuildSpec{
			Steps: []corev1.Container{
				{
					Name:  "simple-step",
					Image: "foo",
				},
			},
		},
	},
}

var templatedTask = &v1alpha1.Task{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "task-with-templating",
		Namespace: "foo",
	},
	Spec: v1alpha1.TaskSpec{
		BuildSpec: &buildv1alpha1.BuildSpec{
			Steps: []corev1.Container{
				{
					Name:  "mycontainer",
					Image: "myimage",
					Args:  []string{"--my-arg=${inputs.params.myarg}"},
				},
				{
					Name:  "myothercontainer",
					Image: "myotherimage",
					Args:  []string{"--my-other-arg=${inputs.resources.git-resource.url}"},
				},
			},
		},
	},
}

var gitResource = &v1alpha1.PipelineResource{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "git-resource",
		Namespace: "foo",
	},
	Spec: v1alpha1.PipelineResourceSpec{
		Type: "git",
		Params: []v1alpha1.Param{
			{
				Name:  "URL",
				Value: "https://foo.git",
			},
		},
	},
}

func TestReconcileBuildsCreated(t *testing.T) {
	taskruns := []*v1alpha1.TaskRun{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-success",
				Namespace: "foo",
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: v1alpha1.TaskRef{
					Name:       "test-task",
					APIVersion: "a1",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-with-sa-run-success",
				Namespace: "foo",
			},
			Spec: v1alpha1.TaskRunSpec{
				ServiceAccount: "test-sa",
				TaskRef: v1alpha1.TaskRef{
					Name:       "test-task",
					APIVersion: "a1",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-templating",
				Namespace: "foo",
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: v1alpha1.TaskRef{
					Name:       "task-with-templating",
					APIVersion: "a1",
				},
				Inputs: v1alpha1.TaskRunInputs{
					Params: []v1alpha1.Param{
						{
							Name:  "myarg",
							Value: "foo",
						},
					},
					Resources: []v1alpha1.PipelineResourceVersion{
						{
							ResourceRef: v1alpha1.PipelineResourceRef{
								Name:       "git-resource",
								APIVersion: "a1",
							},
							Version: "myversion",
						},
					},
				},
			},
		},
	}

	d := test.TestData{
		TaskRuns:          taskruns,
		Tasks:             []*v1alpha1.Task{simpleTask, templatedTask},
		PipelineResources: []*v1alpha1.PipelineResource{gitResource},
	}
	testcases := []struct {
		name            string
		taskRun         string
		wantedBuildSpec buildv1alpha1.BuildSpec
	}{
		{
			name:    "success",
			taskRun: "foo/test-taskrun-run-success",
			wantedBuildSpec: buildv1alpha1.BuildSpec{
				Steps: []corev1.Container{
					{
						Name:  "simple-step",
						Image: "foo",
					},
				},
			},
		},
		{
			name:    "params",
			taskRun: "foo/test-taskrun-templating",
			wantedBuildSpec: buildv1alpha1.BuildSpec{
				Steps: []corev1.Container{
					{
						Name:  "mycontainer",
						Image: "myimage",
						Args:  []string{"--my-arg=foo"},
					},
					{
						Name:  "myothercontainer",
						Image: "myotherimage",
						Args:  []string{"--my-other-arg=https://foo.git"},
					},
				},
			},
		},
		{
			name:    "serviceaccount",
			taskRun: "foo/test-taskrun-with-sa-run-success",
			wantedBuildSpec: buildv1alpha1.BuildSpec{
				ServiceAccountName: "test-sa",
				Steps: []corev1.Container{
					{
						Name:  "simple-step",
						Image: "foo",
					},
				},
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			c, _, _, buildClient := test.GetTaskRunController(d)
			if err := c.Reconciler.Reconcile(context.Background(), tc.taskRun); err != nil {
				t.Errorf("expected no error. Got error %v", err)
			}

			if len(buildClient.Actions()) == 0 {
				t.Errorf("Expected actions to be logged in the buildclient, got none")
			}
			namespace, name, err := cache.SplitMetaNamespaceKey(tc.taskRun)
			if err != nil {
				t.Errorf("Invalid resource key: %v", err)
			}
			// check error
			build, err := buildClient.BuildV1alpha1().Builds(namespace).Get(name, metav1.GetOptions{})
			if err != nil {
				t.Errorf("Failed to fetch build: %v", err)
			}
			if d := cmp.Diff(build.Spec, tc.wantedBuildSpec); d != "" {
				t.Errorf("buildspec doesn't match, diff: %s", d)
			}
		})
	}
}

func TestReconcileBuildCreationErrors(t *testing.T) {
	taskRuns := []*v1alpha1.TaskRun{
		&v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "notaskrun",
				Namespace: "foo",
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: v1alpha1.TaskRef{
					Name:       "notask",
					APIVersion: "a1",
				},
			},
		},
	}

	tasks := []*v1alpha1.Task{
		simpleTask,
	}

	d := test.TestData{
		TaskRuns: taskRuns,
		Tasks:    tasks,
	}

	testcases := []struct {
		name    string
		taskRun string
	}{
		{
			name:    "task run with no task",
			taskRun: "foo/notaskrun",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			c, _, _, buildClient := test.GetTaskRunController(d)
			if err := c.Reconciler.Reconcile(context.Background(), tc.taskRun); err == nil {
				t.Error("Expected not found error for non exitent task but got nil")
			}

			if len(buildClient.Actions()) != 1 {
				t.Errorf("expected no actions to be created by the reconciler, got %v", buildClient.Actions())
			}
		})
	}

}

func TestReconcileBuildFetchError(t *testing.T) {
	taskRun := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-run-success",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: v1alpha1.TaskRef{
				Name:       "test-task",
				APIVersion: "a1",
			},
		},
	}
	d := test.TestData{
		TaskRuns: []*v1alpha1.TaskRun{
			taskRun,
		},
		Tasks: []*v1alpha1.Task{simpleTask},
	}

	c, _, _, buildClient := test.GetTaskRunController(d)

	reactor := func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetVerb() == "get" && action.GetResource().Resource == "builds" {
			// handled fetching builds
			return true, nil, fmt.Errorf("induce failure fetching builds")
		}
		return false, nil, nil
	}

	buildClient.PrependReactor("*", "*", reactor)

	if err := c.Reconciler.Reconcile(context.Background(), fmt.Sprintf("%s/%s", taskRun.Namespace, taskRun.Name)); err == nil {
		t.Fatal("expected error but got nil")
	}
}

func TestReconcileBuildUpdateStatus(t *testing.T) {
	taskRun := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-run-success",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: v1alpha1.TaskRef{
				Name:       "test-task",
				APIVersion: "a1",
			},
		},
	}
	d := test.TestData{
		TaskRuns: []*v1alpha1.TaskRun{
			taskRun,
		},
		Tasks: []*v1alpha1.Task{simpleTask},
	}
	buildSt := &duckv1alpha1.Condition{
		Type: duckv1alpha1.ConditionSucceeded,
		// build is not completed
		Status:  corev1.ConditionUnknown,
		Message: "Running build",
	}

	c, _, pipelineClient, buildClient := test.GetTaskRunController(d)
	build := &buildv1alpha1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      taskRun.Name,
			Namespace: taskRun.Namespace,
		},
		Spec: *simpleTask.Spec.BuildSpec,
	}
	build.Status.SetCondition(buildSt)

	// TODO!
	_, err := buildClient.BuildV1alpha1().Builds(taskRun.Namespace).Create(build)
	if err != nil {
		t.Errorf("error creating build : %v", err)
	}

	if err := c.Reconciler.Reconcile(context.Background(), fmt.Sprintf("%s/%s", taskRun.Namespace, taskRun.Name)); err != nil {
		t.Fatalf("Unexpected error when Reconcile() : %v", err)
	}

	newTr, err := pipelineClient.PipelineV1alpha1().TaskRuns(taskRun.Namespace).Get(taskRun.Name, metav1.GetOptions{})
	//newTr, err := clients.taskrunInformer.Lister().TaskRuns(taskRun.Namespace).Get(taskRun.Name)
	if err != nil {
		t.Fatalf("Expected TaskRun %s to exist but instead got error when getting it: %v", taskRun.Name, err)
	}
	var ignoreLastTransitionTime = cmpopts.IgnoreTypes(duckv1alpha1.Condition{}.LastTransitionTime.Inner.Time)
	if d := cmp.Diff(newTr.Status.GetCondition(duckv1alpha1.ConditionSucceeded), buildSt, ignoreLastTransitionTime); d != "" {
		t.Fatalf("-want, +got: %v", d)
	}

	// update build status and trigger reconcile
	buildSt.Status = corev1.ConditionTrue
	buildSt.Message = "Build completed"

	build.Status.SetCondition(buildSt)

	_, err = buildClient.BuildV1alpha1().Builds(taskRun.Namespace).Update(build)
	if err != nil {
		t.Errorf("Unexpected error while creating build: %v", err)
	}

	if err := c.Reconciler.Reconcile(context.Background(), fmt.Sprintf("%s/%s", taskRun.Namespace, taskRun.Name)); err != nil {
		t.Fatalf("Unexpected error when Reconcile(): %v", err)
	}

	newTr, err = pipelineClient.PipelineV1alpha1().TaskRuns(taskRun.Namespace).Get(taskRun.Name, metav1.GetOptions{})
	//newTr, err = clients.taskrunInformer.Lister().TaskRuns(taskRun.Namespace).Get(taskRun.Name)
	if err != nil {
		t.Fatalf("Unexpected error fetching taskrun: %v", err)
	}
	if d := cmp.Diff(newTr.Status.GetCondition(duckv1alpha1.ConditionSucceeded), buildSt, ignoreLastTransitionTime); d != "" {
		t.Fatalf("Taskrun Status diff -want, +got: %v", d)
	}
}
