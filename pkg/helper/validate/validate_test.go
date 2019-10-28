// Copyright © 2019 The Tekton Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validate

import (
	"testing"

	"github.com/tektoncd/cli/pkg/test"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	pipelinetest "github.com/tektoncd/pipeline/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNamespaceExists_Invalid_Namespace(t *testing.T) {
	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "default",
			},
		},
	}
	cs, _ := test.SeedTestData(t, pipelinetest.Data{Namespaces: ns})
	p := &test.Params{Kube: cs.Kube}
	p.SetNamespace("foo")

	err := NamespaceExists(p)
	test.AssertOutput(t, "namespaces \"foo\" not found", err.Error())
}

func TestNamespaceExists_Valid_Namespace(t *testing.T) {
	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "default",
			},
		},
	}
	cs, _ := test.SeedTestData(t, pipelinetest.Data{Namespaces: ns})
	p := &test.Params{Kube: cs.Kube}

	err := NamespaceExists(p)
	test.AssertOutput(t, nil, err)
}

func TestTaskRefExists_Present(t *testing.T) {
	spec := v1alpha1.TaskRunSpec{
		TaskRef: &v1alpha1.TaskRef{
			Name: "Task",
		},
	}

	output := TaskRefExists(spec)
	test.AssertOutput(t, "Task", output)
}

func TestTaskRefExists_Not_Present(t *testing.T) {
	spec := v1alpha1.TaskRunSpec{
		TaskRef: nil,
	}

	output := TaskRefExists(spec)
	test.AssertOutput(t, "", output)
}
