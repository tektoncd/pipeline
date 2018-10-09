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

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	listers "github.com/knative/build-pipeline/pkg/client/listers/pipeline/v1alpha1"
)

// getNextPipelineRunTaskRun returns the first TaskRun it find in pr's Pipeline (p)
// for which it does not find a corresponding TaskRun via l which can be run. It
// returns the name of the Task referenced in the Pipeline and the expected name of
// the TaskRun that should be created.
func getNextPipelineRunTaskRun(l listers.TaskRunLister, p *v1alpha1.Pipeline, prName string) (string, string, error) {
	for _, pt := range p.Spec.Tasks {
		trName := getTaskRunName(prName, &pt)
		_, listErr := l.TaskRuns(p.Namespace).Get(trName)
		if errors.IsNotFound(listErr) && canTaskRun(&pt) {
			return pt.Name, trName, nil
		}
	}
	return "", "", nil
}

func canTaskRun(pt *v1alpha1.PipelineTask) bool {
	// Check if Task can run now. Go through all the input constraints and see if
	// the upstream tasks have completed successfully and inputs are available.
	return true
}

// getTaskRunName should return a uniquie name for a `TaskRun`.
func getTaskRunName(prName string, pt *v1alpha1.PipelineTask) string {
	return fmt.Sprintf("%s-%s", prName, pt.Name)
}
