// Package testing provides test helpers for the reconciler.
package testing

import (
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/selection"
)

// PipelineTasks is a set of example PipelineTasks for testing.
var PipelineTasks = []v1.PipelineTask{{
	Name:    "mytask1",
	TaskRef: &v1.TaskRef{Name: "task"},
}, {
	Name:    "mytask2",
	TaskRef: &v1.TaskRef{Name: "task"},
}, {
	Name:    "mytask3",
	TaskRef: &v1.TaskRef{Name: "task"},
}, {
	Name:    "mytask4",
	TaskRef: &v1.TaskRef{Name: "task"},
	Retries: 1,
}, {
	Name:    "mytask5",
	TaskRef: &v1.TaskRef{Name: "cancelledTask"},
	Retries: 2,
}, {
	Name:    "mytask6",
	TaskRef: &v1.TaskRef{Name: "task"},
}, {
	Name:     "mytask7",
	TaskRef:  &v1.TaskRef{Name: "taskWithOneParent"},
	RunAfter: []string{"mytask6"},
}, {
	Name:     "mytask8",
	TaskRef:  &v1.TaskRef{Name: "taskWithTwoParents"},
	RunAfter: []string{"mytask1", "mytask6"},
}, {
	Name:     "mytask9",
	TaskRef:  &v1.TaskRef{Name: "taskHasParentWithRunAfter"},
	RunAfter: []string{"mytask8"},
}, {
	Name:    "mytask10",
	TaskRef: &v1.TaskRef{Name: "taskWithWhenExpressions"},
	When: []v1.WhenExpression{{
		Input:    "foo",
		Operator: selection.In,
		Values:   []string{"foo", "bar"},
	}},
}, {
	Name:    "mytask11",
	TaskRef: &v1.TaskRef{Name: "taskWithWhenExpressions"},
	When: []v1.WhenExpression{{
		Input:    "foo",
		Operator: selection.NotIn,
		Values:   []string{"foo", "bar"},
	}},
}, {
	Name:    "mytask12",
	TaskRef: &v1.TaskRef{Name: "taskWithWhenExpressions"},
	When: []v1.WhenExpression{{
		Input:    "foo",
		Operator: selection.In,
		Values:   []string{"foo", "bar"},
	}},
	RunAfter: []string{"mytask1"},
}, {
	Name:    "mytask13",
	TaskRef: &v1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example", Name: "customtask"},
}, {
	Name:    "mytask14",
	TaskRef: &v1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example", Name: "customtask"},
}, {
	Name:    "mytask15",
	TaskRef: &v1.TaskRef{Name: "taskWithReferenceToTaskResult"},
	Params:  v1.Params{{Name: "param1", Value: *v1.NewStructuredValues("$(tasks.mytask1.results.result1)")}},
}, {
	Name: "mytask16",
	Matrix: &v1.Matrix{
		Params: v1.Params{{
			Name:  "browser",
			Value: v1.ParamValue{ArrayVal: []string{"safari", "chrome"}},
		}},
	},
}, {
	Name: "mytask17",
	Matrix: &v1.Matrix{
		Params: v1.Params{{
			Name:  "browser",
			Value: v1.ParamValue{ArrayVal: []string{"safari", "chrome"}},
		}},
	},
}, {
	Name:    "mytask18",
	TaskRef: &v1.TaskRef{Name: "task"},
	Retries: 1,
	Matrix: &v1.Matrix{
		Params: v1.Params{{
			Name:  "browser",
			Value: v1.ParamValue{ArrayVal: []string{"safari", "chrome"}},
		}},
	},
}, {
	Name:    "mytask19",
	TaskRef: &v1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example", Name: "customtask"},
	Matrix: &v1.Matrix{
		Params: v1.Params{{
			Name:  "browser",
			Value: v1.ParamValue{ArrayVal: []string{"safari", "chrome"}},
		}},
	},
}, {
	Name:    "mytask20",
	TaskRef: &v1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example", Name: "customtask"},
	Matrix: &v1.Matrix{
		Params: v1.Params{{
			Name:  "browser",
			Value: v1.ParamValue{ArrayVal: []string{"safari", "chrome"}},
		}},
	},
}, {
	Name:    "mytask21",
	TaskRef: &v1.TaskRef{Name: "task"},
	Retries: 2,
	Matrix: &v1.Matrix{
		Params: v1.Params{{
			Name:  "browser",
			Value: v1.ParamValue{ArrayVal: []string{"safari", "chrome"}},
		}},
	},
}}

// ExamplePipeline is a sample Pipeline for testing.
var ExamplePipeline = &v1.Pipeline{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipeline",
	},
	Spec: v1.PipelineSpec{
		Tasks: PipelineTasks,
	},
}

// ExampleTask is a sample Task for testing.
var ExampleTask = &v1.Task{
	ObjectMeta: metav1.ObjectMeta{
		Name: "task",
	},
	Spec: v1.TaskSpec{
		Steps: []v1.Step{{
			Name: "step1",
		}},
	},
}

// ExampleTaskRuns is a set of TaskRuns for testing.
/* ...copy from trs, renamed... */
var ExampleTaskRuns = []v1.TaskRun{{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipelinerun-mytask1",
	},
	Spec: v1.TaskRunSpec{},
}, {
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipelinerun-mytask2",
	},
	Spec: v1.TaskRunSpec{},
}, {
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipelinerun-mytask4",
	},
	Spec: v1.TaskRunSpec{},
}}

// ExampleCustomRuns is a set of CustomRuns for testing.
/* ...copy from customRuns, renamed... */
var ExampleCustomRuns = []v1beta1.CustomRun{{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipelinerun-mytask13",
	},
	Spec: v1beta1.CustomRunSpec{},
}, {
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipelinerun-mytask14",
	},
	Spec: v1beta1.CustomRunSpec{},
}}

// ExampleMatrixedPipelineTask is a sample matrixed PipelineTask for testing.
/* ...copy from matrixedPipelineTask, renamed... */
var ExampleMatrixedPipelineTask = &v1.PipelineTask{
	Name: "task",
	TaskSpec: &v1.EmbeddedTask{
		TaskSpec: v1.TaskSpec{
			Params: []v1.ParamSpec{{
				Name: "browser",
				Type: v1.ParamTypeString,
			}},
			Results: []v1.TaskResult{{
				Name: "BROWSER",
			}},
			Steps: []v1.Step{{
				Name:   "produce-results",
				Image:  "bash:latest",
				Script: `#!/usr/bin/env bash\necho -n "$(params.browser)" | sha256sum | tee $(results.BROWSER.path)"`,
			}},
		},
	},
	Matrix: &v1.Matrix{
		Params: v1.Params{{
			Name:  "browser",
			Value: v1.ParamValue{ArrayVal: []string{"safari", "chrome"}},
		}},
	},
}

// NewTaskRun returns a new TaskRun for testing.
func NewTaskRun(tr v1.TaskRun) *v1.TaskRun {
	return &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: tr.Namespace,
			Name:      tr.Name,
		},
		Spec: tr.Spec,
		Status: v1.TaskRunStatus{
			Status: v1.TaskRunStatus{}.Status,
		},
	}
}

// NewCustomRun returns a new CustomRun for testing.
func NewCustomRun(run v1beta1.CustomRun) *v1beta1.CustomRun {
	return &v1beta1.CustomRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: run.Namespace,
			Name:      run.Name,
		},
		Spec: run.Spec,
		Status: v1beta1.CustomRunStatus{
			Status: v1beta1.CustomRunStatus{}.Status,
		},
	}
}
