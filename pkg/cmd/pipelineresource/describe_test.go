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

package pipelineresource

import (
	"strings"
	"testing"

	"github.com/tektoncd/cli/pkg/test"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	pipelinetest "github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
)

func TestPipelineResourceDescribe_Empty(t *testing.T) {
	cs, _ := test.SeedTestData(t, pipelinetest.Data{})
	p := &test.Params{Tekton: cs.Pipeline}

	res := Command(p)
	_, err := test.ExecuteCommand(res, "desc", "bar")
	if err == nil {
		t.Errorf("Error expected here")
	}
	expected := "failed to find pipelineresource \"bar\""
	test.AssertOutput(t, expected, err.Error())
}

func TestPipelineResourceDescribe_WithParams(t *testing.T) {
	pres := []*v1alpha1.PipelineResource{
		tb.PipelineResource("test-1", "test-ns-1",
			tb.PipelineResourceSpec("image",
				tb.PipelineResourceSpecParam("URL", "quay.io/tekton/controller"),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{PipelineResources: pres})
	p := &test.Params{Tekton: cs.Pipeline}
	pipelineresource := Command(p)
	out, _ := test.ExecuteCommand(pipelineresource, "desc", "test-1", "-n", "test-ns-1")
	expected := []string{
		"Name:                    test-1",
		"Namespace:               test-ns-1",
		"PipelineResource Type:   image",
		"",
		"Params",
		"NAME   VALUE",
		"URL    quay.io/tekton/controller",
		"",
		"Secret Params",
		"No secret params",
		"",
	}
	test.AssertOutput(t, strings.Join(expected, "\n"), out)
}

func TestPipelineResourceDescribe_WithSecretParams(t *testing.T) {
	pres := []*v1alpha1.PipelineResource{
		tb.PipelineResource("test-1", "test-ns-1",
			tb.PipelineResourceSpec("image",
				tb.PipelineResourceSpecParam("URL", "quay.io/tekton/controller"),
				tb.PipelineResourceSpecParam("TAG", "latest"),
				tb.PipelineResourceSpecSecretParam("githubToken", "github-secrets", "token"),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{PipelineResources: pres})
	p := &test.Params{Tekton: cs.Pipeline}
	pipelineresource := Command(p)
	out, _ := test.ExecuteCommand(pipelineresource, "desc", "test-1", "-n", "test-ns-1")
	expected := []string{
		"Name:                    test-1",
		"Namespace:               test-ns-1",
		"PipelineResource Type:   image",
		"",
		"Params",
		"NAME   VALUE",
		"URL    quay.io/tekton/controller",
		"TAG    latest",
		"",
		"Secret Params",
		"FIELDNAME     SECRETNAME",
		"githubToken   github-secrets",
		"",
	}
	test.AssertOutput(t, strings.Join(expected, "\n"), out)
}
