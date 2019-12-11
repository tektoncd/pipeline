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
)

type Task interface {
	HashKey() string
	Deps() []string
}

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
	//Nodes represent map of PipelineTask name to Node in Pipeline Graph
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
func Build(tasks Tasks) (*Graph, error) {
	d := newGraph()

	deps := map[string][]string{}
	// Add all Tasks mentioned in the `PipelineSpec`
	for _, pt := range tasks.Items() {
		if _, err := d.addPipelineTask(pt); err != nil {
			return nil, fmt.Errorf("task %s is already present in Graph, can't add it again: %w", pt.HashKey(), err)
		}
		deps[pt.HashKey()] = pt.Deps()
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

// GetSchedulable returns a map of PipelineTask that can be scheduled (keyed
// by the name of the PipelineTask) given a list of successfully finished doneTasks.
// It returns tasks which have all dependecies marked as done, and thus can be scheduled. If the
// specified doneTasks are invalid (i.e. if it is indicated that a Task is
// done, but the previous Tasks are not done), an error is returned.
func GetSchedulable(g *Graph, doneTasks ...string) (map[string]struct{}, error) {
	roots := getRoots(g)
	tm := toMap(doneTasks...)
	d := map[string]struct{}{}

	visited := map[string]struct{}{}
	for _, root := range roots {
		schedulable := findSchedulable(root, visited, tm)
		for _, task := range schedulable {
			d[task.HashKey()] = struct{}{}
		}
	}

	var visitedNames []string
	for v := range visited {
		visitedNames = append(visitedNames, v)
	}

	notVisited := list.DiffLeft(doneTasks, visitedNames)
	if len(notVisited) > 0 {
		return map[string]struct{}{}, fmt.Errorf("invalid list of done tasks; some tasks were indicated completed without ancestors being done: %v", notVisited)
	}

	return d, nil
}

func linkPipelineTasks(prev *Node, next *Node) error {
	// Check for self cycle
	if prev.Task.HashKey() == next.Task.HashKey() {
		return fmt.Errorf("cycle detected; task %q depends on itself", next.Task.HashKey())
	}
	// Check if we are adding cycles.
	visited := map[string]bool{prev.Task.HashKey(): true, next.Task.HashKey(): true}
	path := []string{next.Task.HashKey(), prev.Task.HashKey()}
	if err := visit(next.Task.HashKey(), prev.Prev, path, visited); err != nil {
		return fmt.Errorf("cycle detected: %w", err)
	}
	next.Prev = append(next.Prev, prev)
	prev.Next = append(prev.Next, next)
	return nil
}

func visit(currentName string, nodes []*Node, path []string, visited map[string]bool) error {
	for _, n := range nodes {
		path = append(path, n.Task.HashKey())
		if _, ok := visited[n.Task.HashKey()]; ok {
			return errors.New(getVisitedPath(path))
		}
		visited[currentName+"."+n.Task.HashKey()] = true
		if err := visit(n.Task.HashKey(), n.Prev, path, visited); err != nil {
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

func findSchedulable(n *Node, visited map[string]struct{}, doneTasks map[string]struct{}) []Task {
	if _, ok := visited[n.Task.HashKey()]; ok {
		return []Task{}
	}
	visited[n.Task.HashKey()] = struct{}{}
	if _, ok := doneTasks[n.Task.HashKey()]; ok {
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

func isSchedulable(doneTasks map[string]struct{}, prevs []*Node) bool {
	if len(prevs) == 0 {
		return true
	}
	collected := []string{}
	for _, n := range prevs {
		if _, ok := doneTasks[n.Task.HashKey()]; ok {
			collected = append(collected, n.Task.HashKey())
		}
	}
	return len(collected) == len(prevs)
}

func toMap(t ...string) map[string]struct{} {
	m := map[string]struct{}{}
	for _, s := range t {
		m[s] = struct{}{}
	}
	return m
}
