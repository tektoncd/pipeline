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
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/knative/pkg/apis"
	"github.com/tektoncd/cli/pkg/cli"
	tu "github.com/tektoncd/cli/pkg/test"
	cb "github.com/tektoncd/cli/pkg/test/builder"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	fakepipelineclientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/pipelinerun/resources"
	"github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	util_runtime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	k8stest "k8s.io/client-go/testing"
)

func newPipelineClient(objs ...runtime.Object) *fakepipelineclientset.Clientset {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)
	localSchemeBuilder := runtime.SchemeBuilder{
		v1alpha1.AddToScheme,
	}
	v1.AddToGroupVersion(scheme, schema.GroupVersion{Version: "v1"})
	util_runtime.Must(localSchemeBuilder.AddToScheme(scheme))

	o := k8stest.NewObjectTracker(scheme, codecs.UniversalDecoder())
	for _, obj := range objs {
		if err := o.Add(obj); err != nil {
			panic(err)
		}
	}

	c := &fakepipelineclientset.Clientset{}
	c.AddReactor("*", "*", k8stest.ObjectReaction(o))
	c.AddWatchReactor("*", func(action k8stest.Action) (handled bool, ret watch.Interface, err error) {
		gvr := action.GetResource()
		ns := action.GetNamespace()
		watch, err := o.Watch(gvr, ns)
		if err != nil {
			return false, nil, err
		}
		return true, watch, nil
	})

	c.PrependReactor("create", "pipelineruns", func(action k8stest.Action) (bool, runtime.Object, error) {
		create := action.(k8stest.CreateActionImpl)
		obj := create.GetObject().(*v1alpha1.PipelineRun)
		obj.Name = "random"
		rFunc := k8stest.ObjectReaction(o)
		_, o, err := rFunc(action)
		return true, o, err
	})

	return c
}

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

func Test_start_pipeline(t *testing.T) {

	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "ns",
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
		"-r=source=scaffold-git",
		"-p=key1=value1",
		"-s=svc1",
		"-serviceaccounts=task1=svc1",
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

func Test_start_pipeline_last(t *testing.T) {

	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "ns",
			tb.PipelineSpec(
				tb.PipelineDeclaredResource("git-repo", "git"),
				tb.PipelineDeclaredResource("build-image", "image"),
				tb.PipelineParam("pipeline-param-1", tb.PipelineParamDefault("somethingdifferent-1")),
				tb.PipelineParam("rev-param", tb.PipelineParamDefault("revision")),
				tb.PipelineTask("unit-test-1", "unit-test-task",
					tb.PipelineTaskInputResource("workspace", "git-repo"),
					tb.PipelineTaskOutputResource("image-to-use", "best-image"),
					tb.PipelineTaskOutputResource("workspace", "git-repo"),
				),
			), // spec
		), // pipeline
	}

	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun("test-pipeline-run-123", "ns",
			tb.PipelineRunLabel("tekton.dev/pipeline", pipelineName),
			tb.PipelineRunSpec(pipelineName,
				tb.PipelineRunServiceAccount("test-sa"),
				tb.PipelineRunResourceBinding("git-repo", tb.PipelineResourceBindingRef("some-repo")),
				tb.PipelineRunResourceBinding("build-image", tb.PipelineResourceBindingRef("some-image")),
				tb.PipelineRunParam("pipeline-param-1", "somethingmorefun"),
				tb.PipelineRunParam("rev-param", "revision1"),
			),
		),
	}

	objs := []runtime.Object{ps[0], prs[0]}
	pClient := newPipelineClient(objs...)

	cs := test.Clients{
		Pipeline: pClient,
		Kube:     fakekubeclientset.NewSimpleClientset(),
	}

	p := &tu.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pipeline := Command(p)
	got, _ := tu.ExecuteCommand(pipeline, "start", pipelineName,
		"--last",
		"-nns")

	expected := "Pipelinerun started: random\n"
	tu.AssertOutput(t, expected, got)

	pr, err := cs.Pipeline.TektonV1alpha1().PipelineRuns(p.Namespace()).Get("random", v1.GetOptions{})

	if err != nil {
		t.Errorf("Error getting pipelineruns %s", err.Error())
	}

	for _, v := range pr.Spec.Resources {
		if v.Name == "git-repo" {
			tu.AssertOutput(t, "some-repo", v.ResourceRef.Name)
		}
	}

	for _, v := range pr.Spec.Params {
		if v.Name == "rev-param" {
			tu.AssertOutput(t, "revision1", v.Value)
		}
	}
	tu.AssertOutput(t, "test-sa", pr.Spec.ServiceAccount)
}

func Test_start_pipeline_last_merge(t *testing.T) {

	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "ns",
			tb.PipelineSpec(
				tb.PipelineDeclaredResource("git-repo", "git"),
				tb.PipelineDeclaredResource("build-image", "image"),
				tb.PipelineParam("pipeline-param-1", tb.PipelineParamDefault("somethingdifferent-1")),
				tb.PipelineParam("rev-param", tb.PipelineParamDefault("revision")),
				tb.PipelineTask("unit-test-1", "unit-test-task",
					tb.PipelineTaskInputResource("workspace", "git-repo"),
					tb.PipelineTaskOutputResource("image-to-use", "best-image"),
					tb.PipelineTaskOutputResource("workspace", "git-repo"),
				),
			), // spec
		), // pipeline
	}

	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun("test-pipeline-run-123", "ns",
			tb.PipelineRunLabel("tekton.dev/pipeline", pipelineName),
			tb.PipelineRunSpec(pipelineName,
				tb.PipelineRunServiceAccount("test-sa"),
				tb.PipelineRunServiceAccountTask("task1", "task1svc"),
				tb.PipelineRunServiceAccountTask("task3", "task3svc"),
				tb.PipelineRunResourceBinding("git-repo", tb.PipelineResourceBindingRef("some-repo")),
				tb.PipelineRunResourceBinding("build-image", tb.PipelineResourceBindingRef("some-image")),
				tb.PipelineRunParam("pipeline-param-1", "somethingmorefun"),
				tb.PipelineRunParam("rev-param", "revision1"),
			),
		),
	}

	objs := []runtime.Object{ps[0], prs[0]}
	pClient := newPipelineClient(objs...)

	cs := test.Clients{
		Pipeline: pClient,
		Kube:     fakekubeclientset.NewSimpleClientset(),
	}

	p := &tu.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pipeline := Command(p)
	got, _ := tu.ExecuteCommand(pipeline, "start", pipelineName,
		"--last",
		"-r=git-repo=scaffold-git",
		"-p=rev-param=revision2",
		"-s=svc1",
		"--task-serviceaccount=task3=task3svc3",
		"--task-serviceaccount=task5=task3svc5",
		"-n=ns")

	expected := "Pipelinerun started: random\n"
	tu.AssertOutput(t, expected, got)

	pr, err := cs.Pipeline.TektonV1alpha1().PipelineRuns(p.Namespace()).Get("random", v1.GetOptions{})

	if err != nil {
		t.Errorf("Error getting pipelineruns %s", err.Error())
	}

	for _, v := range pr.Spec.Resources {
		if v.Name == "git-repo" {
			tu.AssertOutput(t, "scaffold-git", v.ResourceRef.Name)
		}
	}

	for _, v := range pr.Spec.Params {
		if v.Name == "rev-param" {
			tu.AssertOutput(t, "revision2", v.Value)
		}
	}

	for _, v := range pr.Spec.ServiceAccounts {
		if v.TaskName == "task3" {
			tu.AssertOutput(t, "task3svc3", v.ServiceAccount)
		}
	}

	tu.AssertOutput(t, "svc1", pr.Spec.ServiceAccount)
}

func Test_start_pipeline_last_no_pipelineruns(t *testing.T) {

	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "ns",
			tb.PipelineSpec(
				tb.PipelineDeclaredResource("git-repo", "git"),
				tb.PipelineDeclaredResource("build-image", "image"),
				tb.PipelineParam("pipeline-param-1", tb.PipelineParamDefault("somethingdifferent-1")),
				tb.PipelineParam("rev-param", tb.PipelineParamDefault("revision")),
				tb.PipelineTask("unit-test-1", "unit-test-task",
					tb.PipelineTaskInputResource("workspace", "git-repo"),
					tb.PipelineTaskOutputResource("image-to-use", "best-image"),
					tb.PipelineTaskOutputResource("workspace", "git-repo"),
				),
			), // spec
		), // pipeline
	}

	objs := []runtime.Object{ps[0]}
	pClient := newPipelineClient(objs...)

	cs := test.Clients{
		Pipeline: pClient,
		Kube:     fakekubeclientset.NewSimpleClientset(),
	}

	p := &tu.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pipeline := Command(p)
	got, _ := tu.ExecuteCommand(pipeline, "start", pipelineName,
		"--last",
		"-nns")

	expected := "Error: No pipelineruns found in namespace: ns\n"
	tu.AssertOutput(t, expected, got)
}

func Test_start_pipeline_last_list_err(t *testing.T) {

	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "ns",
			tb.PipelineSpec(
				tb.PipelineDeclaredResource("git-repo", "git"),
				tb.PipelineDeclaredResource("build-image", "image"),
				tb.PipelineParam("pipeline-param-1", tb.PipelineParamDefault("somethingdifferent-1")),
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

	cs.Pipeline.PrependReactor("list", "pipelineruns", func(action k8stest.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("test generated error")
	})

	pipeline := Command(p)
	got, _ := tu.ExecuteCommand(pipeline, "start", pipelineName,
		"--last",
		"-nns")

	expected := "Error: test generated error\n"
	tu.AssertOutput(t, expected, got)
}

func Test_start_pipeline_client_error(t *testing.T) {

	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "namespace",
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
		"-r=source=scaffold-git",
		"-p=key1=value1",
		"-s=svc1",
		"-n=namespace")

	expected := "Error: mock error\n"
	tu.AssertOutput(t, expected, got)
}

func Test_start_pipeline_resource_error(t *testing.T) {

	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "namespace",
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
		"-p=key1=value1",
		"-s=svc1",
		"-n=namespace")

	expected := "Error: invalid resource parameter:  scaffold-git\nPlease pass resource as -r ResourceName=ResourceRef\n"

	tu.AssertOutput(t, expected, got)
}

func Test_start_pipeline_param_error(t *testing.T) {

	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "namespace",
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
		"-r=source=scaffold-git",
		"-p value1",
		"-s=svc1",
		"-n=namespace")

	expected := "Error: invalid param parameter:  value1\nPlease pass param as -p ParamName=ParamValue\n"

	tu.AssertOutput(t, expected, got)
}

func Test_start_pipeline_task_svc_error(t *testing.T) {

	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "foo",
			tb.PipelineSpec(
				tb.PipelineDeclaredResource("git-repo", "git"),
				tb.PipelineParam("pipeline-param", tb.PipelineParamDefault("somethingdifferent")),
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
		"--task-serviceaccount=task3svc3",
		"-n=foo")

	expected := "Error: invalid service account parameter: task3svc3\n" +
		"Please pass task service accounts as --task-serviceaccount" +
		" TaskName=ServiceAccount\n"

	tu.AssertOutput(t, expected, got)
}

func Test_parseRes(t *testing.T) {
	type args struct {
		res []string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]v1alpha1.PipelineResourceBinding
		wantErr bool
	}{{
		name: "Test_parseRes No Err",
		args: args{
			res: []string{"source=git", "image=docker2"},
		},
		want: map[string]v1alpha1.PipelineResourceBinding{"source": {
			Name: "source",
			ResourceRef: v1alpha1.PipelineResourceRef{
				Name: "git",
			},
		}, "image": {
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
		want    map[string]v1alpha1.Param
		wantErr bool
	}{{
		name: "Test_parseParam No Err",
		args: args{
			p: []string{"key1=value1", "key2=value2"},
		},
		want: map[string]v1alpha1.Param{
			"key1": {Name: "key1", Value: "value1"},
			"key2": {Name: "key2", Value: "value2"},
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

func Test_parseTaskSvc(t *testing.T) {
	type args struct {
		p []string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]v1alpha1.PipelineRunSpecServiceAccount
		wantErr bool
	}{{
		name: "Test_parseParam No Err",
		args: args{
			p: []string{"key1=value1", "key2=value2"},
		},
		want: map[string]v1alpha1.PipelineRunSpecServiceAccount{
			"key1": {TaskName: "key1", ServiceAccount: "value1"},
			"key2": {TaskName: "key2", ServiceAccount: "value2"},
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
			got, err := parseTaskSvc(tt.args.p)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSvc() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseSvc() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_lastPipelineRun(t *testing.T) {
	clock := clockwork.NewFakeClock()

	pr1Started := clock.Now().Add(10 * time.Second)
	pr2Started := clock.Now().Add(-2 * time.Hour)
	pr3Started := clock.Now().Add(-450 * time.Hour)

	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun("pr-2", "namespace",
			tb.PipelineRunLabel("tekton.dev/pipeline", "test"),
			cb.PipelineRunCreationTimestamp(pr2Started),
			tb.PipelineRunStatus(
				tb.PipelineRunStatusCondition(apis.Condition{
					Status: corev1.ConditionTrue,
					Reason: resources.ReasonRunning,
				}),
			),
		),
		tb.PipelineRun("pr-3", "namespace",
			tb.PipelineRunLabel("tekton.dev/pipeline", "test"),
			cb.PipelineRunCreationTimestamp(pr3Started),
			tb.PipelineRunStatus(
				tb.PipelineRunStatusCondition(apis.Condition{
					Status: corev1.ConditionFalse,
					Reason: resources.ReasonFailed,
				}),
			),
		),
		tb.PipelineRun("pr-1", "namespace",
			tb.PipelineRunLabel("tekton.dev/pipeline", "test"),
			cb.PipelineRunCreationTimestamp(pr1Started),
			tb.PipelineRunStatus(
				tb.PipelineRunStatusCondition(apis.Condition{
					Status: corev1.ConditionTrue,
					Reason: resources.ReasonSucceeded,
				}),
			),
		),
	}

	type args struct {
		p        cli.Params
		pipeline string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "lastPipelineRun Test No Err",
			args: args{
				pipeline: "test",
				p: func() *tu.Params {
					clock.Advance(time.Duration(60) * time.Minute)

					cs, _ := test.SeedTestData(t, test.Data{PipelineRuns: prs})
					p := &tu.Params{Tekton: cs.Pipeline, Clock: clock}
					p.SetNamespace("namespace")
					return p

				}(),
			},
			want:    "pr-1",
			wantErr: false,
		},
		{
			name: "lastPipelineRun Test Err",
			args: args{
				pipeline: "test",
				p: func() *tu.Params {
					cs, _ := test.SeedTestData(t, test.Data{})
					p := &tu.Params{Tekton: cs.Pipeline}
					p.SetNamespace("namespace")
					return p

				}(),
			},

			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs, _ := tt.args.p.Clients()
			got, err := lastPipelineRun(cs, tt.args.pipeline, tt.args.p.Namespace())
			if (err != nil) != tt.wantErr {
				t.Errorf("lastPipelineRun() error = %v, wantErr %v", err, tt.wantErr)
				return
			} else if err == nil {
				tu.AssertOutput(t, tt.want, got.Name)
			}
		})
	}
}
