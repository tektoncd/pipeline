// Package testing provides test helpers for the reconciler.
package testing

import (
	"fmt"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	apis "knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
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
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type: apis.ConditionSucceeded,
				}},
			},
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
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type: apis.ConditionSucceeded,
				}},
			},
		},
	}
}

var (
	trueb = true
)

// TwoPipelinesInPipelineMixedTasks creates a parent Pipeline with two embedded child Pipelines:
// one using an embedded taskSpec and the other using a taskRef. It also creates a PipelineRun
// for the parent Pipeline, the expected child PipelineRuns for each child Pipeline and the
// referenced task.
func TwoPipelinesInPipelineMixedTasks(t *testing.T, namespace, parentPipelineRunName string) (*v1.Task, *v1.Pipeline, *v1.PipelineRun, []*v1.PipelineRun) {
	t.Helper()
	uid := "bar"
	taskName := "ref-task"
	parentPipelineName := "parent-pipeline-mixed"
	childPipelineName1 := "child-pipeline-taskspec"
	childPipelineName2 := "child-pipeline-taskref"
	childPipelineTaskName1 := "child-taskspec"
	childPipelineTaskName2 := "child-taskref"

	task := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: mystep
    image: mirror.gcr.io/busybox
    script: 'echo "Hello from referenced task in child PipelineRun 2!"'
`, taskName, namespace))

	parentPipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: %s
    pipelineSpec:
      tasks:
      - name: %s
        taskSpec:
          steps:
          - name: mystep
            image: mirror.gcr.io/busybox
            script: 'echo "Hello from child PipelineRun 1!"'
  - name: %s
    pipelineSpec:
      tasks:
      - name: %s
        taskRef:
          name: %s
`, parentPipelineName, namespace, childPipelineName1, childPipelineTaskName1, childPipelineName2, childPipelineTaskName2, taskName))

	parentPipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
  uid: %s
spec:
  pipelineRef:
    name: %s
`, parentPipelineRunName, namespace, uid, parentPipelineName))

	expectedName1 := parentPipelineRunName + "-" + childPipelineName1
	expectedChildPipelineRun1 := parse.MustParseChildPipelineRunWithObjectMeta(
		t,
		childPipelineRunWithObjectMeta(
			expectedName1,
			namespace,
			parentPipelineRunName,
			parentPipelineName,
			childPipelineName1,
			uid,
		),
		fmt.Sprintf(`
spec:
  pipelineSpec:
    tasks:
    - name: %s
      taskSpec:
        steps:
        - name: mystep
          image: mirror.gcr.io/busybox
          script: 'echo "Hello from child PipelineRun 1!"'
  taskRunTemplate:
    serviceAccountName: default
`, childPipelineTaskName1),
	)

	expectedName2 := parentPipelineRunName + "-" + childPipelineName2
	expectedChildPipelineRun2 := parse.MustParseChildPipelineRunWithObjectMeta(
		t,
		childPipelineRunWithObjectMeta(
			expectedName2,
			namespace,
			parentPipelineRunName,
			parentPipelineName,
			childPipelineName2,
			uid,
		),
		fmt.Sprintf(`
spec:
  pipelineSpec:
    tasks:
    - name: %s
      taskRef:
        name: %s
  taskRunTemplate:
    serviceAccountName: default
`, childPipelineTaskName2, taskName),
	)

	return task, parentPipeline, parentPipelineRun, []*v1.PipelineRun{expectedChildPipelineRun1, expectedChildPipelineRun2}
}

// OnePipelineInPipeline creates a single Pipeline with one child pipeline using
// PipelineSpec with TaskSpec. It also creates the according PipelineRun for it
// and the expected child PipelineRun against which the test will validate.
func OnePipelineInPipeline(t *testing.T, namespace, parentPipelineRunName string) (*v1.Pipeline, *v1.PipelineRun, *v1.PipelineRun) {
	t.Helper()
	uid := "bar"
	parentPipelineName := "parent-pipeline"
	childPipelineName := "child-pipeline"
	childPipelineTaskName := "child-pipeline-task"

	parentPipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: %s
    pipelineSpec:
      tasks:
      - name: %s
        taskSpec:
          steps:
          - name: mystep
            image: mirror.gcr.io/busybox
            script: 'echo "Hello from child PipelineRun!"'
`, parentPipelineName, namespace, childPipelineName, childPipelineTaskName))

	parentPipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
  uid: %s
spec:
  pipelineRef:
    name: %s
`, parentPipelineRunName, namespace, uid, parentPipelineName))

	expectedName := parentPipelineRunName + "-" + childPipelineName
	expectedChildPipelineRun := parse.MustParseChildPipelineRunWithObjectMeta(
		t,
		childPipelineRunWithObjectMeta(
			expectedName,
			namespace,
			parentPipelineRunName,
			parentPipelineName,
			childPipelineName,
			uid,
		),
		fmt.Sprintf(`
spec:
  pipelineSpec:
    tasks:
    - name: %s
      taskSpec:
        steps:
        - name: mystep
          image: mirror.gcr.io/busybox
          script: 'echo "Hello from child PipelineRun!"'
  taskRunTemplate:
    serviceAccountName: default
`, childPipelineTaskName),
	)

	return parentPipeline, parentPipelineRun, expectedChildPipelineRun
}

// OnePipelineRefInPipelineWithParamsAndWorkspaces creates a parent Pipeline that references a child Pipeline
// via pipelineRef with both params and workspaces. It returns (parentPipeline, childPipeline, parentPipelineRun, expectedChildPipelineRun).
func OnePipelineRefInPipelineWithParamsAndWorkspaces(t *testing.T, namespace, parentPipelineRunName string) (*v1.Pipeline, *v1.Pipeline, *v1.PipelineRun, *v1.PipelineRun) {
	t.Helper()
	uid := "bar"
	parentPipelineName := "parent-pipeline"
	childPipelineName := "child-pipeline"
	childPipelineTaskName := "child-pipeline-task"

	childPipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  params:
  - name: msg
    type: string
  workspaces:
  - name: child-ws
  tasks:
  - name: %s
    taskSpec:
      params:
      - name: msg
        type: string
      steps:
      - name: mystep
        image: mirror.gcr.io/busybox
        script: 'echo $(params.msg) && ls $(workspaces.child-ws.path)'
      workspaces:
      - name: child-ws
    params:
    - name: msg
      value: $(params.msg)
    workspaces:
    - name: child-ws
      workspace: child-ws
`, childPipelineName, namespace, childPipelineTaskName))

	parentPipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  params:
  - name: greeting
    type: string
  workspaces:
  - name: parent-ws
  tasks:
  - name: %s
    pipelineRef:
      name: %s
    params:
    - name: msg
      value: $(params.greeting)
    workspaces:
    - name: child-ws
      workspace: parent-ws
`, parentPipelineName, namespace, childPipelineName, childPipelineName))

	parentPipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
  uid: %s
spec:
  pipelineRef:
    name: %s
  params:
  - name: greeting
    value: hello-world
  workspaces:
  - name: parent-ws
    emptyDir: {}
`, parentPipelineRunName, namespace, uid, parentPipelineName))

	expectedName := parentPipelineRunName + "-" + childPipelineName
	childObjectMeta := childPipelineRunWithObjectMeta(
		expectedName,
		namespace,
		parentPipelineRunName,
		parentPipelineName,
		childPipelineName,
		uid,
	)
	childObjectMeta.Labels[pipeline.PipelineLabelKey] = childPipelineName
	expectedChildPipelineRun := parse.MustParseChildPipelineRunWithObjectMeta(
		t,
		childObjectMeta,
		fmt.Sprintf(`
spec:
  pipelineRef:
    name: %s
  params:
  - name: msg
    value: hello-world
  workspaces:
  - name: child-ws
    emptyDir: {}
  taskRunTemplate:
    serviceAccountName: default
`, childPipelineName),
	)

	return parentPipeline, childPipeline, parentPipelineRun, expectedChildPipelineRun
}

func WithAnnotationAndLabel(pr *v1.PipelineRun, withUnused bool) *v1.PipelineRun {
	if pr.Annotations == nil {
		pr.Annotations = map[string]string{}
	}
	pr.Annotations["tekton.test/annotation"] = "test-annotation-value"

	if pr.Labels == nil {
		pr.Labels = map[string]string{}
	}
	pr.Labels["tekton.test/label"] = "test-label-value"

	if withUnused {
		pr.Labels["tekton.dev/pipeline"] = "will-not-be-used"
	}

	return pr
}

// WithServiceAccount sets serviceAccountName on the PipelineRun's taskRunTemplate. It decorates a
// fixture's PipelineRuns serviceAccountName in the TaskRunTemplate
func WithServiceAccount(pr *v1.PipelineRun, serviceAccountName string) *v1.PipelineRun {
	pr.Spec.TaskRunTemplate.ServiceAccountName = serviceAccountName
	return pr
}

func childPipelineRunWithObjectMeta(
	childPipelineRunName,
	ns,
	parentPipelineRunName,
	parentPipelineName,
	pipelineTaskName,
	uid string,
) metav1.ObjectMeta {
	om := metav1.ObjectMeta{
		Name:      childPipelineRunName,
		Namespace: ns,
		OwnerReferences: []metav1.OwnerReference{{
			Kind:               pipeline.PipelineRunControllerName,
			Name:               parentPipelineRunName,
			APIVersion:         "tekton.dev/v1",
			Controller:         &trueb,
			BlockOwnerDeletion: &trueb,
			UID:                types.UID(uid),
		}},
		Labels: map[string]string{
			pipeline.PipelineLabelKey:       parentPipelineName,
			pipeline.PipelineRunLabelKey:    parentPipelineRunName,
			pipeline.PipelineTaskLabelKey:   pipelineTaskName,
			pipeline.PipelineRunUIDLabelKey: uid,
			pipeline.MemberOfLabelKey:       v1.PipelineTasks,
		},
		Annotations: map[string]string{},
	}

	return om
}

// NestedPipelinesInPipeline creates a three-level nested pipeline structure:
// Parent Pipeline -> Child Pipeline -> Grandchild Pipeline
// Returns the parent pipeline, parent pipelinerun, expected child pipelinerun, and expected grandchild pipelinerun
func NestedPipelinesInPipeline(t *testing.T, namespace, parentPipelineRunName string) (*v1.Pipeline, *v1.PipelineRun, *v1.PipelineRun, *v1.PipelineRun) {
	t.Helper()
	uid := "nested"
	parentPipelineName := "parent-pipeline"
	childPipelineName := "child-ppl"
	grandchildPipelineName := "grandchild-ppl"
	grandchildPipelineTaskName := "grandchild-task"

	parentPipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: %s
    pipelineSpec:
      tasks:
      - name: %s
        pipelineSpec:
          tasks:
          - name: %s
            taskSpec:
              steps:
              - name: mystep
                image: mirror.gcr.io/busybox
                script: 'echo "Hello from grandchild Pipeline!"'
`, parentPipelineName, namespace, childPipelineName, grandchildPipelineName, grandchildPipelineTaskName))

	parentPipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
  uid: %s
spec:
  pipelineRef:
    name: %s
`, parentPipelineRunName, namespace, uid, parentPipelineName))

	// expected child pipeline run created by parent
	expectedChildName := kmeta.ChildName(parentPipelineRunName, "-"+childPipelineName)
	expectedChildPipelineRun := parse.MustParseChildPipelineRunWithObjectMeta(
		t,
		childPipelineRunWithObjectMeta(
			expectedChildName,
			namespace,
			parentPipelineRunName,
			parentPipelineName,
			childPipelineName,
			uid,
		),
		fmt.Sprintf(`
spec:
  pipelineSpec:
    tasks:
    - name: %s
      pipelineSpec:
        tasks:
        - name: %s
          taskSpec:
            steps:
            - name: mystep
              image: mirror.gcr.io/busybox
              script: 'echo "Hello from grandchild Pipeline!"'
  taskRunTemplate:
    serviceAccountName: default
`, grandchildPipelineName, grandchildPipelineTaskName),
	)

	// expected grandchild pipeline run created by child
	expectedGrandchildName := kmeta.ChildName(expectedChildName, "-"+grandchildPipelineName)
	expectedGrandchildPipelineRun := parse.MustParseChildPipelineRunWithObjectMeta(
		t,
		childPipelineRunWithObjectMeta(
			expectedGrandchildName,
			namespace,
			expectedChildName,
			expectedChildName,
			grandchildPipelineName,
			"", // keep empty, UID is not set on actual child PipelineRun by fake client
		),
		fmt.Sprintf(`
spec:
  pipelineSpec:
    tasks:
    - name: %s
      taskSpec:
        steps:
        - name: mystep
          image: mirror.gcr.io/busybox
          script: 'echo "Hello from grandchild Pipeline!"'
  taskRunTemplate:
    serviceAccountName: default
`, grandchildPipelineTaskName),
	)

	return parentPipeline, parentPipelineRun, expectedChildPipelineRun, expectedGrandchildPipelineRun
}

// NestedPipelineRefsInPipeline creates a three-level nested pipeline structure using PipelineRef:
// Parent Pipeline (A) -> Child Pipeline (B) via PipelineRef -> Grandchild Pipeline (C) via PipelineRef
// Returns all three pipelines, the parent PipelineRun, expected child PipelineRun, and expected grandchild PipelineRun.
func NestedPipelineRefsInPipeline(t *testing.T, namespace, parentPipelineRunName string) (*v1.Pipeline, *v1.Pipeline, *v1.Pipeline, *v1.PipelineRun, *v1.PipelineRun, *v1.PipelineRun) {
	t.Helper()
	uid := "nested-ref"
	parentPipelineName := "parent-pipeline"
	childPipelineName := "child-ppl"
	grandchildPipelineName := "grandchild-ppl"
	grandchildTaskName := "grandchild-task"

	grandchildPipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: %s
    taskSpec:
      steps:
      - name: mystep
        image: mirror.gcr.io/busybox
        script: 'echo "Hello from grandchild Pipeline!"'
`, grandchildPipelineName, namespace, grandchildTaskName))

	childPipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: %s
    pipelineRef:
      name: %s
`, childPipelineName, namespace, grandchildPipelineName, grandchildPipelineName))

	parentPipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: %s
    pipelineRef:
      name: %s
`, parentPipelineName, namespace, childPipelineName, childPipelineName))

	parentPipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
  uid: %s
spec:
  pipelineRef:
    name: %s
`, parentPipelineRunName, namespace, uid, parentPipelineName))

	// expected child pipeline run created by parent
	expectedChildName := kmeta.ChildName(parentPipelineRunName, "-"+childPipelineName)
	childObjectMeta := childPipelineRunWithObjectMeta(
		expectedChildName,
		namespace,
		parentPipelineRunName,
		parentPipelineName,
		childPipelineName,
		uid,
	)
	childObjectMeta.Labels[pipeline.PipelineLabelKey] = childPipelineName
	expectedChildPipelineRun := parse.MustParseChildPipelineRunWithObjectMeta(
		t,
		childObjectMeta,
		fmt.Sprintf(`
spec:
  pipelineRef:
    name: %s
  taskRunTemplate:
    serviceAccountName: default
`, childPipelineName),
	)

	// expected grandchild pipeline run created by child
	expectedGrandchildName := kmeta.ChildName(expectedChildName, "-"+grandchildPipelineName)
	grandchildObjectMeta := childPipelineRunWithObjectMeta(
		expectedGrandchildName,
		namespace,
		expectedChildName,
		expectedChildName,
		grandchildPipelineName,
		"", // keep empty, UID is not set on actual child PipelineRun by fake client
	)
	grandchildObjectMeta.Labels[pipeline.PipelineLabelKey] = grandchildPipelineName
	expectedGrandchildPipelineRun := parse.MustParseChildPipelineRunWithObjectMeta(
		t,
		grandchildObjectMeta,
		fmt.Sprintf(`
spec:
  pipelineRef:
    name: %s
  taskRunTemplate:
    serviceAccountName: default
`, grandchildPipelineName),
	)

	return parentPipeline, childPipeline, grandchildPipeline, parentPipelineRun, expectedChildPipelineRun, expectedGrandchildPipelineRun
}

// SelfReferencingPipelineRefCycle returns a Pipeline whose only PipelineTask
// has a pipelineRef back to itself, and a PipelineRun that runs it. The
// PipelineRun is pre-labelled with tekton.dev/pipeline so unit-level reconcile
// tests (which seed the run directly) exercise the cycle-detection label walk.
// Shared by unit and e2e tests for self-cycle detection.
func SelfReferencingPipelineRefCycle(t *testing.T, namespace, parentPipelineRunName string) (*v1.Pipeline, *v1.PipelineRun) {
	t.Helper()
	pipelineName := "self-ref-pipeline"
	pipelineObj := &v1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: pipelineName, Namespace: namespace},
		Spec: v1.PipelineSpec{Tasks: []v1.PipelineTask{{
			Name:        "ref-self",
			PipelineRef: &v1.PipelineRef{Name: pipelineName},
		}}},
	}
	pipelineRun := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      parentPipelineRunName,
			Namespace: namespace,
			Labels:    map[string]string{pipeline.PipelineLabelKey: pipelineName},
		},
		Spec: v1.PipelineRunSpec{PipelineRef: &v1.PipelineRef{Name: pipelineName}},
	}
	return pipelineObj, pipelineRun
}

// TwoLevelPipelineRefCycle returns Pipelines pipeline-a and pipeline-b where
// each references the other via pipelineRef, plus a PipelineRun that runs
// pipeline-a. Shared by unit and e2e tests for two-level cycle detection.
func TwoLevelPipelineRefCycle(t *testing.T, namespace, parentPipelineRunName string) (*v1.Pipeline, *v1.Pipeline, *v1.PipelineRun) {
	t.Helper()
	const (
		pipelineAName = "pipeline-a"
		pipelineBName = "pipeline-b"
	)
	pipelineA := &v1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: pipelineAName, Namespace: namespace},
		Spec: v1.PipelineSpec{Tasks: []v1.PipelineTask{{
			Name:        pipelineBName,
			PipelineRef: &v1.PipelineRef{Name: pipelineBName},
		}}},
	}
	pipelineB := &v1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: pipelineBName, Namespace: namespace},
		Spec: v1.PipelineSpec{Tasks: []v1.PipelineTask{{
			Name:        pipelineAName,
			PipelineRef: &v1.PipelineRef{Name: pipelineAName},
		}}},
	}
	parentPipelineRun := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      parentPipelineRunName,
			Namespace: namespace,
			Labels:    map[string]string{pipeline.PipelineLabelKey: pipelineAName},
		},
		Spec: v1.PipelineRunSpec{PipelineRef: &v1.PipelineRef{Name: pipelineAName}},
	}
	return pipelineA, pipelineB, parentPipelineRun
}

// OnePipelineRefMissing returns a parent Pipeline that references a
// non-existent Pipeline by name, plus a PipelineRun that runs the parent.
// Shared by unit and e2e tests for the missing-child-pipeline failure path.
func OnePipelineRefMissing(t *testing.T, namespace, parentPipelineRunName string) (*v1.Pipeline, *v1.PipelineRun) {
	t.Helper()
	parentPipeline := &v1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "parent-ref-not-found", Namespace: namespace},
		Spec: v1.PipelineSpec{Tasks: []v1.PipelineTask{{
			Name:        "ref-nonexistent",
			PipelineRef: &v1.PipelineRef{Name: "nonexistent-pipeline"},
		}}},
	}
	parentPipelineRun := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: parentPipelineRunName, Namespace: namespace},
		Spec:       v1.PipelineRunSpec{PipelineRef: &v1.PipelineRef{Name: "parent-ref-not-found"}},
	}
	return parentPipeline, parentPipelineRun
}

// OnePipelineRefMissingWorkspace returns a parent Pipeline that references a
// child Pipeline declaring a workspace, where the parent omits the
// corresponding binding — so the child PipelineRun fails workspace
// validation.
func OnePipelineRefMissingWorkspace(t *testing.T, namespace, parentPipelineRunName string) (*v1.Pipeline, *v1.Pipeline, *v1.PipelineRun) {
	t.Helper()
	childPipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: child-pipeline-workspace
  namespace: %s
spec:
  workspaces:
  - name: child-ws
  tasks:
  - name: use-workspace
    taskSpec:
      steps:
      - name: ls
        image: mirror.gcr.io/busybox
        script: |
          ls $(workspaces.shared.path)
      workspaces:
      - name: shared
    workspaces:
    - name: shared
      workspace: child-ws
`, namespace))
	parentPipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: parent-pipeline-missing-ws
  namespace: %s
spec:
  tasks:
  - name: child-pipeline-workspace
    pipelineRef:
      name: child-pipeline-workspace
`, namespace))
	parentPipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: parent-pipeline-missing-ws
`, parentPipelineRunName, namespace))
	return parentPipeline, childPipeline, parentPipelineRun
}

// pinpChildTaskSpecPipeline returns a minimal standalone child Pipeline with a
// single taskSpec task, for use as a pipelineRef target in PinP tests. The task
// echoes the pipeline name so child PipelineRuns are distinguishable at runtime.
func pinpChildTaskSpecPipeline(t *testing.T, name, namespace string) *v1.Pipeline {
	t.Helper()
	return parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: run
    taskSpec:
      steps:
      - name: echo
        image: mirror.gcr.io/busybox
        script: 'echo %s'
`, name, namespace, name))
}

// MultiplePipelineRefsInPipeline returns a parent Pipeline with two pipelineRef
// PipelineTasks (each targeting a standalone child Pipeline), the two child
// Pipelines, the parent PipelineRun, and the two expected child PipelineRuns.
// Shared by unit and e2e tests for the multi-child pipelineRef path.
func MultiplePipelineRefsInPipeline(t *testing.T, namespace, parentPipelineRunName string) (*v1.Pipeline, []*v1.Pipeline, *v1.PipelineRun, []*v1.PipelineRun) {
	t.Helper()
	childA := pinpChildTaskSpecPipeline(t, "child-a", namespace)
	childB := pinpChildTaskSpecPipeline(t, "child-b", namespace)

	parentPipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: parent-multi-ref
  namespace: %s
spec:
  tasks:
  - name: child-a
    pipelineRef:
      name: child-a
  - name: child-b
    pipelineRef:
      name: child-b
`, namespace))

	parentPipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: parent-multi-ref
`, parentPipelineRunName, namespace))

	var expectedChildPipelineRuns []*v1.PipelineRun
	for _, name := range []string{"child-a", "child-b"} {
		expectedChildPipelineRuns = append(expectedChildPipelineRuns, &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: kmeta.ChildName(parentPipelineRunName, "-"+name), Namespace: namespace},
			Spec:       v1.PipelineRunSpec{PipelineRef: &v1.PipelineRef{Name: name}},
		})
	}

	return parentPipeline, []*v1.Pipeline{childA, childB}, parentPipelineRun, expectedChildPipelineRuns
}

// MixedPipelineRefSpecAndTaskRefInPipeline returns a parent Pipeline that mixes
// the three child kinds in one Pipeline: a pipelineRef child, an inline
// pipelineSpec child, and a regular taskRef task. It returns the referenced
// Task, the parent Pipeline, the standalone child Pipeline (the pipelineRef
// target), the parent PipelineRun, and the expected pipelineRef and pipelineSpec
// child PipelineRuns. Shared by unit and e2e tests for the mixed-children path.
func MixedPipelineRefSpecAndTaskRefInPipeline(t *testing.T, namespace, parentPipelineRunName string) (*v1.Task, *v1.Pipeline, *v1.Pipeline, *v1.PipelineRun, *v1.PipelineRun, *v1.PipelineRun) {
	t.Helper()
	helloTask := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: hello-task
  namespace: %s
spec:
  steps:
  - name: echo
    image: mirror.gcr.io/busybox
    script: 'echo hello'
`, namespace))

	childRef := pinpChildTaskSpecPipeline(t, "child-ref", namespace)

	parentPipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: parent-mixed
  namespace: %s
spec:
  tasks:
  - name: ref-child
    pipelineRef:
      name: child-ref
  - name: spec-child
    pipelineSpec:
      tasks:
      - name: inline
        taskSpec:
          steps:
          - name: echo
            image: mirror.gcr.io/busybox
            script: 'echo spec-child'
  - name: direct-task
    taskRef:
      name: hello-task
`, namespace))

	parentPipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: parent-mixed
`, parentPipelineRunName, namespace))

	expectedRefChild := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: kmeta.ChildName(parentPipelineRunName, "-ref-child"), Namespace: namespace},
		Spec:       v1.PipelineRunSpec{PipelineRef: &v1.PipelineRef{Name: "child-ref"}},
	}
	// The pipelineSpec child carries the inline spec copied from the parent's
	// spec-child task, so createKindsMap can derive its leaf TaskRun and
	// assertPinP can diff the resolved spec.
	expectedSpecChild := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: kmeta.ChildName(parentPipelineRunName, "-spec-child"), Namespace: namespace},
		Spec:       v1.PipelineRunSpec{PipelineSpec: parentPipeline.Spec.Tasks[1].PipelineSpec},
	}

	return helloTask, parentPipeline, childRef, parentPipelineRun, expectedRefChild, expectedSpecChild
}

// PipelineRefInPipelineWhenSkipped returns a parent Pipeline whose only
// PipelineTask references a child Pipeline but is guarded by a when expression
// that evaluates to false, plus the child Pipeline and the parent PipelineRun.
// No child PipelineRun is expected. Shared by unit and e2e tests for the
// when-expression-skips-child path.
func PipelineRefInPipelineWhenSkipped(t *testing.T, namespace, parentPipelineRunName string) (*v1.Pipeline, *v1.Pipeline, *v1.PipelineRun) {
	t.Helper()
	childPipeline := pinpChildTaskSpecPipeline(t, "child-skip", namespace)

	// "run" is not in ["no"], so the guard is false and the task is skipped.
	parentPipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: parent-when-skip
  namespace: %s
spec:
  tasks:
  - name: maybe-child
    when:
    - input: "run"
      operator: in
      values: ["no"]
    pipelineRef:
      name: child-skip
`, namespace))

	parentPipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: parent-when-skip
`, parentPipelineRunName, namespace))

	return parentPipeline, childPipeline, parentPipelineRun
}

// PipelineRefInFinally returns a parent Pipeline with a main task and a finally
// PipelineTask that references a child Pipeline, plus the child Pipeline, the
// parent PipelineRun, and the expected finally child PipelineRun. Used by e2e
// tests for the pipelineRef-in-finally path.
func PipelineRefInFinally(t *testing.T, namespace, parentPipelineRunName string) (*v1.Pipeline, *v1.Pipeline, *v1.PipelineRun, *v1.PipelineRun) {
	t.Helper()
	finallyChild := pinpChildTaskSpecPipeline(t, "child-finally", namespace)

	parentPipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: parent-finally
  namespace: %s
spec:
  tasks:
  - name: main
    taskSpec:
      steps:
      - name: echo
        image: mirror.gcr.io/busybox
        script: 'echo main'
  finally:
  - name: cleanup-child
    pipelineRef:
      name: child-finally
`, namespace))

	parentPipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: parent-finally
`, parentPipelineRunName, namespace))

	expectedChildPipelineRun := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: kmeta.ChildName(parentPipelineRunName, "-cleanup-child"), Namespace: namespace},
		Spec:       v1.PipelineRunSpec{PipelineRef: &v1.PipelineRef{Name: "child-finally"}},
	}

	return parentPipeline, finallyChild, parentPipelineRun, expectedChildPipelineRun
}

// PipelineRefWithPVCWorkspace returns a parent Pipeline that writes a file to a
// PVC-backed workspace and then maps that same workspace into a child Pipeline
// via pipelineRef, where a task in the child reads the file back. It returns the
// child Pipeline, the parent Pipeline, the parent PipelineRun (with a
// volumeClaimTemplate workspace), and the expected child PipelineRun. The parent
// only succeeds if the child sees the parent-written data, proving the volume is
// shared across the parent->child boundary. Used by e2e tests.
func PipelineRefWithPVCWorkspace(t *testing.T, namespace, parentPipelineRunName string) (*v1.Pipeline, *v1.Pipeline, *v1.PipelineRun, *v1.PipelineRun) {
	t.Helper()
	childPipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: child-reader
  namespace: %s
spec:
  workspaces:
  - name: child-ws
  tasks:
  - name: read
    workspaces:
    - name: ws
      workspace: child-ws
    taskSpec:
      workspaces:
      - name: ws
      steps:
      - name: read
        image: mirror.gcr.io/busybox
        script: |
          grep -q hello-pinp $(workspaces.ws.path)/data
`, namespace))

	parentPipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: parent-pvc-ws
  namespace: %s
spec:
  workspaces:
  - name: shared
  tasks:
  - name: write
    workspaces:
    - name: ws
      workspace: shared
    taskSpec:
      workspaces:
      - name: ws
      steps:
      - name: write
        image: mirror.gcr.io/busybox
        script: |
          echo hello-pinp > $(workspaces.ws.path)/data
  - name: reader-child
    runAfter: [write]
    workspaces:
    - name: child-ws
      workspace: shared
    pipelineRef:
      name: child-reader
`, namespace))

	parentPipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: parent-pvc-ws
  workspaces:
  - name: shared
    volumeClaimTemplate:
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 16Mi
`, parentPipelineRunName, namespace))

	expectedChildPipelineRun := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: kmeta.ChildName(parentPipelineRunName, "-reader-child"), Namespace: namespace},
		Spec:       v1.PipelineRunSpec{PipelineRef: &v1.PipelineRef{Name: "child-reader"}},
	}

	return parentPipeline, childPipeline, parentPipelineRun, expectedChildPipelineRun
}

// ChildPipelineViaClusterResolver returns a parent Pipeline whose PipelineTask
// resolves its child Pipeline through the cluster resolver (rather than a local
// name ref), plus the resolved child Pipeline, the parent PipelineRun, and the
// expected child PipelineRun. The expected child PipelineRun carries only the
// resolved tekton.dev/pipeline label (its Spec is left empty because the
// reconciler stores the resolver ref, which tests don't reconstruct). Used by
// e2e tests for the child-pipeline-via-resolver path.
func ChildPipelineViaClusterResolver(t *testing.T, namespace, parentPipelineRunName string) (*v1.Pipeline, *v1.Pipeline, *v1.PipelineRun, *v1.PipelineRun) {
	t.Helper()
	resolvedChild := pinpChildTaskSpecPipeline(t, "resolved-child", namespace)

	parentPipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: parent-resolver
  namespace: %s
spec:
  tasks:
  - name: via-resolver
    pipelineRef:
      resolver: cluster
      params:
      - name: kind
        value: pipeline
      - name: name
        value: resolved-child
      - name: namespace
        value: %s
`, namespace, namespace))

	parentPipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: parent-resolver
`, parentPipelineRunName, namespace))

	expectedChildPipelineRun := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kmeta.ChildName(parentPipelineRunName, "-via-resolver"),
			Namespace: namespace,
			Labels:    map[string]string{pipeline.PipelineLabelKey: "resolved-child"},
		},
	}

	return parentPipeline, resolvedChild, parentPipelineRun, expectedChildPipelineRun
}
