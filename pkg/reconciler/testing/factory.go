package testing

import (
	"fmt"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

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

// OnePipelineRefInPipeline creates a standalone child Pipeline and a parent Pipeline that references
// the child via pipelineRef (instead of inline pipelineSpec). It also creates the according PipelineRun
// for it and the expected child PipelineRun against which the test will validate.
func OnePipelineRefInPipeline(t *testing.T, namespace, parentPipelineRunName string) (*v1.Pipeline, *v1.Pipeline, *v1.PipelineRun, *v1.PipelineRun) {
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
  tasks:
  - name: %s
    taskSpec:
      steps:
      - name: mystep
        image: mirror.gcr.io/busybox
        script: 'echo "Hello from child PipelineRun!"'
`, childPipelineName, namespace, childPipelineTaskName))

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

	expectedName := parentPipelineRunName + "-" + childPipelineName
	childObjectMeta := childPipelineRunWithObjectMeta(
		expectedName,
		namespace,
		parentPipelineRunName,
		parentPipelineName,
		childPipelineName,
		uid,
	)
	// Add cycle-detection ancestry annotations (set by the reconciler for PipelineRef tasks).
	childObjectMeta.Annotations = map[string]string{
		pipeline.PipelinePIPAncestryAnnotationKey: parentPipelineName,
		pipeline.PipelinePIPNameAnnotationKey:     childPipelineName,
	}
	expectedChildPipelineRun := parse.MustParseChildPipelineRunWithObjectMeta(
		t,
		childObjectMeta,
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

	return childPipeline, parentPipeline, parentPipelineRun, expectedChildPipelineRun
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
	expectedChildName := parentPipelineRunName + "-" + childPipelineName
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
	expectedGrandchildName := expectedChildName + "-" + grandchildPipelineName
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
