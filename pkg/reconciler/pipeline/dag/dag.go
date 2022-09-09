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
	"sort"
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
	// Key represent a unique name of the node in a graph
	Key string
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
		Key: t.HashKey(),
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

	// Ensure no cycles in the graph
	if err := findCyclesInDependencies(deps); err != nil {
		return nil, fmt.Errorf("cycle detected; %w", err)
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

// GetCandidateTasks returns a set of names of PipelineTasks whose ancestors are all completed,
// given a list of finished doneTasks. If the specified
// doneTasks are invalid (i.e. if it is indicated that a Task is done, but the
// previous Tasks are not done), an error is returned.
func GetCandidateTasks(g *Graph, doneTasks ...string) (sets.String, error) {
	roots := getRoots(g)
	tm := sets.NewString(doneTasks...)
	d := sets.NewString()

	visited := sets.NewString()
	for _, root := range roots {
		schedulable := findSchedulable(root, visited, tm)
		for _, taskName := range schedulable {
			d.Insert(taskName)
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

func linkPipelineTasks(prev *Node, next *Node) {
	next.Prev = append(next.Prev, prev)
	prev.Next = append(prev.Next, next)
}

// use Kahn's algorithm to find cycles in dependencies
func findCyclesInDependencies(deps map[string][]string) error {
	independentTasks := sets.NewString()
	dag := make(map[string]sets.String, len(deps))
	childMap := make(map[string]sets.String, len(deps))
	for task, taskDeps := range deps {
		if len(taskDeps) == 0 {
			continue
		}
		dag[task] = sets.NewString(taskDeps...)
		for _, dep := range taskDeps {
			if len(deps[dep]) == 0 {
				independentTasks.Insert(dep)
			}
			if children, ok := childMap[dep]; ok {
				children.Insert(task)
			} else {
				childMap[dep] = sets.NewString(task)
			}
		}
	}

	for {
		parent, ok := independentTasks.PopAny()
		if !ok {
			break
		}
		children := childMap[parent]
		for {
			child, ok := children.PopAny()
			if !ok {
				break
			}
			dag[child].Delete(parent)
			if dag[child].Len() == 0 {
				independentTasks.Insert(child)
				delete(dag, child)
			}
		}
	}

	return getInterdependencyError(dag)
}

func getInterdependencyError(dag map[string]sets.String) error {
	if len(dag) == 0 {
		return nil
	}
	firstChild := ""
	for task := range dag {
		if firstChild == "" || firstChild > task {
			firstChild = task
		}
	}
	deps := dag[firstChild].List()
	depNames := make([]string, 0, len(deps))
	sort.Strings(deps)
	for _, dep := range deps {
		depNames = append(depNames, fmt.Sprintf("%q", dep))
	}
	return fmt.Errorf("task %q depends on %s", firstChild, strings.Join(depNames, ", "))
}

func addLink(pt string, previousTask string, nodes map[string]*Node) error {
	prev, ok := nodes[previousTask]
	if !ok {
		return fmt.Errorf("task %s depends on %s but %s wasn't present in Pipeline", pt, previousTask, previousTask)
	}
	next := nodes[pt]
	linkPipelineTasks(prev, next)
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

func findSchedulable(n *Node, visited sets.String, doneTasks sets.String) []string {
	if visited.Has(n.Key) {
		return []string{}
	}
	visited.Insert(n.Key)
	if doneTasks.Has(n.Key) {
		schedulable := []string{}
		// This one is done! Take note of it and look at the next candidate
		for _, next := range n.Next {
			if _, ok := visited[next.Key]; !ok {
				schedulable = append(schedulable, findSchedulable(next, visited, doneTasks)...)
			}
		}
		return schedulable
	}
	// This one isn't done! Return it if it's schedulable
	if isSchedulable(doneTasks, n.Prev) {
		// FIXME(vdemeester)
		return []string{n.Key}
	}
	// This one isn't done, but it also isn't ready to schedule
	return []string{}
}

func isSchedulable(doneTasks sets.String, prevs []*Node) bool {
	if len(prevs) == 0 {
		return true
	}
	collected := []string{}
	for _, n := range prevs {
		if doneTasks.Has(n.Key) {
			collected = append(collected, n.Key)
		}
	}
	return len(collected) == len(prevs)
}
