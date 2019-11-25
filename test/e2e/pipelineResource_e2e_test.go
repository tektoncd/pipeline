// +build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	"gotest.tools/v3/icmd"
	knativetest "knative.dev/pkg/test"
)

func TestPipelineResourceDeleteE2EUsingCli(t *testing.T) {
	PipelineResourceName := []string{"go-example-git", "go-example-git-1", "go-example-git-2"}
	c, namespace := Setup(t)
	knativetest.CleanupOnInterrupt(func() { TearDown(t, c, namespace) }, t.Logf)
	defer TearDown(t, c, namespace)

	for _, prname := range PipelineResourceName {
		t.Logf("Creating Git PipelineResource %s", prname)
		if _, err := c.PipelineResourceClient.Create(getGitResource(prname, namespace)); err != nil {
			t.Fatalf("Failed to create Pipeline Resource `%s`: %s", prname, err)
		}
	}

	t.Run("Remove pipeline resources With force delete flag (shorthand)", func(t *testing.T) {
		run := Prepare(t)
		res := icmd.RunCmd(run("resource", "rm", PipelineResourceName[0], "-n", namespace, "-f"))
		fmt.Printf("Output : %+v", res.Stdout())
		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      icmd.None,
			Out:      "PipelineResource deleted: " + PipelineResourceName[0] + "\n",
		})

	})

	t.Run("Remove pipeline resources With force delete flag", func(t *testing.T) {
		run := Prepare(t)
		res := icmd.RunCmd(run("resource", "rm", PipelineResourceName[1], "-n", namespace, "--force"))
		fmt.Printf("Output : %+v", res.Stdout())
		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      icmd.None,
			Out:      "PipelineResource deleted: " + PipelineResourceName[1] + "\n",
		})

	})

	t.Run("Remove pipeline resources Without force delete flag, reply no", func(t *testing.T) {
		run := Prepare(t)
		res := icmd.RunCmd(run("resource", "rm", PipelineResourceName[2], "-n", namespace), icmd.WithStdin(strings.NewReader("n")))
		fmt.Printf("Output : %+v", res.Stdout())
		res.Assert(t, icmd.Expected{
			ExitCode: 1,
			Err:      "Error: canceled deleting pipelineresource \"" + PipelineResourceName[2] + "\"\n",
		})

	})

	t.Run("Remove pipeline resources Without force delete flag, reply yes", func(t *testing.T) {
		run := Prepare(t)
		res := icmd.RunCmd(run("resource", "rm", PipelineResourceName[2], "-n", namespace), icmd.WithStdin(strings.NewReader("y")))
		fmt.Printf("Output : %+v", res.Stdout())
		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      icmd.None,
			Out:      "Are you sure you want to delete pipelineresource \"" + PipelineResourceName[2] + "\" (y/n): PipelineResource deleted: " + PipelineResourceName[2] + "\n",
		})
	})
}

func TestPipelineResourcesE2EUsingCli(t *testing.T) {

	PipelineResourceName := []string{"go-example-git", "go-example-image"}
	c, namespace := Setup(t)
	knativetest.CleanupOnInterrupt(func() { TearDown(t, c, namespace) }, t.Logf)
	defer TearDown(t, c, namespace)

	run := Prepare(t)

	t.Logf("Creating Git PipelineResource %s", PipelineResourceName[0])
	if _, err := c.PipelineResourceClient.Create(getGitResource(PipelineResourceName[0], namespace)); err != nil {
		t.Fatalf("Failed to create Pipeline Resource `%s`: %s", PipelineResourceName[0], err)
	}

	t.Logf("Creating Image PipelineResource %s", PipelineResourceName[1])
	if _, err := c.PipelineResourceClient.Create(getImageResource(PipelineResourceName[1], namespace)); err != nil {
		t.Fatalf("Failed to create Pipeline Resource `%s`: %s", PipelineResourceName[1], err)
	}

	t.Run("Get list of Pipeline Resources from namespace  "+namespace, func(t *testing.T) {
		res := icmd.RunCmd(run("resources", "list", "-n", namespace))
		expected := ListAllPipelineResourcesOutput(t, c,
			map[int]interface{}{
				0: &PipelineResourcesData{
					Name:    PipelineResourceName[0],
					Type:    "git",
					Details: "https://github.com/GoogleContainerTools/kaniko",
				},
				1: &PipelineResourcesData{
					Name:    PipelineResourceName[1],
					Type:    "image",
					Details: "gcr.io/staging-images/kritis",
				}},
		)
		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      icmd.None,
		})

		if d := cmp.Diff(expected, res.Stdout()); d != "" {
			t.Errorf("Unexpected output mismatch: \n%s\n", d)
		}

	})

	t.Run("Get list of Pipeline Resources from other namespace [default] should throw Error", func(t *testing.T) {
		res := icmd.RunCmd(run("resources", "list", "-n", "default"))
		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      "No pipelineresources found.\n",
		})

		if d := cmp.Diff("No pipelineresources found.\n", res.Stderr()); d != "" {
			t.Errorf("Unexpected output mismatch: \n%s\n", d)
		}
	})

	t.Run("Validate Pipeline Resources format for -o (output) flag,  Json Path ", func(t *testing.T) {
		res := icmd.RunCmd(run("resource", "list", "-n", namespace,
			`-o=jsonpath={range.items[*]}{.metadata.name}{"\n"}{end}`))

		expected := ListResourceNamesForJsonPath(GetPipelineResourceListWithTestData(t, c,
			map[int]interface{}{
				0: &PipelineResourcesData{
					Name:    PipelineResourceName[0],
					Type:    "git",
					Details: "https://github.com/GoogleContainerTools/kaniko",
				},
				1: &PipelineResourcesData{
					Name:    PipelineResourceName[1],
					Type:    "image",
					Details: "gcr.io/staging-images/kritis",
				}}))

		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      icmd.None,
		})
		if d := cmp.Diff(expected, res.Stdout()); d != "" {
			t.Errorf("Unexpected output mismatch: \n%s\n", d)
		}
	})

	t.Run("Pipeline Resources json Schema validation with -o (output) flag, as Json ", func(t *testing.T) {
		res := icmd.RunCmd(run("resource", "list", "-n", namespace, "-o", "json"))

		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      icmd.None,
		})
		err := json.Unmarshal([]byte(res.Stdout()), &v1alpha1.PipelineResourceList{})
		if err != nil {
			log.Fatalf("error: %v", err)
		}
	})

	t.Run("Validate  Pipeline Resources(git) describe command", func(t *testing.T) {
		res := icmd.RunCmd(run("resource", "describe", PipelineResourceName[0], "-n", namespace))

		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      icmd.None,
		})

		expected := GetPipelineResourceDescribeOutput(t, c, PipelineResourceName[0],
			map[int]interface{}{

				0: &PipelineResourcesDescribeData{
					Name:                 PipelineResourceName[0],
					Namespace:            namespace,
					PipelineResourceType: "git",
					Params: map[string]string{
						"Url":      "https://github.com/GoogleContainerTools/kaniko",
						"Revision": "master",
					},
					SecretParams: map[string]string{},
				},
			})
		if d := cmp.Diff(expected, res.Stdout()); d != "" {
			t.Errorf("Unexpected output mismatch: \n%s\n", d)
		}

	})

	t.Run("Validate  Pipeline Resources(image) describe command", func(t *testing.T) {
		res := icmd.RunCmd(run("resource", "describe", PipelineResourceName[1], "-n", namespace))

		res.Assert(t, icmd.Expected{
			ExitCode: 0,
			Err:      icmd.None,
		})

		expected := GetPipelineResourceDescribeOutput(t, c, PipelineResourceName[1],
			map[int]interface{}{

				0: &PipelineResourcesDescribeData{
					Name:                 PipelineResourceName[0],
					Namespace:            namespace,
					PipelineResourceType: "image",
					Params: map[string]string{
						"Url": "gcr.io/staging-images/kritis",
					},
					SecretParams: map[string]string{},
				},
			})
		if d := cmp.Diff(expected, res.Stdout()); d != "" {
			t.Errorf("Unexpected output mismatch: \n%s\n", d)
		}

	})

}

func getGitResource(prname string, namespace string) *v1alpha1.PipelineResource {
	return tb.PipelineResource(prname, namespace, tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeGit,
		tb.PipelineResourceSpecParam("Url", "https://github.com/GoogleContainerTools/kaniko"),
		tb.PipelineResourceSpecParam("Revision", "master"),
	))
}

func getImageResource(prname string, namespace string) *v1alpha1.PipelineResource {
	return tb.PipelineResource(prname, namespace, tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeImage,
		tb.PipelineResourceSpecParam("Url", "gcr.io/staging-images/kritis"),
	))
}
