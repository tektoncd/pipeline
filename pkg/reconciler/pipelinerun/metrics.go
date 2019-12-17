/*
Copyright 2019 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pipelinerun

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1alpha1"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"knative.dev/pkg/metrics"
)

var (
	prDuration = stats.Float64(
		"pipelinerun_duration_seconds",
		"The pipelinerun execution time in seconds",
		stats.UnitDimensionless)
	prDistributions = view.Distribution(10, 30, 60, 300, 900, 1800, 3600, 5400, 10800, 21600, 43200, 86400)

	prCount = stats.Float64("pipelinerun_count",
		"number of pipelineruns",
		stats.UnitDimensionless)

	runningPRsCount = stats.Float64("running_pipelineruns_count",
		"Number of pipelineruns executing currently",
		stats.UnitDimensionless)
)

type Recorder struct {
	initialized bool

	pipeline    tag.Key
	pipelineRun tag.Key
	namespace   tag.Key
	status      tag.Key
}

// NewRecorder creates a new metrics recorder instance
// to log the PipelineRun related metrics
func NewRecorder() (*Recorder, error) {
	r := &Recorder{
		initialized: true,
	}

	pipeline, err := tag.NewKey("pipeline")
	if err != nil {
		return nil, err
	}
	r.pipeline = pipeline

	pipelineRun, err := tag.NewKey("pipelinerun")
	if err != nil {
		return nil, err
	}
	r.pipelineRun = pipelineRun

	namespace, err := tag.NewKey("namespace")
	if err != nil {
		return nil, err
	}
	r.namespace = namespace

	status, err := tag.NewKey("status")
	if err != nil {
		return nil, err
	}
	r.status = status

	err = view.Register(
		&view.View{
			Description: prDuration.Description(),
			Measure:     prDuration,
			Aggregation: prDistributions,
			TagKeys:     []tag.Key{r.pipeline, r.pipelineRun, r.namespace, r.status},
		},
		&view.View{
			Description: prCount.Description(),
			Measure:     prCount,
			Aggregation: view.Count(),
			TagKeys:     []tag.Key{r.status},
		},
		&view.View{
			Description: runningPRsCount.Description(),
			Measure:     runningPRsCount,
			Aggregation: view.LastValue(),
		},
	)

	if err != nil {
		r.initialized = false
		return r, err
	}

	return r, nil
}

// DurationAndCount logs the duration of PipelineRun execution and
// count for number of PipelineRuns succeed or failed
// returns an error if its failed to log the metrics
func (r *Recorder) DurationAndCount(pr *v1alpha1.PipelineRun) error {
	if !r.initialized {
		return fmt.Errorf("ignoring the metrics recording for %s , failed to initialize the metrics recorder", pr.Name)
	}

	duration := time.Since(pr.Status.StartTime.Time)
	if pr.Status.CompletionTime != nil {
		duration = pr.Status.CompletionTime.Sub(pr.Status.StartTime.Time)
	}

	status := "success"
	if pr.Status.Conditions[0].Status == corev1.ConditionFalse {
		status = "failed"
	}

	pipelineName := "anonymous"
	if pr.Spec.PipelineRef != nil && pr.Spec.PipelineRef.Name != "" {
		pipelineName = pr.Spec.PipelineRef.Name
	}
	ctx, err := tag.New(
		context.Background(),
		tag.Insert(r.pipeline, pipelineName),
		tag.Insert(r.pipelineRun, pr.Name),
		tag.Insert(r.namespace, pr.Namespace),
		tag.Insert(r.status, status),
	)

	if err != nil {
		return err
	}

	metrics.Record(ctx, prDuration.M(float64(duration/time.Second)))
	metrics.Record(ctx, prCount.M(1))

	return nil
}

// RunningPipelineRuns logs the number of PipelineRuns running right now
// returns an error if its failed to log the metrics
func (r *Recorder) RunningPipelineRuns(lister listers.PipelineRunLister) error {
	if !r.initialized {
		return errors.New("ignoring the metrics recording, failed to initialize the metrics recorder")
	}

	prs, err := lister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list pipelineruns while generating metrics : %v", err)
	}

	var runningPRs int
	for _, pr := range prs {
		if !pr.IsDone() {
			runningPRs++
		}
	}

	ctx, err := tag.New(context.Background())
	if err != nil {
		return err
	}
	metrics.Record(ctx, runningPRsCount.M(float64(runningPRs)))

	return nil
}
