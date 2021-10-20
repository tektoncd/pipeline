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

package dag

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tektoncd/pipeline/pkg/list"
	"k8s.io/apimachinery/pkg/util/sets"
)

// Task is an interface for all types that could be in a DAG
type Task interface {
	HashKey() string
	Deps() []string
}

// Tasks is an interface for lists of types that could be in a DAG
type Tasks interface {
	Items() []Task
}

// Node represents a Task in a pipeline.
type Node struct {
	// Task represent the PipelineTask in Pipeline
	Task Task
	// Prev represent all the Previous task Nodes for the current Task
	Prev []*Node
	// Next represent all the Next task Nodes for the current Task
	Next []*Node
}

// Graph represents the Pipeline Graph
type Graph struct {
	// Nodes represent map of PipelineTask name to Node in Pipeline Graph
	Nodes map[string]*Node
}

// Returns an empty Pipeline Graph
func newGraph() *Graph {
	return &Graph{Nodes: map[string]*Node{}}
}

func (g *Graph) addPipelineTask(t Task) (*Node, error) {
	if _, ok := g.Nodes[t.HashKey()]; ok {
		return nil, errors.New("duplicate pipeline task")
	}
	newNode := &Node{
		Task: t,
	}
	g.Nodes[t.HashKey()] = newNode
	return newNode, nil
}

// Build returns a valid pipeline Graph. Returns error if the pipeline is invalid
func Build(tasks Tasks, deps map[string][]string) (*Graph, error) {
	d := newGraph()

	// Add all Tasks mentioned in the `PipelineSpec`
	for _, pt := range tasks.Items() {
		if _, err := d.addPipelineTask(pt); err != nil {
			return nil, fmt.Errorf("task %s is already present in Graph, can't add it again: %w", pt.HashKey(), err)
		}
	}

	// Process all from and runAfter constraints to add task dependency
	for pt, taskDeps := range deps {
		for _, previousTask := range taskDeps {
			if err := addLink(pt, previousTask, d.Nodes); err != nil {
				return nil, fmt.Errorf("couldn't add link between %s and %s: %w", pt, previousTask, err)
			}
		}
	}
	return d, nil
}

// GetSchedulable returns a set of PipelineTask names that can be scheduled,
// given a list of successfully finished doneTasks. It returns tasks which have
// all dependencies marked as done, and thus can be scheduled. If the specified
// doneTasks are invalid (i.e. if it is indicated that a Task is done, but the
// previous Tasks are not done), an error is returned.
func GetSchedulable(g *Graph, doneTasks ...string) (sets.String, error) {
	roots := getRoots(g)
	tm := sets.NewString(doneTasks...)
	d := sets.NewString()

	visited := sets.NewString()
	for _, root := range roots {
		schedulable := findSchedulable(root, visited, tm)
		for _, task := range schedulable {
			d.Insert(task.HashKey())
		}
	}

	var visitedNames []string
	for v := range visited {
		visitedNames = append(visitedNames, v)
	}

	notVisited := list.DiffLeft(doneTasks, visitedNames)
	if len(notVisited) > 0 {
		return nil, fmt.Errorf("invalid list of done tasks; some tasks were indicated completed without ancestors being done: %v", notVisited)
	}

	return d, nil
}

func linkPipelineTasks(prev *Node, next *Node) error {
	// Check for self cycle
	if prev.Task.HashKey() == next.Task.HashKey() {
		return fmt.Errorf("cycle detected; task %q depends on itself", next.Task.HashKey())
	}
	// Check if we are adding cycles.
	path := []string{next.Task.HashKey(), prev.Task.HashKey()}
	if err := lookForNode(prev.Prev, path, next.Task.HashKey()); err != nil {
		return fmt.Errorf("cycle detected: %w", err)
	}
	next.Prev = append(next.Prev, prev)
	prev.Next = append(prev.Next, next)
	return nil
}

func lookForNode(nodes []*Node, path []string, next string) error {
	for _, n := range nodes {
		path = append(path, n.Task.HashKey())
		if n.Task.HashKey() == next {
			return errors.New(getVisitedPath(path))
		}
		if err := lookForNode(n.Prev, path, next); err != nil {
			return err
		}
	}
	return nil
}

func getVisitedPath(path []string) string {
	// Reverse the path since we traversed the Graph using prev pointers.
	for i := len(path)/2 - 1; i >= 0; i-- {
		opp := len(path) - 1 - i
		path[i], path[opp] = path[opp], path[i]
	}
	return strings.Join(path, " -> ")
}

func addLink(pt string, previousTask string, nodes map[string]*Node) error {
	prev, ok := nodes[previousTask]
	if !ok {
		return fmt.Errorf("task %s depends on %s but %s wasn't present in Pipeline", pt, previousTask, previousTask)
	}
	next := nodes[pt]
	if err := linkPipelineTasks(prev, next); err != nil {
		return fmt.Errorf("couldn't create link from %s to %s: %w", prev.Task.HashKey(), next.Task.HashKey(), err)
	}
	return nil
}

func getRoots(g *Graph) []*Node {
	n := []*Node{}
	for _, node := range g.Nodes {
		if len(node.Prev) == 0 {
			n = append(n, node)
		}
	}
	return n
}

func findSchedulable(n *Node, visited sets.String, doneTasks sets.String) []Task {
	if visited.Has(n.Task.HashKey()) {
		return []Task{}
	}
	visited.Insert(n.Task.HashKey())
	if doneTasks.Has(n.Task.HashKey()) {
		schedulable := []Task{}
		// This one is done! Take note of it and look at the next candidate
		for _, next := range n.Next {
			if _, ok := visited[next.Task.HashKey()]; !ok {
				schedulable = append(schedulable, findSchedulable(next, visited, doneTasks)...)
			}
		}
		return schedulable
	}
	// This one isn't done! Return it if it's schedulable
	if isSchedulable(doneTasks, n.Prev) {
		// FIXME(vdemeester)
		return []Task{n.Task}
	}
	// This one isn't done, but it also isn't ready to schedule
	return []Task{}
}

func isSchedulable(doneTasks sets.String, prevs []*Node) bool {
	if len(prevs) == 0 {
		return true
	}
	collected := []string{}
	for _, n := range prevs {
		if doneTasks.Has(n.Task.HashKey()) {
			collected = append(collected, n.Task.HashKey())
		}
	}
	return len(collected) == len(prevs)
}
