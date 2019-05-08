package pipeline

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/knative/pkg/apis"
	"github.com/tektoncd/cli/pkg/test"
	cb "github.com/tektoncd/cli/pkg/test/builder"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/pipelinerun/resources"
	pipelinetest "github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
)

func TestPipelinesList_empty(t *testing.T) {

	cs, _ := pipelinetest.SeedTestData(pipelinetest.Data{})
	p := &test.Params{Tekton: cs.Pipeline}

	pipeline := Command(p)
	output, err := test.ExecuteCommand(pipeline, "list", "-n", "foo")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := emptyMsg + "\n"
	if d := cmp.Diff(expected, output); d != "" {
		t.Errorf("Unexpected output mismatch: %s", d)
	}
}

func TestPipelineList_only_pipelines(t *testing.T) {
	pipelines := []pipelineDetails{
		{"tomatoes", 1 * time.Minute},
		{"mangoes", 20 * time.Second},
		{"bananas", 512 * time.Hour}, // 3 weeks
	}

	clock := clockwork.NewFakeClock()
	cs, _ := seedPipelines(clock, pipelines, "namespace")
	p := &test.Params{Tekton: cs.Pipeline, Clock: clock}

	pipeline := Command(p)
	output, err := test.ExecuteCommand(pipeline, "list", "-n", "namespace")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := []string{
		"NAME       AGE              LAST RUN   STARTED   DURATION   STATUS",
		"tomatoes   1 minute ago     ---        ---       ---        ---",
		"mangoes    20 seconds ago   ---        ---       ---        ---",
		"bananas    3 weeks ago      ---        ---       ---        ---",
		"",
	}

	text := strings.Join(expected, "\n")
	if d := cmp.Diff(text, output); d != "" {
		t.Errorf("Unexpected output mismatch: %s", d)
	}
}

func TestPipelinesList_with_single_run(t *testing.T) {
	clock := clockwork.NewFakeClock()

	cs, _ := pipelinetest.SeedTestData(pipelinetest.Data{
		Pipelines: []*v1alpha1.Pipeline{
			tb.Pipeline("pipeline", "ns",
				// created  5 minutes back
				cb.PipelineCreationTimestamp(clock.Now().Add(-5*time.Minute)),
			),
		},
		PipelineRuns: []*v1alpha1.PipelineRun{

			tb.PipelineRun("pipeline-run-1", "ns",
				cb.PipelineRunCreationTimestamp(clock.Now()),
				tb.PipelineRunLabel("tekton.dev/pipeline", "pipeline"),
				tb.PipelineRunSpec("pipeline"),
				tb.PipelineRunStatus(
					tb.PipelineRunStatusCondition(apis.Condition{
						Status: corev1.ConditionTrue,
						Reason: resources.ReasonSucceeded,
					}),
					// pipeline run starts now
					tb.PipelineRunStartTime(clock.Now()),
					// takes 10 minutes to complete
					cb.PipelineRunCompletionTime(clock.Now().Add(10*time.Minute)),
				),
			),
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock}
	pipeline := Command(p)

	// -5 : pipeline created
	//  0 : pipeline run - 1 started
	// 10 : pipeline run - 1 finished
	// 15 : <<< now run pipeline ls << - advance clock to this point

	clock.Advance(15 * time.Minute)
	got, err := test.ExecuteCommand(pipeline, "list", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := []string{
		"NAME       AGE              LAST RUN         STARTED          DURATION     STATUS",
		"pipeline   20 minutes ago   pipeline-run-1   15 minutes ago   10 minutes   Succeeded",
		"",
	}

	text := strings.Join(expected, "\n")
	if d := cmp.Diff(text, got); d != "" {
		t.Errorf("Unexpected output mismatch: \n%s\n", d)
	}
}
func TestPipelinesList_latest_run(t *testing.T) {
	clock := clockwork.NewFakeClock()
	//  Time --->
	//  |---5m ---|------------ ││--││------------- ---│--│
	//	now      pipeline       ││  │`secondRun stated │  `*first*RunCompleted
	//                          ││  `secondRun         `*second*RunCompleted
	//	                        │`firstRun started
	//	                        `firstRun
	// NOTE: firstRun completed **after** second but latest should still be
	// second run based on creationTimestamp

	var (
		pipelineCreated = clock.Now().Add(-5 * time.Minute)
		runDuration     = 5 * time.Minute

		firstRunCreated   = clock.Now().Add(10 * time.Minute)
		firstRunStarted   = firstRunCreated.Add(2 * time.Second)
		firstRunCompleted = firstRunStarted.Add(2 * runDuration) // take twice as long

		secondRunCreated   = firstRunCreated.Add(1 * time.Minute)
		secondRunStarted   = secondRunCreated.Add(2 * time.Second)
		secondRunCompleted = secondRunStarted.Add(runDuration) // takes less thus completes
	)

	cs, _ := pipelinetest.SeedTestData(pipelinetest.Data{
		Pipelines: []*v1alpha1.Pipeline{
			tb.Pipeline("pipeline", "ns",
				// created  5 minutes back
				cb.PipelineCreationTimestamp(pipelineCreated),
			),
		},
		PipelineRuns: []*v1alpha1.PipelineRun{

			tb.PipelineRun("pipeline-run-1", "ns",
				cb.PipelineRunCreationTimestamp(firstRunCreated),
				tb.PipelineRunLabel("tekton.dev/pipeline", "pipeline"),
				tb.PipelineRunSpec("pipeline"),
				tb.PipelineRunStatus(
					tb.PipelineRunStatusCondition(apis.Condition{
						Status: corev1.ConditionTrue,
						Reason: resources.ReasonSucceeded,
					}),
					tb.PipelineRunStartTime(firstRunStarted),
					cb.PipelineRunCompletionTime(firstRunCompleted),
				),
			),
			tb.PipelineRun("pipeline-run-2", "ns",
				cb.PipelineRunCreationTimestamp(secondRunCreated),
				tb.PipelineRunLabel("tekton.dev/pipeline", "pipeline"),
				tb.PipelineRunSpec("pipeline"),
				tb.PipelineRunStatus(
					tb.PipelineRunStatusCondition(apis.Condition{
						Status: corev1.ConditionTrue,
						Reason: resources.ReasonSucceeded,
					}),
					tb.PipelineRunStartTime(secondRunStarted),
					cb.PipelineRunCompletionTime(secondRunCompleted),
				),
			),
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock}
	pipeline := Command(p)

	clock.Advance(30 * time.Minute)

	got, err := test.ExecuteCommand(pipeline, "list", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := []string{
		"NAME       AGE              LAST RUN         STARTED          DURATION    STATUS",
		"pipeline   35 minutes ago   pipeline-run-2   18 minutes ago   5 minutes   Succeeded",
		"",
	}

	text := strings.Join(expected, "\n")
	if d := cmp.Diff(text, got); d != "" {
		t.Errorf("Unexpected output mismatch: \n%s\n", d)
	}
}

type pipelineDetails struct {
	name string
	age  time.Duration
}

func seedPipelines(clock clockwork.Clock, ps []pipelineDetails, ns string) (pipelinetest.Clients, pipelinetest.Informers) {
	pipelines := []*v1alpha1.Pipeline{}
	for _, p := range ps {
		pipelines = append(pipelines,
			tb.Pipeline(p.name, ns,
				cb.PipelineCreationTimestamp(clock.Now().Add(p.age*-1)),
			),
		)
	}

	return pipelinetest.SeedTestData(pipelinetest.Data{Pipelines: pipelines})
}
