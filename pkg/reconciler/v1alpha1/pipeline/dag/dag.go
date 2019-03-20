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

package dag

import (
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/list"
)

// GetSchedulable returns a map of PipelineTask that can be scheduled (keyed
// by the name of the PipelineTask) given a list of successfully finished doneTasks.
// It returns tasks which have all dependecies marked as done, and thus can be scheduled. If the
// specified doneTasks are invalid (i.e. if it is indicated that a Task is
// done, but the previous Tasks are not done), an error is returned.
func GetSchedulable(g *v1alpha1.DAG, doneTasks ...string) (map[string]v1alpha1.PipelineTask, error) {
	roots := getRoots(g)
	tm := toMap(doneTasks...)
	d := map[string]v1alpha1.PipelineTask{}

	visited := map[string]struct{}{}
	for _, root := range roots {
		schedulable := findSchedulable(root, visited, tm)
		for _, task := range schedulable {
			d[task.Name] = task
		}
	}

	visitedNames := make([]string, len(visited))
	for v := range visited {
		visitedNames = append(visitedNames, v)
	}

	notVisited := list.DiffLeft(doneTasks, visitedNames)
	if len(notVisited) > 0 {
		return map[string]v1alpha1.PipelineTask{}, fmt.Errorf("invalid list of done tasks; some tasks were indicated completed without ancestors being done: %v", notVisited)
	}

	return d, nil
}

func getRoots(g *v1alpha1.DAG) []*v1alpha1.Node {
	n := []*v1alpha1.Node{}
	for _, node := range g.Nodes {
		if len(node.Prev) == 0 {
			n = append(n, node)
		}
	}
	return n
}

func findSchedulable(n *v1alpha1.Node, visited map[string]struct{}, doneTasks map[string]struct{}) []v1alpha1.PipelineTask {
	if _, ok := visited[n.Task.Name]; ok {
		return []v1alpha1.PipelineTask{}
	}
	visited[n.Task.Name] = struct{}{}
	if _, ok := doneTasks[n.Task.Name]; ok {
		schedulable := []v1alpha1.PipelineTask{}
		// This one is done! Take note of it and look at the next candidate
		for _, next := range n.Next {
			if _, ok := visited[next.Task.Name]; !ok {
				schedulable = append(schedulable, findSchedulable(next, visited, doneTasks)...)
			}
		}
		return schedulable
	}
	// This one isn't done! Return it if it's schedulable
	if isSchedulable(doneTasks, n.Prev) {
		return []v1alpha1.PipelineTask{n.Task}
	}
	// This one isn't done, but it also isn't ready to schedule
	return []v1alpha1.PipelineTask{}
}

func isSchedulable(doneTasks map[string]struct{}, prevs []*v1alpha1.Node) bool {
	if len(prevs) == 0 {
		return true
	}
	collected := []string{}
	for _, n := range prevs {
		if _, ok := doneTasks[n.Task.Name]; ok {
			collected = append(collected, n.Task.Name)
		}
	}
	return len(collected) == len(prevs)
}

func toMap(t ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(t))
	for _, s := range t {
		m[s] = struct{}{}
	}
	return m
}
