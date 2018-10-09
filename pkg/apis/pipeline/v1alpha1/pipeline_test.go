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
package v1alpha1

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var p = &Pipeline{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipeline",
	},
	Spec: PipelineSpec{
		Tasks: []PipelineTask{{
			Name:    "mytask1",
			TaskRef: TaskRef{Name: "task"},
		}, {
			Name:    "mytask2",
			TaskRef: TaskRef{Name: "task"},
		}},
	},
}

func TestGetTasks(t *testing.T) {
	task := &Task{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace",
			Name:      "task",
		},
		Spec: TaskSpec{},
	}
	tasks, err := p.GetTasks(func(namespace, name string) (*Task, error) {
		return task, nil
	})
	if err != nil {
		t.Fatalf("Error getting tasks for fake pipeline %s: %s", p.ObjectMeta.Name, err)
	}
	expectedTasks := map[string]*Task{
		"mytask1": task,
		"mytask2": task,
	}
	if d := cmp.Diff(tasks, expectedTasks); d != "" {
		t.Fatalf("Expected to get map of tasks %v, but actual differed: %s", expectedTasks, d)
	}
}

func TestGetTasksDoesntExist(t *testing.T) {
	_, err := p.GetTasks(func(namespace, name string) (*Task, error) {
		return nil, fmt.Errorf("failed to get tasks for pipeline %s", p.Name)
	})
	if err == nil {
		t.Fatalf("Expected error getting non-existent Tasks for Pipeline %s but got none", p.ObjectMeta.Name)
	}
}
