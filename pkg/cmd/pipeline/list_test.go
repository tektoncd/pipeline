package pipeline

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
)

type TestParams struct {
	ns, kc string
	client versioned.Interface
}

func (p *TestParams) SetNamespace(ns string) {
	p.ns = ns
}
func (p *TestParams) Namespace() string {
	return p.ns
}

func (p *TestParams) SetKubeConfigPath(path string) {
	p.kc = path
}
func (p *TestParams) KubeConfigPath() string {
	return p.kc
}

func (p *TestParams) Clientset() (versioned.Interface, error) {
	return p.client, nil
}

var _ cli.Params = &TestParams{}

func executeCommand(root *cobra.Command, args ...string) (output string, err error) {
	_, output, err = executeCommandC(root, args...)
	return output, err
}

func executeCommandC(root *cobra.Command, args ...string) (c *cobra.Command, output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOutput(buf)
	root.SetArgs(args)

	c, err = root.ExecuteC()

	return c, buf.String(), err
}

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
	p := &TestParams{client: cs.Pipeline}

	pipeline := Command(p)
	output, err := executeCommand(pipeline, "list", "-n", "foo")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	//  account for new line at the end
	expected := strings.Join(append(names, ""), "\n")
	if d := cmp.Diff(expected, output); d != "" {
		t.Errorf("Unexpected output mismatch: %s", d)
	}
}
