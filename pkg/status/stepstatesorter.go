/*
 Copyright 2019 Knative Authors LLC
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

package status

import (
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sort"
)


// StepStateSorter implements a sorting mechanism to align the order of the steps in TaskRun
// with the spec steps in Task.
type StepStateSorter struct {
	taskRunSteps []v1alpha1.StepState
	mapForSort map[string]int
}

func (trt *StepStateSorter) Init(taskRunSteps []v1alpha1.StepState, taskSpecSteps []corev1.Container) {
	trt.taskRunSteps = taskRunSteps
	trt.mapForSort = trt.constructTaskStepsSorter(taskSpecSteps)
}

// constructTaskStepsSorter constructs a map matching the names of
// the steps to their indices for a task.
func (trt *StepStateSorter) constructTaskStepsSorter(taskSpecSteps []corev1.Container) map[string]int {
	sorter := make(map[string]int)
	for index, step := range taskSpecSteps {
		sorter[step.Name] = index
	}
	return sorter
}

// changeIndex sorts the steps of the task run, based on the
// order of the steps in the task. Instead of changing the element with the one next to it,
// we directly swap it with the desired index.
func (trt *StepStateSorter) changeIndex(index int) {
	// Check if the current index is equal to the desired index. If they are equal, do not swap; if they
	// are not equal, swap index j with the desired index.
	desiredIndex, exist := trt.mapForSort[trt.taskRunSteps[index].Name]
	if exist && index != desiredIndex {
		trt.taskRunSteps[desiredIndex], trt.taskRunSteps[index] = trt.taskRunSteps[index], trt.taskRunSteps[desiredIndex]
	}
}

func (trt *StepStateSorter) Len() int {
	return len(trt.taskRunSteps)
}

func (trt *StepStateSorter) Swap(i, j int) {
	trt.changeIndex(j)
	// The index j is unable to reach the last index.
	// When i reaches the end of the array, we need to check whether the last one needs a swap.
	if (i == trt.Len() - 1 ) {
		trt.changeIndex(i)
	}
}

func (trt *StepStateSorter) Less(i, j int) bool {
	// Since the logic is complicated, we move it into the Swap function to decide whether
	// and how to change the index. We set it to true here in order to iterate all the
	// elements of the array in the Swap function.
	return true
}

func SortTaskRunStepOrder(taskRunSteps []v1alpha1.StepState, taskSpecSteps []corev1.Container) {
	trt := new(StepStateSorter)
	trt.Init(taskRunSteps, taskSpecSteps)
	sort.Sort(trt)
}
