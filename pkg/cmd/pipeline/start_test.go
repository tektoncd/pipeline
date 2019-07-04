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

package pipeline

import (
	"fmt"
	"reflect"
	"testing"

	tu "github.com/tektoncd/cli/pkg/test"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stest "k8s.io/client-go/testing"
)

func Test_start_has_pipeline_arg(t *testing.T) {
	c := Command(&tu.Params{})

	_, err := tu.ExecuteCommand(c, "start", "-n", "ns")

	if err == nil {
		t.Error("Expecting an error but it's empty")
	}
}

func Test_start_pipeline_not_found(t *testing.T) {
	ps := []*v1alpha1.Pipeline{
		tb.Pipeline("test-pipeline", "foo",
			tb.PipelineSpec(
				tb.PipelineDeclaredResource("git-repo", "git"),
				tb.PipelineParam("pipeline-param", tb.PipelineParamDefault("somethingdifferent")),
				tb.PipelineParam("rev-param", tb.PipelineParamDefault("revision")),
				tb.PipelineTask("unit-test-1", "unit-test-task",
					tb.PipelineTaskInputResource("workspace", "git-repo"),
					tb.PipelineTaskOutputResource("image-to-use", "best-image"),
					tb.PipelineTaskOutputResource("workspace", "git-repo"),
				),
			),
		),
	}

	cs, _ := test.SeedTestData(t, test.Data{Pipelines: ps})
	p := &tu.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pipeline := Command(p)
	got, _ := tu.ExecuteCommand(pipeline, "start", "test-pipeline-2", "-n", "ns")
	expected := "Error: " + errInvalidPipeline.Error() + "\n"
	tu.AssertOutput(t, expected, got)
}

func Test_start_trigger(t *testing.T) {

	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "foo",
			tb.PipelineSpec(
				tb.PipelineDeclaredResource("git-repo", "git"),
				tb.PipelineParam("pipeline-param", tb.PipelineParamDefault("somethingdifferent")),
				tb.PipelineParam("rev-param", tb.PipelineParamDefault("revision")),
				tb.PipelineTask("unit-test-1", "unit-test-task",
					tb.PipelineTaskInputResource("workspace", "git-repo"),
					tb.PipelineTaskOutputResource("image-to-use", "best-image"),
					tb.PipelineTaskOutputResource("workspace", "git-repo"),
				),
			), // spec
		), // pipeline
	}

	cs, _ := test.SeedTestData(t, test.Data{Pipelines: ps})
	p := &tu.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pipeline := Command(p)
	got, _ := tu.ExecuteCommand(pipeline, "start", pipelineName,
		"-r source=scaffold-git",
		"-p key1=value1",
		"-s svc1",
		"-n", "ns")

	expected := "Pipelinerun started: \n"
	tu.AssertOutput(t, expected, got)

	pr, err := cs.Pipeline.TektonV1alpha1().PipelineRuns("ns").List(v1.ListOptions{})
	if err != nil {
		t.Errorf("Error listing pipelineruns %s", err.Error())
	}

	if pr.Items[0].ObjectMeta.GenerateName != (pipelineName + "-run-") {
		t.Errorf("Error pipelinerun generated is different %+v", pr)
	}
}

func Test_start_trigger_client_error(t *testing.T) {

	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "foo",
			tb.PipelineSpec(
				tb.PipelineDeclaredResource("git-repo", "git"),
				tb.PipelineParam("pipeline-param", tb.PipelineParamDefault("somethingdifferent")),
				tb.PipelineParam("rev-param", tb.PipelineParamDefault("revision")),
				tb.PipelineTask("unit-test-1", "unit-test-task",
					tb.PipelineTaskInputResource("workspace", "git-repo"),
					tb.PipelineTaskOutputResource("image-to-use", "best-image"),
					tb.PipelineTaskOutputResource("workspace", "git-repo"),
				),
			),
		),
	}

	cs, _ := test.SeedTestData(t, test.Data{Pipelines: ps})

	cs.Pipeline.PrependReactor("create", "*", func(_ k8stest.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("mock error")
	})
	p := &tu.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pipeline := Command(p)
	got, _ := tu.ExecuteCommand(pipeline, "start", pipelineName,
		"-r source=scaffold-git",
		"-p key1=value1",
		"-s svc1",
		"-n", "ns")

	expected := "Error: mock error\n"
	tu.AssertOutput(t, expected, got)
}

func Test_start_trigger_resource_error(t *testing.T) {

	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "foo",
			tb.PipelineSpec(
				tb.PipelineDeclaredResource("git-repo", "git"),
				tb.PipelineParam("pipeline-param", tb.PipelineParamDefault("somethingdifferent")),
				tb.PipelineParam("rev-param", tb.PipelineParamDefault("revision")),
				tb.PipelineTask("unit-test-1", "unit-test-task",
					tb.PipelineTaskInputResource("workspace", "git-repo"),
					tb.PipelineTaskOutputResource("image-to-use", "best-image"),
					tb.PipelineTaskOutputResource("workspace", "git-repo"),
				),
			),
		),
	}

	cs, _ := test.SeedTestData(t, test.Data{Pipelines: ps})

	p := &tu.Params{Tekton: cs.Pipeline, Kube: cs.Kube}
	pipeline := Command(p)
	got, _ := tu.ExecuteCommand(pipeline, "start", pipelineName,
		"-r scaffold-git",
		"-p key1=value1",
		"-s svc1",
		"-n", "ns")
	expected := "Error: invalid resource parameter:  scaffold-git\n Please pass resource as -p ResourceName=ResourceRef\n"

	tu.AssertOutput(t, expected, got)
}

func Test_start_trigger_param_error(t *testing.T) {

	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "foo",
			tb.PipelineSpec(
				tb.PipelineDeclaredResource("git-repo", "git"),
				tb.PipelineParam("pipeline-param", tb.PipelineParamDefault("somethingdifferent")),
				tb.PipelineParam("rev-param", tb.PipelineParamDefault("revision")),
				tb.PipelineTask("unit-test-1", "unit-test-task",
					tb.PipelineTaskInputResource("workspace", "git-repo"),
					tb.PipelineTaskOutputResource("image-to-use", "best-image"),
					tb.PipelineTaskOutputResource("workspace", "git-repo"),
				),
			),
		),
	}

	cs, _ := test.SeedTestData(t, test.Data{Pipelines: ps})

	p := &tu.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pipeline := Command(p)
	got, _ := tu.ExecuteCommand(pipeline, "start", pipelineName,
		"-r source=scaffold-git",
		"-p value1",
		"-s svc1",
		"-n", "ns")
	expected := "Error: invalid param parameter:  value1\n Please pass resource as -r ParamName=ParamValue\n"

	tu.AssertOutput(t, expected, got)
}

func Test_parseRes(t *testing.T) {
	type args struct {
		res []string
	}
	tests := []struct {
		name    string
		args    args
		want    []v1alpha1.PipelineResourceBinding
		wantErr bool
	}{{
		name: "Test_parseRes No Err",
		args: args{
			res: []string{"source=git", "image=docker2"},
		},
		want: []v1alpha1.PipelineResourceBinding{{
			Name: "source",
			ResourceRef: v1alpha1.PipelineResourceRef{
				Name: "git",
			},
		}, {
			Name: "image",
			ResourceRef: v1alpha1.PipelineResourceRef{
				Name: "docker2",
			},
		}},
		wantErr: false,
	}, {
		name: "Test_parseRes Err",
		args: args{
			res: []string{"value1", "value2"},
		},
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRes(tt.args.res)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseRes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseParam(t *testing.T) {
	type args struct {
		p []string
	}
	tests := []struct {
		name    string
		args    args
		want    []v1alpha1.Param
		wantErr bool
	}{{
		name: "Test_parseParam No Err",
		args: args{
			p: []string{"key1=value1", "key2=value2"},
		},
		want: []v1alpha1.Param{
			{Name: "key1", Value: "value1"},
			{Name: "key2", Value: "value2"},
		},
		wantErr: false,
	}, {
		name: "Test_parseParam Err",
		args: args{
			p: []string{"value1", "value2"},
		},
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseParam(tt.args.p)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseParam() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseParam() = %v, want %v", got, tt.want)
			}
		})
	}
}
