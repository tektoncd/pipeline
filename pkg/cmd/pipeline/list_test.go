package pipeline

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
)

func TestPipelineList(t *testing.T) {
	pipeline := tb.Pipeline("tomatoes", "foo",
		tb.PipelineSpec(
			tb.PipelineDeclaredResource("my-only-git-resource", "git"),
			tb.PipelineDeclaredResource("my-only-image-resource", "image"),
		))
	test.SeedTestData(test.Data{
		Pipelines: []*v1alpha1.Pipeline{pipeline},
	})
	// NOTE: For now, this servers as a validation that `go test` works fine
	// and that all dependencies are in place

	// TODO (sthaha) complete the test
}
