// +build e2e

package e2e

import (
	"encoding/json"
	"log"
	"strings"
	"testing"
	"time"

	knativetest "knative.dev/pkg/test"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/icmd"
)

const (
	teTaskRunName = "te-task-run"
)

func TestTaskRunE2EUsingCli(t *testing.T) {
	t.Helper()
	t.Parallel()

	c, namespace := Setup(t)

	knativetest.CleanupOnInterrupt(func() { TearDown(t, c, namespace) }, t.Logf)
	defer TearDown(t, c, namespace)

	t.Logf("Creating Task Run Resource  %s", teTaskRunName)
	if _, err := c.TaskRunClient.Create(getTaskRun(c, teTaskName, teTaskRunName, namespace)); err != nil {
		t.Fatalf("Failed to create Task Run Resource `%s`: %s", teTaskRunName, err)
	}

	WaitForTaskRunToComplete(c, teTaskRunName, namespace)

	run := Prepare(t)

	t.Run("Get list of Taskruns from namespace  "+namespace, func(t *testing.T) {

		res := icmd.RunCmd(run("taskrun", "list", "-n", namespace))

		expected := ListAllTaskRunsOutput(t, c, map[int]interface{}{
			0: &TaskRunData{
				Name:   teTaskRunName,
				Status: "Succeeded",
			},
		})

		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      icmd.None,
			Out:      expected,
		})
	})

	t.Run("Get list of Taskruns from other namespace [default] should throw Error", func(t *testing.T) {
		res := icmd.RunCmd(run("taskrun", "list", "-n", "default"))
		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      "No taskruns found\n",
		})

	})

	t.Run("Validate Taskruns format for -o (output) flag, as Json Path ", func(t *testing.T) {
		res := icmd.RunCmd(run("taskrun", "list", "-n", namespace,
			`-o=jsonpath={range.items[*]}{.metadata.name}{"\n"}{end}`))

		expected := ListResourceNamesForJsonPath(
			GetTaskRunListWithTestData(t, c,
				map[int]interface{}{
					0: &TaskRunData{
						Name:   teTaskRunName,
						Status: "Succeeded",
					},
				}))

		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      icmd.None,
			Out:      expected,
		})
	})

	t.Run("Validate Taskruns Schema for -o (output) flag as Json ", func(t *testing.T) {
		res := icmd.RunCmd(run("taskrun", "list", "-n", namespace, "-o", "json"))

		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      icmd.None,
		})
		err := json.Unmarshal([]byte(res.Stdout()), &v1alpha1.TaskRunList{})
		if err != nil {
			log.Fatalf("error: %v", err)
		}
	})

	t.Run("Validate Taskrun logs using follow flag (-f), which streams logs to console ", func(t *testing.T) {

		expected := map[int]interface{}{
			0: []string{`.*(\[amazing-busybox\].*Hello)`, `.*(\[amazing-busybox-1\].*Welcome To Tekton!!)`},
		}

		for i, tr := range GetTaskRunList(c).Items {
			res := icmd.RunCmd(run("taskrun", "logs", "-f", tr.Name, "-n", namespace))
			res.Assert(t, icmd.Expected{
				ExitCode: 0,
				Err:      icmd.None,
			})

			for _, logRegex := range expected[i].([]string) {
				assert.Assert(t, is.Regexp(logRegex, res.Stdout()))
			}

		}
	})

	t.Run("Validate Taskrun logs using all containers flag (-a) ", func(t *testing.T) {

		expected := map[int]interface{}{
			0: []string{`.*(\[amazing-busybox\].*Hello)`, `.*(\[amazing-busybox-1\].*Welcome To Tekton!!)`},
		}

		for i, tr := range GetTaskRunList(c).Items {
			res := icmd.RunCmd(run("taskrun", "logs", "-a", tr.Name, "-n", namespace))

			res.Assert(t, icmd.Expected{
				ExitCode: 0,
				Err:      icmd.None,
			})

			for _, logRegex := range expected[i].([]string) {
				assert.Assert(t, is.Regexp(logRegex, res.Stdout()))
			}
		}

	})

	t.Run("Validate Taskrun describe command", func(t *testing.T) {
		res := icmd.RunCmd(run("taskrun", "describe", teTaskRunName, "-n", namespace))

		expected := GetTaskRunDescribeOutput(t, c, teTaskRunName,
			map[int]interface{}{
				0: &TaskRunDescribeData{
					Name:           teTaskRunName,
					Namespace:      namespace,
					TaskRef:        teTaskName,
					ServiceAccount: "",
					Status:         "Succeeded",
					FailureMessage: "",
					Input:          map[string]string{},
					Output:         map[string]string{},
					Params:         map[string]interface{}{},
					Steps:          []string{"amazing-busybox", "amazing-busybox-1"},
				},
			})

		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      icmd.None,
			Out:      expected,
		})

	})

}

func TestTaskRunCancelAndDeleteE2EUsingCli(t *testing.T) {
	t.Helper()
	t.Parallel()

	c, namespace := Setup(t)

	knativetest.CleanupOnInterrupt(func() { TearDown(t, c, namespace) }, t.Logf)
	defer TearDown(t, c, namespace)

	t.Logf("Creating Task Run Resource %s", teTaskRunName)
	if _, err := c.TaskRunClient.Create(getTaskRun(c, teTaskName, teTaskRunName, namespace)); err != nil {
		t.Fatalf("Failed to create Task Run Resource `%s`: %s", teTaskRunName, err)
	}

	WaitForTaskRunToBeStarted(c, teTaskRunName, namespace)

	run := Prepare(t)

	t.Run("Cancel Running Task Run "+teTaskRunName+" in namespace "+namespace, func(t *testing.T) {

		res := icmd.RunCmd(run("taskrun", "cancel", teTaskRunName, "-n", namespace))

		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      icmd.None,
			Out:      "TaskRun cancelled: " + teTaskRunName + "\n",
		})

	})

	time.Sleep(1 * time.Second)

	t.Run("Check for Error message and status of Taskrun after Task Run Cancelled Successfully", func(t *testing.T) {
		res := icmd.RunCmd(run("taskrun", "describe", teTaskRunName, "-n", namespace))

		expected := GetTaskRunDescribeOutput(t, c, teTaskRunName,
			map[int]interface{}{
				0: &TaskRunDescribeData{
					Name:           teTaskRunName,
					Namespace:      namespace,
					TaskRef:        teTaskName,
					ServiceAccount: "",
					Status:         "TaskRunCancelled",
					FailureMessage: "TaskRun \"" + teTaskRunName + "\" was cancelled",
					Input:          map[string]string{},
					Output:         map[string]string{},
					Params:         map[string]interface{}{},
					Steps:          []string{"amazing-busybox", "amazing-busybox-1"},
				},
			})

		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      icmd.None,
			Out:      expected,
		})

	})

	t.Run("Delete Taskrun "+teTaskRunName+" from namespace "+namespace+" Without force delete flag, reply no", func(t *testing.T) {
		res := icmd.RunCmd(run("taskrun", "rm", teTaskRunName, "-n", namespace),
			icmd.WithStdin(strings.NewReader("n")))

		res.Assert(t, icmd.Expected{
			ExitCode: 1,
			Err:      "Error: canceled deleting taskrun \"" + teTaskRunName + "\"\n",
		})

	})

	t.Run("Delete Taskrun "+teTaskRunName+" from namespace "+namespace+" Without force delete flag, reply yes", func(t *testing.T) {
		res := icmd.RunCmd(run("taskrun", "rm", teTaskRunName, "-n", namespace),
			icmd.WithStdin(strings.NewReader("y")))

		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      icmd.None,
			Out:      "Are you sure you want to delete taskrun \"" + teTaskRunName + "\" (y/n): TaskRun deleted: " + teTaskRunName + "\n",
		})

	})

	t.Run("Check  TaskRun "+teTaskRunName+" shouldn't exists in namespace "+namespace, func(t *testing.T) {
		res := icmd.RunCmd(run("taskrun", "list", "-n", namespace))
		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      "No taskruns found\n",
		})
	})

}

func getTaskRun(c *Clients, taskName string, taskRunName string, namespace string) *v1alpha1.TaskRun {
	log.Printf("Creating Task and TaskRun in namespace %s", namespace)
	task := tb.Task(taskName, namespace,
		tb.TaskSpec(tb.Step("amazing-busybox", "busybox", tb.StepCommand("/bin/sh"), tb.StepArgs("-c", "echo Hello")),
			tb.Step("amazing-busybox-1", "busybox", tb.StepCommand("/bin/sh"), tb.StepArgs("-c", "echo Welcome To Tekton!!")),
		))
	if _, err := c.TaskClient.Create(task); err != nil {
		log.Fatalf("Failed to create Task `%s`: %s", task.Name, err)
	}
	return tb.TaskRun(taskRunName, namespace, tb.TaskRunSpec(tb.TaskRunTaskRef(task.Name)))
}
