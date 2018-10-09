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

package pipeline

import (
	"fmt"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	listers "github.com/knative/build-pipeline/pkg/client/listers/pipeline/v1alpha1"
)

// GetTasks retrieves all Tasks instances which the pipeline p references, using
// lister l. If it is unable to retrieve an instance of a references Task, it will return
// an error, otherwise it returns a map from the name of the Task in the Pipeline to the
// name of the Task object itself.
func GetTasks(l listers.TaskLister, p *v1alpha1.Pipeline) (map[string]*v1alpha1.Task, error) {
	tasks := map[string]*v1alpha1.Task{}
	for _, pt := range p.Spec.Tasks {
		t, err := l.Tasks(p.Namespace).Get(pt.TaskRef.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to get tasks for Pipeline %q: Error getting task %q : %s",
				fmt.Sprintf("%s/%s", p.Namespace, p.Name),
				fmt.Sprintf("%s/%s", p.Namespace, pt.TaskRef.Name), err)
		}
		tasks[pt.Name] = t
	}
	return tasks, nil
}
