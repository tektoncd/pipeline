// +build e2e

package e2e

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	"gotest.tools/v3/icmd"
	knativetest "knative.dev/pkg/test"
)

const (
	teTaskName = "te-task"
)

func TestTaskResourcesE2E(t *testing.T) {

	t.Helper()
	t.Parallel()

	// Setup New namespace, client for each Test
	c, namespace := Setup(t)
	knativetest.CleanupOnInterrupt(func() { TearDown(t, c, namespace) }, t.Logf)
	defer TearDown(t, c, namespace)

	//Creates Task Resource

	for i := 1; i <= 3; i++ {
		t.Logf("Creating Task Resource %s", teTaskName+"-"+strconv.Itoa(i))
		if _, err := c.TaskClient.Create(getTask(teTaskName+"-"+strconv.Itoa(i), namespace)); err != nil {
			t.Fatalf("Failed to create Task Resource `%s`: %s", teTaskName+"-"+strconv.Itoa(i), err)
		}
	}

	time.Sleep(1 * time.Second)

	run := Prepare(t)

	t.Run("Get list of Tasks from namespace  "+namespace, func(t *testing.T) {
		res := icmd.RunCmd(run("task", "list", "-n", namespace))

		expected := ListAllTasksOutput(t, c, map[int]interface{}{
			0: &TaskData{
				Name: teTaskName + "-1",
			},
			1: &TaskData{
				Name: teTaskName + "-2",
			},
			2: &TaskData{
				Name: teTaskName + "-3",
			},
		})

		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      icmd.None,
			Out:      expected,
		})

	})

	t.Run("Get list of Tasks from other namespace [default] should throw Error", func(t *testing.T) {
		res := icmd.RunCmd(run("task", "list", "-n", "default"))

		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      "No tasks found\n",
		})

	})

	t.Run("Validate Tasks list format for -o (output) flag, as Json Path ", func(t *testing.T) {
		res := icmd.RunCmd(run("task", "list", "-n", namespace,
			`-o=jsonpath={range.items[*]}{.metadata.name}{"\n"}{end}`))

		expected := ListResourceNamesForJsonPath(GetTaskListWithTestData(t, c, map[int]interface{}{
			0: &TaskData{
				Name: teTaskName + "-1",
			},
			1: &TaskData{
				Name: teTaskName + "-2",
			},
			2: &TaskData{
				Name: teTaskName + "-3",
			},
		}))

		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      icmd.None,
			Out:      expected,
		})

	})

	t.Run("Validate Taskruns Schema for -o (output) flag as Json ", func(t *testing.T) {
		res := icmd.RunCmd(run("task", "list", "-n", namespace, "-o", "json"))

		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      icmd.None,
		})
		err := json.Unmarshal([]byte(res.Stdout()), &v1alpha1.TaskList{})
		if err != nil {
			t.Errorf("error: %v", err)
		}
	})

	t.Run("Delete Task "+teTaskName+"-1"+" from namespace "+namespace+" without force flag, reply no", func(t *testing.T) {
		res := icmd.RunCmd(run("task", "rm", teTaskName+"-1", "-n", namespace),
			icmd.WithStdin(strings.NewReader("n")))

		res.Assert(t, icmd.Expected{
			ExitCode: 1,
			Err:      "Error: canceled deleting task \"" + teTaskName + "-1" + "\"\n",
		})

	})

	t.Run("Delete Task "+teTaskName+"-1"+" from namespace "+namespace+" without force flag, reply yes", func(t *testing.T) {
		res := icmd.RunCmd(run("task", "rm", teTaskName+"-1", "-n", namespace),
			icmd.WithStdin(strings.NewReader("y")))

		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      icmd.None,
			Out:      "Are you sure you want to delete task \"" + teTaskName + "-1" + "\" (y/n): Task deleted: " + teTaskName + "-1" + "\n",
		})

	})

	t.Run("Check for Task Resource, After Successfull Deletion of task in namespace "+namespace+" , Resource shouldn't exist", func(t *testing.T) {
		res := icmd.RunCmd(run("task", "list", "-n", namespace))

		expected := ListAllTasksOutput(t, c, map[int]interface{}{
			0: &TaskData{
				Name: teTaskName + "-2",
			},
			1: &TaskData{
				Name: teTaskName + "-3",
			},
		})

		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      icmd.None,
			Out:      expected,
		})
	})

}

func TestTaskDeleteE2EUsingCli(t *testing.T) {
	t.Helper()
	t.Parallel()

	c, namespace := Setup(t)
	knativetest.CleanupOnInterrupt(func() { TearDown(t, c, namespace) }, t.Logf)
	defer TearDown(t, c, namespace)

	for i := 1; i <= 3; i++ {
		t.Logf("Creating Task Resource %s", teTaskName+"-"+strconv.Itoa(i))
		if _, err := c.TaskClient.Create(getTask(teTaskName+"-"+strconv.Itoa(i), namespace)); err != nil {
			t.Fatalf("Failed to create Task Resource `%s`: %s", teTaskName+"-"+strconv.Itoa(i), err)
		}
	}

	time.Sleep(1 * time.Second)

	run := Prepare(t)

	t.Run("Delete Task "+teTaskName+"-1"+" from namespace "+namespace+" With force delete flag (shorthand)", func(t *testing.T) {
		res := icmd.RunCmd(run("task", "rm", teTaskName+"-1", "-n", namespace, "-f"))

		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      icmd.None,
			Out:      "Task deleted: " + teTaskName + "-1" + "\n",
		})

	})

	t.Run("Delete Task "+teTaskName+"-2"+" from namespace "+namespace+" With force delete flag", func(t *testing.T) {
		res := icmd.RunCmd(run("task", "rm", teTaskName+"-2", "-n", namespace, "--force"))

		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      icmd.None,
			Out:      "Task deleted: " + teTaskName + "-2" + "\n",
		})

	})

	t.Run("Delete Task "+teTaskName+"-3"+" from namespace "+namespace+" without force flag, reply no", func(t *testing.T) {
		res := icmd.RunCmd(run("task", "rm", teTaskName+"-3", "-n", namespace),
			icmd.WithStdin(strings.NewReader("n")))

		res.Assert(t, icmd.Expected{
			ExitCode: 1,
			Err:      "Error: canceled deleting task \"" + teTaskName + "-3" + "\"\n",
		})

	})

	t.Run("Delete Task "+teTaskName+"-3"+" from namespace "+namespace+" without force flag, reply yes", func(t *testing.T) {
		res := icmd.RunCmd(run("task", "rm", teTaskName+"-3", "-n", namespace),
			icmd.WithStdin(strings.NewReader("y")))

		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      icmd.None,
			Out:      "Are you sure you want to delete task \"" + teTaskName + "-3" + "\" (y/n): Task deleted: " + teTaskName + "-3" + "\n",
		})

	})

	t.Run("Get all available Tasks from namespace "+namespace+" should throw Error", func(t *testing.T) {
		res := icmd.RunCmd(run("task", "list", "-n", namespace))

		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      "No tasks found\n",
		})

	})

}

func getTask(taskname string, namespace string) *v1alpha1.Task {
	return tb.Task(taskname, namespace,
		tb.TaskSpec(tb.Step("amazing-busybox", "busybox", tb.StepCommand("/bin/sh"), tb.StepArgs("-c", "echo Hello")),
			tb.Step("amazing-busybox-1", "busybox", tb.StepCommand("/bin/sh"), tb.StepArgs("-c", "echo Welcome To Tekton!!")),
		))
}
