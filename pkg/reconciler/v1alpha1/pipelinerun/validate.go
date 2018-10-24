/*
Copyright 2018 The Knative Authors

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
	"fmt"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
)

// validate all ref exist
func (c *Reconciler) validatePipelineRun(pr *v1alpha1.PipelineRun) (*v1alpha1.Pipeline, string, error) {
	// verify pipeline reference and all corresponding tasks exist
	p, err := c.pipelineLister.Pipelines(pr.Namespace).Get(pr.Spec.PipelineRef.Name)
	if err != nil {
		return nil, "", fmt.Errorf("Error listing pipeline ref %s: %v", pr.Spec.PipelineRef.Name, err)
	}
	for _, t := range p.Spec.Tasks {
		if _, terr := c.taskLister.Tasks(pr.Namespace).Get(t.TaskRef.Name); terr != nil {
			return nil, "", fmt.Errorf("Error listing task %s for pipeline %s: %v", t.TaskRef.Name, p.Name, terr)
		}
	}

	var sa string
	if pr.Spec.PipelineParamsRef.Name != "" {
		pp, err := c.pipelineParamsLister.PipelineParamses(pr.Namespace).Get(pr.Spec.PipelineParamsRef.Name)
		if err != nil {
			return nil, "", fmt.Errorf("Error listing pipelineparams %s for pipelinerun %s: %v", pr.Spec.PipelineParamsRef.Name, pr.Name, err)
		}
		sa = pp.Spec.ServiceAccount
	}
	return p, sa, nil
}
