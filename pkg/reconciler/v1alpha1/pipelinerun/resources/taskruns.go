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

package resources

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
)

// GetTaskRun is a function that will retrieve the TaskRun name from namespace.
type GetTaskRun func(namespace, name string) (*v1alpha1.TaskRun, error)


// GetNextPipelineRunTaskRun returns the first TaskRun it find in pr's Pipeline (p)
// for which it does not find a corresponding TaskRun via g which can be run. It
// returns the name of the Task referenced in the Pipeline and the expected name of
// the TaskRun that should be created.
func GetNextPipelineRunTaskRun(g GetTaskRun, p *v1alpha1.Pipeline, prName string) (string, string, error) {
	for _, pt := range p.Spec.Tasks {
		trName := getTaskRunName(prName, &pt)
		_, err := g(p.Namespace, trName)
		if errors.IsNotFound(err) && canTaskRun(&pt) {
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
