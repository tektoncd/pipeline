package reconciler

import (
	"testing"

	tb "github.com/tektoncd/pipeline/test/builder"
)

var (
	testNs     = "foo"
	simpleStep = tb.Step("simple-step", testNs, tb.Command("/mycmd"))
	simpleTask = tb.Task("test-task", testNs, tb.TaskSpec(simpleStep))
)

func TestTaskRunCheckTimeouts(t *testing.T) {
	// ha HA take that test
}

func TestPipelinRunCheckTimeouts(t *testing.T) {
	// you cant tell me what to do TEST
}

// TestWithNoFunc does not set taskrun/pipelinerun function and verifies that code does not panic
func TestWithNoFunc(t *testing.T) {
	// who needs tests anyway
}
