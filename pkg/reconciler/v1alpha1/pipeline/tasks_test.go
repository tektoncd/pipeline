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
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	fakepipelineclientset "github.com/knative/build-pipeline/pkg/client/clientset/versioned/fake"
	informers "github.com/knative/build-pipeline/pkg/client/informers/externalversions"
	informersv1alpha1 "github.com/knative/build-pipeline/pkg/client/informers/externalversions/pipeline/v1alpha1"
)

func getFakeInformer() informersv1alpha1.TaskInformer {
	pipelineClient := fakepipelineclientset.NewSimpleClientset()
	sharedInfomer := informers.NewSharedInformerFactory(pipelineClient, 0)
	return sharedInfomer.Pipeline().V1alpha1().Tasks()
}

var p = &v1alpha1.Pipeline{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipeline",
	},
	Spec: v1alpha1.PipelineSpec{
		Tasks: []v1alpha1.PipelineTask{{
			Name:    "mytask1",
			TaskRef: v1alpha1.TaskRef{Name: "task"},
		}, {
			Name:    "mytask2",
			TaskRef: v1alpha1.TaskRef{Name: "task"},
		}},
	},
}

func TestGetTasks(t *testing.T) {
	task := &v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace",
			Name:      "task",
		},
		Spec: v1alpha1.TaskSpec{},
	}
	i := getFakeInformer()
	i.Informer().GetIndexer().Add(task)

	tasks, err := GetTasks(i.Lister(), p)
	if err != nil {
		t.Fatalf("Error getting tasks for fake pipeline %s: %s", p.ObjectMeta.Name, err)
	}
	expectedTasks := map[string]*v1alpha1.Task{
		"mytask1": task,
		"mytask2": task,
	}
	if !reflect.DeepEqual(tasks, expectedTasks) {
		t.Fatalf("Expected to get map of tasks %v but got %v instead", expectedTasks, tasks)
	}
}

func TestGetTasksDoesntExist(t *testing.T) {
	i := getFakeInformer()
	_, err := GetTasks(i.Lister(), p)
	if err == nil {
		t.Fatalf("Expected error getting non-existent Tasks for Pipeline %s but got none", p.ObjectMeta.Name)
	}
}
