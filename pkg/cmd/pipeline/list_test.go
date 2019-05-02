package pipeline

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/cli/pkg/testutil"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
)

func TestPipelineList(t *testing.T) {
	names := []string{
		"tomatoes",
		"mangoes",
		"bananas",
	}

	pipelines := []*v1alpha1.Pipeline{}
	for _, name := range names {
		pipelines = append(pipelines, tb.Pipeline(name, "foo", tb.PipelineSpec()))
	}

	cs, _ := test.SeedTestData(test.Data{Pipelines: pipelines})
	p := &testutil.TestParams{Client: cs.Pipeline}

	pipeline := Command(p)
	output, err := testutil.ExecuteCommand(pipeline, "list", "-n", "foo")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	//  account for new line at the end
	expected := strings.Join(append(names, ""), "\n")
	if d := cmp.Diff(expected, output); d != "" {
		t.Errorf("Unexpected output mismatch: %s", d)
	}
}
