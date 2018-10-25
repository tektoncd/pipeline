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
	"strings"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
)

// Node represents a Task in a pipeline.
type Node struct {
	// Task represent the PipelineTask in Pipeline
	Task v1alpha1.PipelineTask
	// Prev represent all the Previous task Nodes for the current Task
	Prev []*Node
}

func (n Node) getPrevTasks() []v1alpha1.PipelineTask {
	p := make([]v1alpha1.PipelineTask, len(n.Prev))
	for i := range n.Prev {
		p[i] = n.Prev[i].Task
	}
	return p
}

func (n Node) printPrevTasks() string {
	s := make([]string, len(n.Prev))
	for i, n := range n.Prev {
		s[i] = n.Task.Name
	}
	return fmt.Sprintf("Previous tasks: [%s]", strings.Join(s, ","))
}

func (n Node) String() string {
	return fmt.Sprintf("PipelineTask.Name: %s, %s", n.Task.Name, n.printPrevTasks())
}
