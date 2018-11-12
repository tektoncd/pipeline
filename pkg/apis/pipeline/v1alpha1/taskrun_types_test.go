/*
Copyright 2018 The Knative Authors.

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
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTaskRun_GetBuildRef(t *testing.T) {
	tr := TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "taskrunname",
			Namespace: "testns",
		},
	}
	expectedBuildRef := corev1.ObjectReference{
		APIVersion: "build.knative.dev/v1alpha1",
		Kind:       "Build",
		Namespace:  "testns",
		Name:       "taskrunname",
	}
	if d := cmp.Diff(tr.GetBuildRef(), expectedBuildRef); d != "" {
		t.Fatalf("taskrun build ref mismatch: %s", d)
	}
}

func TestTasRun_Valid_GetPipelineRunPVCName(t *testing.T) {
	tr := TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "taskrunname",
			Namespace: "testns",
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "PipelineRun",
				Name: "testpr",
			}},
		},
	}
	expectedPVCName := "testpr-pvc"
	if tr.GetPipelineRunPVCName() != expectedPVCName {
		t.Fatalf("taskrun pipeline run mismatch: got %s ; expected %s", tr.GetPipelineRunPVCName(), expectedPVCName)
	}
}

func TestTasRun_InvalidOwner_GetPipelineRunPVCName(t *testing.T) {
	tr := TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "taskrunname",
			Namespace: "testns",
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "SomeOtherOwner",
				Name: "testpr",
			}},
		},
	}
	expectedPVCName := ""
	if tr.GetPipelineRunPVCName() != expectedPVCName {
		t.Fatalf("taskrun pipeline run pvc name mismatch: got %s ; expected %s", tr.GetPipelineRunPVCName(), expectedPVCName)
	}
}
