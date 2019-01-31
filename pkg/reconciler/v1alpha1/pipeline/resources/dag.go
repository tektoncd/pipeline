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
	errors "github.com/knative/build-pipeline/pkg/errors"
)

// DAG represents the Pipeline DAG
type DAG struct {
	//Nodes represent map of PipelineTask name to Node in Pipeline DAG
	Nodes map[string]*Node
}

// Returns an empty Pipeline DAG
func new() *DAG {
	return &DAG{Nodes: map[string]*Node{}}
}

func (g *DAG) addPipelineTask(t v1alpha1.PipelineTask) (*Node, error) {
	if _, ok := g.Nodes[t.Name]; ok {
		return nil, fmt.Errorf("duplicate pipeline taks")
	}
	newNode := &Node{
		Task: t,
	}
	g.Nodes[t.Name] = newNode
	return newNode, nil
}

func (g *DAG) addPrevPipelineTask(prev *Node, next *Node) error {
	// Check for self cycle
	if prev.Task.Name == next.Task.Name {
		return fmt.Errorf("cycle detected; task %q depends on itself", next.Task.Name)
	}
	// Check if we are adding cycles.
	visited := map[string]bool{prev.Task.Name: true, next.Task.Name: true}
	path := []string{next.Task.Name, prev.Task.Name}
	if err := visit(prev.Prev, path, visited); err != nil {
		return fmt.Errorf("cycle detected; %s ", err.Error())
	}
	next.Prev = append(next.Prev, prev)
	return nil
}

func visit(nodes []*Node, path []string, visited map[string]bool) error {
	for _, n := range nodes {
		path = append(path, n.Task.Name)
		if _, ok := visited[n.Task.Name]; ok {
			return fmt.Errorf(getVisitedPath(path))
		}
		visited[n.Task.Name] = true
		if err := visit(n.Prev, path, visited); err != nil {
			return err
		}
	}
	return nil
}

func getVisitedPath(path []string) string {
	// Reverse the path since we traversed the graph using prev pointers.
	for i := len(path)/2 - 1; i >= 0; i-- {
		opp := len(path) - 1 - i
		path[i], path[opp] = path[opp], path[i]
	}
	return strings.Join(path, " -> ")
}

//GetPreviousTasks return all the previous tasks for a PipelineTask in the DAG
func (g *DAG) GetPreviousTasks(pt string) []v1alpha1.PipelineTask {
	v, ok := g.Nodes[pt]
	if !ok {
		return nil
	}
	return v.getPrevTasks()
}

// Build returns a valid pipeline DAG. Returns error if the pipeline is invalid
func Build(p *v1alpha1.Pipeline) (*DAG, error) {
	d := new()

	// Add all Tasks mentioned in the `PipelineSpec`
	for _, pt := range p.Spec.Tasks {
		if _, err := d.addPipelineTask(pt); err != nil {
			return nil, errors.NewDuplicatePipelineTask(p, pt.Name)
		}
	}
	// Process all from constraints to add task dependency
	for _, pt := range p.Spec.Tasks {
		if pt.Resources != nil {
			for _, rd := range pt.Resources.Inputs {
				for _, constraint := range rd.From {
					// We need to add dependency from constraint to node n
					prev, ok := d.Nodes[constraint]
					if !ok {
						return nil, errors.NewPipelineTaskNotFound(p, constraint)
					}
					next, _ := d.Nodes[pt.Name]
					if err := d.addPrevPipelineTask(prev, next); err != nil {
						return nil, errors.NewInvalidPipeline(p, err.Error())
					}
				}
			}
		}
	}
	return d, nil
}
