package taskrun_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/test"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var validBuildSteps = []corev1.Container{{
	Name:    "mystep",
	Image:   "myimage",
	Command: []string{"mycmd"},
}}

func Test_ValidTaskRunTask(t *testing.T) {
	trs := []*v1alpha1.TaskRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "taskrun-valid-input",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: v1alpha1.TaskRef{
				Name: "task-valid-input",
			},
			Inputs: v1alpha1.TaskRunInputs{
				Resources: []v1alpha1.TaskRunResource{
					v1alpha1.TaskRunResource{
						Name: "resource-to-build",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "example-resource",
						},
					},
				},
			},
		},
	}}

	ts := []*v1alpha1.Task{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "task-valid-input",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskSpec{
			Steps: validBuildSteps,
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{{
					Name: "resource-to-build",
					Type: v1alpha1.PipelineResourceTypeGit,
				}},
			},
		},
	}}

	rr := []*v1alpha1.PipelineResource{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-resource",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: v1alpha1.PipelineResourceTypeGit,
			Params: []v1alpha1.Param{{
				Name:  "foo",
				Value: "bar",
			}},
		},
	}}

	tcs := []struct {
		name    string
		taskrun *v1alpha1.TaskRun
		reason  string
	}{{
		name:    "taskrun-valid-input",
		taskrun: trs[0],
		reason:  "taskrun-with-valid-inputs",
	}}

	for i, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			d := test.Data{
				TaskRuns:          []*v1alpha1.TaskRun{trs[i]},
				Tasks:             ts,
				PipelineResources: rr,
			}

			c, _, _ := test.GetTaskRunController(d)
			err := c.Reconciler.Reconcile(context.Background(),
				fmt.Sprintf("%s/%s", tc.taskrun.Namespace, tc.taskrun.Name))

			if err != nil {
				t.Errorf("Did not expect to see error when reconciling invalid task but saw %q", err)
			}
			condition := trs[i].Status.GetCondition(duckv1alpha1.ConditionSucceeded)
			if condition == nil || condition.Status == corev1.ConditionFalse {
				t.Errorf("Valid task %s failed with condition %s, expected no failure", tc.taskrun.Name, condition)
			}
		})
	}
}

func Test_InvalidTaskRunTask(t *testing.T) {
	trs := []*v1alpha1.TaskRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-bad-task-input-resourceref",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: v1alpha1.TaskRef{
				Name: "unit-test-task",
			},
			Inputs: v1alpha1.TaskRunInputs{
				Resources: []v1alpha1.TaskRunResource{
					v1alpha1.TaskRunResource{
						Name: "test-resource-name",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "non-existent-resource1",
						},
					},
				},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-bad-task-output-resourceref",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: v1alpha1.TaskRef{
				Name: "unit-test-task",
			},
			Outputs: v1alpha1.TaskRunOutputs{
				Resources: []v1alpha1.TaskRunResource{
					v1alpha1.TaskRunResource{
						Name: "test-resource-name",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "non-existent-resource1",
						},
					},
				},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-bad-inputkey",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: v1alpha1.TaskRef{
				Name: "unit-test-wrong-input",
			},
			Inputs: v1alpha1.TaskRunInputs{
				Resources: []v1alpha1.TaskRunResource{
					v1alpha1.TaskRunResource{
						Name: "test-resource-name",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "non-existent",
						},
					},
				},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-bad-outputkey",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: v1alpha1.TaskRef{
				Name: "unit-test-task",
			},
			Outputs: v1alpha1.TaskRunOutputs{
				Resources: []v1alpha1.TaskRunResource{
					v1alpha1.TaskRunResource{
						Name: "test-resource-name",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "non-existent",
						},
					},
				},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-param-mismatch",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: v1alpha1.TaskRef{
				Name: "unit-task-multiple-params",
			},
			Inputs: v1alpha1.TaskRunInputs{
				Params: []v1alpha1.Param{
					v1alpha1.Param{
						Name:  "foobar",
						Value: "somethingfun",
					},
				},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-bad-resourcetype",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: v1alpha1.TaskRef{
				Name: "unit-task-bad-resourcetype",
			},
			Inputs: v1alpha1.TaskRunInputs{
				Resources: []v1alpha1.TaskRunResource{
					v1alpha1.TaskRunResource{
						Name: "testimageinput",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "git-test-resource",
						},
					},
				},
			},
		},
	}}

	ts := []*v1alpha1.Task{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unit-test-task",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{{}},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unit-task-wrong-input",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{{
					Name: "testinput",
				}},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unit-task-wrong-output",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskSpec{
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{{
					Name: "testoutput",
				}},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unit-task-multiple-params",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: &v1alpha1.Inputs{
				Params: []v1alpha1.TaskParam{{
					Name: "foo",
				}, {
					Name: "bar",
				}},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unit-task-bad-resourcetype",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{{
					Name: "testimageinput",
					Type: v1alpha1.PipelineResourceTypeImage,
				}},
			},
		},
	}}

	rr := []*v1alpha1.PipelineResource{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "git-test-resource",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: v1alpha1.PipelineResourceTypeGit,
			Params: []v1alpha1.Param{{
				Name:  "foo",
				Value: "bar",
			}},
		},
	}}

	tcs := []struct {
		name    string
		taskrun *v1alpha1.TaskRun
		reason  string
	}{
		{
			name:    "bad-input-source-bindings",
			taskrun: trs[0],
			reason:  "input-source-binding-to-invalid-resource",
		}, {
			name:    "bad-output-source-bindings",
			taskrun: trs[1],
			reason:  "output-source-binding-to-invalid-resource",
		}, {
			name:    "bad-inputkey",
			taskrun: trs[2],
			reason:  "bad-input-mapping",
		}, {
			name:    "bad-outputkey",
			taskrun: trs[3],
			reason:  "bad-output-mapping",
		}, {
			name:    "param-mismatch",
			taskrun: trs[4],
			reason:  "input-param-mismatch",
		}, {
			name:    "resource-mismatch",
			taskrun: trs[5],
			reason:  "input-resource-mismatch",
		}}

	for i, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			d := test.Data{
				TaskRuns:          []*v1alpha1.TaskRun{trs[i]},
				Tasks:             ts,
				PipelineResources: rr,
			}

			c, _, _ := test.GetTaskRunController(d)
			err := c.Reconciler.Reconcile(context.Background(),
				fmt.Sprintf("%s/%s", tc.taskrun.Namespace, tc.taskrun.Name))

			if err != nil {
				t.Errorf("Did not expect to see error when reconciling invalid task but saw %q", err)
			}
			condition := trs[i].Status.GetCondition(duckv1alpha1.ConditionSucceeded)
			if condition == nil || condition.Status != corev1.ConditionFalse {
				t.Errorf("Expected status to be failed on invalid task %s but was: %v", tc.taskrun.Name, condition)
			}
		})
	}
}
