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

package stepper

import (
	"context"
	"github.com/pkg/errors"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/remote"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"strconv"
)

type Resolver struct {
	ResolveRemote func(ctx context.Context, uses *v1beta1.Uses) (remote.Resolver, error)
}

// NewResolver creates a new stepper resolver
func NewResolver(opts *RemoterOptions) *Resolver {
	if opts.Logger == nil {
		observer, _ := observer.New(zap.InfoLevel)
		opts.Logger = zap.New(observer).Sugar()
	}

	cachingResolver := NewCachingRemoter(opts.CreateRemote)

	return &Resolver{
		ResolveRemote: cachingResolver.CreateRemote,
	}
}

// Resolve will resolve any `uses` structs on any steps in any tasks
func (r *Resolver) Resolve(ctx context.Context, prs *v1beta1.PipelineRun) error {
	ps := prs.Spec.PipelineSpec
	if ps == nil {
		return nil
	}
	for i := range ps.Tasks {
		pt := &ps.Tasks[i]
		if pt.TaskSpec != nil {
			ts := &pt.TaskSpec.TaskSpec
			var steps []v1beta1.Step
			for j := range ts.Steps {
				step := ts.Steps[j]
				uses := combineUsesTemplate(step.Uses, ts.UsesTemplate)
				if uses == nil {
					steps = append(steps, step)
					continue
				}
				taskName := pt.Name
				if taskName == "" {
					taskName = strconv.Itoa(i)
				}
				stepName := step.Name
				if stepName == "" {
					stepName = strconv.Itoa(j)
				}
				replaceSteps, err := r.UsesSteps(ctx, uses, ps, pt, step)
				if err != nil {
					return errors.Wrapf(err, "failed to use %s for step %s/%s", uses.String(), taskName, stepName)
				}
				if len(replaceSteps) == 0 {
					return errors.Errorf("no steps found for use %s on step %s/%s", uses.String(), taskName, stepName)
				}
				steps = append(steps, replaceSteps...)
			}
			ts.Steps = steps
			ts.UsesTemplate = nil
		}
	}
	return nil
}

// UsesSteps lets resolve the uses.String() to a PipelineRun and find the step or steps
// for the given task name and/or step name then lets apply any overrides from the step
func (r *Resolver) UsesSteps(ctx context.Context, uses *v1beta1.Uses, ps *v1beta1.PipelineSpec, pt *v1beta1.PipelineTask, step v1beta1.Step) ([]v1beta1.Step, error) {
	remote, err := r.ResolveRemote(ctx, uses)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve remote for uses %s", uses.String())
	}

	obj, err := remote.Get("task", uses.Path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve uses %s", uses.Path)
	}
	if obj == nil {
		return nil, errors.Errorf("no task found for path %s", uses.Path)
	}

	// PipelineRuns
	if pr, ok := obj.(*v1beta1.PipelineRun); ok {
		ps := pr.Spec.PipelineSpec
		if ps == nil {
			return nil, errors.Errorf("the PipelineRun %s at path %s has no PipelineSpec", pr.Name, uses.Path)
		}
		return r.findPipelineSteps(ctx, uses, ps, pt, step)
	}
	if pr, ok := obj.(*v1alpha1.PipelineRun); ok {
		betaPR := &v1beta1.PipelineRun{}
		err := pr.ConvertTo(ctx, betaPR)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to convert v1alpha1.PipelineRun")
		}
		ps := betaPR.Spec.PipelineSpec
		if ps == nil {
			return nil, errors.Errorf("the PipelineRun %s at path %s has no PipelineSpec", pr.Name, uses.Path)
		}
		return r.findPipelineSteps(ctx, uses, ps, pt, step)
	}

	// Pipelines
	if pipeline, ok := obj.(v1beta1.PipelineObject); ok {
		ps := pipeline.PipelineSpec()
		return r.findPipelineSteps(ctx, uses, &ps, pt, step)
	}
	if pipeline, ok := obj.(*v1alpha1.Pipeline); ok {
		betaPipeline := &v1beta1.Pipeline{}
		err := pipeline.ConvertTo(ctx, betaPipeline)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to convert v1alpha1.Pipeline")
		}
		return r.findPipelineSteps(ctx, uses, &betaPipeline.Spec, pt, step)
	}

	// Tasks
	if task, ok := obj.(v1beta1.TaskObject); ok {
		ts := task.TaskSpec()
		return r.findTaskSteps(ctx, uses, task.TaskMetadata().Name, &ts, ps, pt, step)
	}
	if task, ok := obj.(*v1alpha1.Task); ok {
		betaTask := &v1beta1.Task{}
		err := task.ConvertTo(ctx, betaTask)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to convert v1alpha1.Task")
		}
		return r.findTaskSteps(ctx, uses, betaTask.Name, &betaTask.Spec, ps, pt, step)
	}

	return nil, errors.Errorf("could not convert object %#v to task for path %s", obj, uses.Path)
}

func (r *Resolver) findPipelineSteps(ctx context.Context, uses *v1beta1.Uses, ps *v1beta1.PipelineSpec, pt *v1beta1.PipelineTask, step v1beta1.Step) ([]v1beta1.Step, error) {
	taskName := pt.Name
	if uses.Task != "" {
		taskName = uses.Task
	}
	pipelineTasks := ps.Tasks
	for _, task := range pipelineTasks {
		if task.Name == taskName || taskName == "" {
			ts := task.TaskSpec
			if ts != nil {
				return r.findTaskSteps(ctx, uses, task.Name, &ts.TaskSpec, ps, pt, step)
			}
		}
	}
	return nil, errors.Errorf("uses %s has no spec.pipelineSpec.tasks that match task name %s", uses.String(), taskName)
}

func (r *Resolver) findTaskSteps(ctx context.Context, uses *v1beta1.Uses, taskName string, ts *v1beta1.TaskSpec, ps *v1beta1.PipelineSpec, pt *v1beta1.PipelineTask, step v1beta1.Step) ([]v1beta1.Step, error) {
	err := UseParametersAndResults(ps, pt, ts)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to ")
	}

	if ts == nil {
		return nil, errors.Errorf("uses %s has no task spec for task %s", uses.String(), taskName)
	}
	name := uses.Step
	if name == "" {
		return ts.Steps, nil
	}

	for i := range ts.Steps {
		s := &ts.Steps[i]
		if s.Name == name {
			replaceStep := *s
			OverrideStep(&replaceStep, &step)
			return []v1beta1.Step{replaceStep}, nil
		}
	}
	return nil, errors.Errorf("uses %s task %s has no step named %s", uses.String(), taskName, name)
}

// combineUsesTemplate lets share the previous uses values so we can avoid repeating paths for each step
func combineUsesTemplate(u1 *v1beta1.Uses, u2 *v1beta1.Uses) *v1beta1.Uses {
	if u1 == nil {
		return nil
	}
	if u2 == nil {
		return u1
	}
	result := *u1
	if u2.Path != "" && result.Path == "" {
		result.Path = u2.Path
	}
	return &result
}
