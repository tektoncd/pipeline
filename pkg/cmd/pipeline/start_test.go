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

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/Netflix/go-expect"
	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/helper/pipeline"
	"github.com/tektoncd/cli/pkg/test"
	cb "github.com/tektoncd/cli/pkg/test/builder"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	fakepipelineclientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	pipelinetest "github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	util_runtime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	k8stest "k8s.io/client-go/testing"
	"knative.dev/pkg/apis"
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

func Test_start_invalid_namespace(t *testing.T) {

	cs, _ := test.SeedTestData(t, pipelinetest.Data{})
	c := Command(&test.Params{Tekton: cs.Pipeline, Kube: cs.Kube})

	_, err := test.ExecuteCommand(c, "start", "task", "-n", "invalid")

	if err == nil {
		t.Error("Expected an error for invalid namespace")
	}

	test.AssertOutput(t, "namespaces \"invalid\" not found", err.Error())
}

func Test_start_has_pipeline_arg(t *testing.T) {
	c := Command(&test.Params{})

	_, err := test.ExecuteCommand(c, "start", "-n", "ns")

	if err == nil {
		t.Error("Expecting an error but it's empty")
	}
	test.AssertOutput(t, "missing pipeline name", err.Error())
}

func Test_start_pipeline_not_found(t *testing.T) {
	ps := []*v1alpha1.Pipeline{
		tb.Pipeline("test-pipeline", "foo",
			tb.PipelineSpec(
				tb.PipelineDeclaredResource("git-repo", "git"),
				tb.PipelineParamSpec("pipeline-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("somethingdifferent")),
				tb.PipelineParamSpec("rev-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("revision")),
				tb.PipelineTask("unit-test-1", "unit-test-task",
					tb.PipelineTaskInputResource("workspace", "git-repo"),
					tb.PipelineTaskOutputResource("image-to-use", "best-image"),
					tb.PipelineTaskOutputResource("workspace", "git-repo"),
				),
			),
		),
	}

	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
		},
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{Pipelines: ps, Namespaces: ns})
	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pipeline := Command(p)
	got, _ := test.ExecuteCommand(pipeline, "start", "test-pipeline-2", "-n", "foo")
	expected := "Error: pipeline name test-pipeline-2 does not exist in namespace foo\n"
	test.AssertOutput(t, expected, got)
}

func Test_start_pipeline(t *testing.T) {
	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "ns",
			tb.PipelineSpec(
				tb.PipelineDeclaredResource("git-repo", "git"),
				tb.PipelineParamSpec("pipeline-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("somethingdifferent")),
				tb.PipelineParamSpec("rev-param", v1alpha1.ParamTypeArray, tb.ParamSpecDefault("booms", "booms", "booms")),
				tb.PipelineTask("unit-test-1", "unit-test-task",
					tb.PipelineTaskInputResource("workspace", "git-repo"),
					tb.PipelineTaskOutputResource("image-to-use", "best-image"),
					tb.PipelineTaskOutputResource("workspace", "git-repo"),
				),
			), // spec
		), // pipeline
	}

	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		},
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{Pipelines: ps, Namespaces: ns})
	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}
	pipeline := Command(p)

	got, _ := test.ExecuteCommand(pipeline, "start", pipelineName,
		"-r=source=scaffold-git",
		"--showlog=false",
		"-p=key1=value1",
		"-p=rev-param=cat,foo,bar",
		"-l=jemange=desfrites",
		"-s=svc1",
		"-n", "ns")

	expected := "Pipelinerun started: \n\nIn order to track the pipelinerun progress run:\ntkn pipelinerun logs  -f -n ns\n"
	test.AssertOutput(t, expected, got)

	pr, err := cs.Pipeline.TektonV1alpha1().PipelineRuns("ns").List(v1.ListOptions{})
	if err != nil {
		t.Errorf("Error listing pipelineruns %s", err.Error())
	}

	if pr.Items[0].ObjectMeta.GenerateName != (pipelineName + "-run-") {
		t.Errorf("Error pipelinerun generated is different %+v", pr)
	}

	for _, v := range pr.Items[0].Spec.Params {
		if v.Name == "rev-param" {
			test.AssertOutput(t, v1alpha1.ArrayOrString{Type: v1alpha1.ParamTypeArray, ArrayVal: []string{"cat", "foo", "bar"}}, v.Value)
		}
	}

	if d := cmp.Equal(pr.Items[0].ObjectMeta.Labels, map[string]string{"jemange": "desfrites"}); !d {
		t.Errorf("Error labels generated is different Labels Got: %+v", pr.Items[0].ObjectMeta.Labels)
	}

}

func Test_start_pipeline_showlogs_false(t *testing.T) {
	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "ns",
			tb.PipelineSpec(
				tb.PipelineDeclaredResource("git-repo", "git"),
				tb.PipelineParamSpec("pipeline-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("somethingdifferent")),
				tb.PipelineParamSpec("rev-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("revision")),
				tb.PipelineTask("unit-test-1", "unit-test-task",
					tb.PipelineTaskInputResource("workspace", "git-repo"),
					tb.PipelineTaskOutputResource("image-to-use", "best-image"),
					tb.PipelineTaskOutputResource("workspace", "git-repo"),
				),
			), // spec
		), // pipeline
	}

	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		},
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{Pipelines: ps, Namespaces: ns})
	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}
	pipeline := Command(p)

	got, _ := test.ExecuteCommand(pipeline, "start", pipelineName,
		"-r=source=scaffold-git",
		"-p=key1=value1",
		"-l=jemange=desfrites",
		"--showlog=false",
		"-s=svc1",
		"-n", "ns")

	expected := "Pipelinerun started: \n\nIn order to track the pipelinerun progress run:\ntkn pipelinerun logs  -f -n ns\n"
	test.AssertOutput(t, expected, got)
}

func Test_start_pipeline_interactive(t *testing.T) {

	pipelineName := "test-pipeline"

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		Pipelines: []*v1alpha1.Pipeline{
			tb.Pipeline(pipelineName, "ns",
				tb.PipelineSpec(
					tb.PipelineDeclaredResource("git-repo", "git"),
					tb.PipelineParamSpec("pipeline-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("somethingdifferent")),
					tb.PipelineParamSpec("rev-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("revision")),
					tb.PipelineTask("unit-test-1", "unit-test-task",
						tb.PipelineTaskInputResource("workspace", "git-repo"),
						tb.PipelineTaskOutputResource("image-to-use", "best-image"),
						tb.PipelineTaskOutputResource("workspace", "git-repo"),
					),
				),
			),
		},
		PipelineResources: []*v1alpha1.PipelineResource{
			tb.PipelineResource("scaffold-git", "ns",
				tb.PipelineResourceSpec("git",
					tb.PipelineResourceSpecParam("url", "git@github.com:tektoncd/cli.git"),
				),
			),
		},
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	tests := []promptTest{
		{
			name:    "basic interaction",
			cmdArgs: []string{pipelineName},

			procedure: func(c *expect.Console) error {
				if _, err := c.ExpectString("Choose the git resource to use for git-repo:"); err != nil {
					return err
				}

				if _, err := c.SendLine(string(terminal.KeyArrowDown)); err != nil {
					return err
				}

				if _, err := c.ExpectString("scaffold-git (git@github.com:tektoncd/cli.git)"); err != nil {
					return err
				}

				if _, err := c.SendLine(string(terminal.KeyEnter)); err != nil {
					return err
				}

				if _, err := c.ExpectString("Value of param `pipeline-param` ?"); err != nil {
					return err
				}

				if _, err := c.SendLine("test"); err != nil {
					return err
				}

				if _, err := c.ExpectString("Value of param `rev-param` ?"); err != nil {
					return err
				}

				if _, err := c.SendLine("test2"); err != nil {
					return err
				}

				if _, err := c.ExpectString("Pipelinerun started:"); err != nil {
					return err
				}

				if _, err := c.ExpectEOF(); err != nil {
					return err
				}

				c.Close()
				return nil
			},
		},
	}
	opts := startOpts("ns", cs, false, "svc1", []string{"task1=svc1"})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			opts.RunPromptTest(t, test)
		})
	}
}

func Test_start_pipeline_no_resource(t *testing.T) {

	pipelineName := "test-pipeline"

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		Pipelines: []*v1alpha1.Pipeline{
			tb.Pipeline(pipelineName, "ns",
				tb.PipelineSpec(
					tb.PipelineDeclaredResource("git-repo", "git"),
					tb.PipelineTask("unit-test-1", "unit-test-task",
						tb.PipelineTaskInputResource("workspace", "git-repo"),
						tb.PipelineTaskOutputResource("image-to-use", "best-image"),
						tb.PipelineTaskOutputResource("workspace", "git-repo"),
					),
				),
			),
		},
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pipeline := Command(p)
	got, _ := test.ExecuteCommand(pipeline, "start", pipelineName, "-n", "ns")
	expected := "Error: " + "no pipeline resource of type git found in namespace: ns\n"
	test.AssertOutput(t, expected, got)
}

func Test_start_pipeline_last(t *testing.T) {

	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "ns",
			tb.PipelineSpec(
				tb.PipelineDeclaredResource("git-repo", "git"),
				tb.PipelineDeclaredResource("build-image", "image"),
				tb.PipelineParamSpec("pipeline-param-1", v1alpha1.ParamTypeString, tb.ParamSpecDefault("somethingdifferent-1")),
				tb.PipelineParamSpec("rev-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("revision")),
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
				tb.PipelineRunDeprecatedServiceAccountName("", "test-sa"),
				tb.PipelineRunResourceBinding("git-repo", tb.PipelineResourceBindingRef("some-repo")),
				tb.PipelineRunResourceBinding("build-image", tb.PipelineResourceBindingRef("some-image")),
				tb.PipelineRunParam("pipeline-param-1", "somethingmorefun"),
				tb.PipelineRunParam("rev-param", "revision1"),
			),
		),
	}

	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		},
	}

	//Add namespaces to kube client
	seedData, _ := test.SeedTestData(t, pipelinetest.Data{Namespaces: ns})

	objs := []runtime.Object{ps[0], prs[0]}
	pClient := newPipelineClient(objs...)

	cs := pipelinetest.Clients{
		Pipeline: pClient,
		Kube:     seedData.Kube,
	}

	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pipeline := Command(p)
	got, _ := test.ExecuteCommand(pipeline, "start", pipelineName,
		"--last",
		"-r=git-repo=scaffold-git",
		"-p=rev-param=revision2",
		"-s=svc1",
		"--showlog=false",
		"--task-serviceaccount=task3=task3svc3",
		"--task-serviceaccount=task5=task3svc5",
		"-n", "ns")

	expected := "Pipelinerun started: random\n\nIn order to track the pipelinerun progress run:\ntkn pipelinerun logs random -f -n ns\n"
	test.AssertOutput(t, expected, got)

	pr, err := cs.Pipeline.TektonV1alpha1().PipelineRuns(p.Namespace()).Get("random", v1.GetOptions{})

	if err != nil {
		t.Errorf("Error getting pipelineruns %s", err.Error())
	}

	for _, v := range pr.Spec.Resources {
		if v.Name == "git-repo" {
			test.AssertOutput(t, "scaffold-git", v.ResourceRef.Name)
		}
	}

	for _, v := range pr.Spec.Params {
		if v.Name == "rev-param" {
			test.AssertOutput(t, v1alpha1.ArrayOrString{Type: v1alpha1.ParamTypeString, StringVal: "revision2"}, v.Value)
		}
	}
	test.AssertOutput(t, "svc1", pr.Spec.DeprecatedServiceAccount)
}

func Test_start_pipeline_last_without_res_param(t *testing.T) {

	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "ns",
			tb.PipelineSpec(
				tb.PipelineDeclaredResource("git-repo", "git"),
				tb.PipelineDeclaredResource("build-image", "image"),
				tb.PipelineParamSpec("pipeline-param-1", v1alpha1.ParamTypeString, tb.ParamSpecDefault("somethingdifferent-1")),
				tb.PipelineParamSpec("rev-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("revision")),
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
				tb.PipelineRunDeprecatedServiceAccountName("", "test-sa"),
				tb.PipelineRunResourceBinding("git-repo", tb.PipelineResourceBindingRef("some-repo")),
				tb.PipelineRunResourceBinding("build-image", tb.PipelineResourceBindingRef("some-image")),
				tb.PipelineRunParam("pipeline-param-1", "somethingmorefun"),
				tb.PipelineRunParam("rev-param", "revision1"),
			),
		),
	}

	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		},
	}

	//Add namespaces to kube client
	seedData, _ := test.SeedTestData(t, pipelinetest.Data{Namespaces: ns})

	objs := []runtime.Object{ps[0], prs[0]}
	pClient := newPipelineClient(objs...)

	cs := pipelinetest.Clients{
		Pipeline: pClient,
		Kube:     seedData.Kube,
	}

	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pipeline := Command(p)
	got, _ := test.ExecuteCommand(pipeline, "start", pipelineName,
		"--last",
		"--showlog=false",
		"-n", "ns")

	expected := "Pipelinerun started: random\n\nIn order to track the pipelinerun progress run:\ntkn pipelinerun logs random -f -n ns\n"
	test.AssertOutput(t, expected, got)

	pr, err := cs.Pipeline.TektonV1alpha1().PipelineRuns(p.Namespace()).Get("random", v1.GetOptions{})

	if err != nil {
		t.Errorf("Error getting pipelineruns %s", err.Error())
	}

	for _, v := range pr.Spec.Resources {
		if v.Name == "git-repo" {
			test.AssertOutput(t, "some-repo", v.ResourceRef.Name)
		}
	}

	for _, v := range pr.Spec.Params {
		if v.Name == "rev-param" {
			test.AssertOutput(t, v1alpha1.ArrayOrString{Type: v1alpha1.ParamTypeString, StringVal: "revision1"}, v.Value)
		}
	}
	test.AssertOutput(t, "test-sa", pr.Spec.DeprecatedServiceAccount)
}

func Test_start_pipeline_last_merge(t *testing.T) {

	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "ns",
			tb.PipelineSpec(
				tb.PipelineDeclaredResource("git-repo", "git"),
				tb.PipelineDeclaredResource("build-image", "image"),
				tb.PipelineParamSpec("pipeline-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("somethingdifferent-1")),
				tb.PipelineParamSpec("rev-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("revision")),
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
				tb.PipelineRunDeprecatedServiceAccountName("", "test-sa"),
				tb.PipelineRunDeprecatedServiceAccountTask("task1", "task1svc"),
				tb.PipelineRunDeprecatedServiceAccountTask("task3", "task3svc"),
				tb.PipelineRunResourceBinding("git-repo", tb.PipelineResourceBindingRef("some-repo")),
				tb.PipelineRunResourceBinding("build-image", tb.PipelineResourceBindingRef("some-image")),
				tb.PipelineRunParam("pipeline-param-1", "somethingmorefun"),
				tb.PipelineRunParam("rev-param", "revision1"),
			),
		),
	}

	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		},
	}

	//Add namespaces to kube client
	seedData, _ := test.SeedTestData(t, pipelinetest.Data{Namespaces: ns})

	objs := []runtime.Object{ps[0], prs[0]}
	pClient := newPipelineClient(objs...)

	cs := pipelinetest.Clients{
		Pipeline: pClient,
		Kube:     seedData.Kube,
	}

	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pipeline := Command(p)
	got, _ := test.ExecuteCommand(pipeline, "start", pipelineName,
		"--last",
		"--showlog=false",
		"-s=svc1",
		"-r=git-repo=scaffold-git",
		"-p=rev-param=revision2",
		"--task-serviceaccount=task3=task3svc3",
		"--task-serviceaccount=task5=task3svc5",
		"-n=ns")

	expected := "Pipelinerun started: random\n\nIn order to track the pipelinerun progress run:\ntkn pipelinerun logs random -f -n ns\n"
	test.AssertOutput(t, expected, got)

	pr, err := cs.Pipeline.TektonV1alpha1().PipelineRuns(p.Namespace()).Get("random", v1.GetOptions{})

	if err != nil {
		t.Errorf("Error getting pipelineruns %s", err.Error())
	}

	for _, v := range pr.Spec.Resources {
		if v.Name == "git-repo" {
			test.AssertOutput(t, "scaffold-git", v.ResourceRef.Name)
		}
	}

	for _, v := range pr.Spec.Params {
		if v.Name == "rev-param" {
			test.AssertOutput(t, v1alpha1.ArrayOrString{Type: v1alpha1.ParamTypeString, StringVal: "revision2"}, v.Value)
		}
	}

	for _, v := range pr.Spec.DeprecatedServiceAccounts {
		if v.TaskName == "task3" {
			test.AssertOutput(t, "task3svc3", v.DeprecatedServiceAccount)
		}
	}

	test.AssertOutput(t, "svc1", pr.Spec.DeprecatedServiceAccount)
}

func Test_start_pipeline_last_no_pipelineruns(t *testing.T) {

	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "ns",
			tb.PipelineSpec(
				tb.PipelineDeclaredResource("git-repo", "git"),
				tb.PipelineDeclaredResource("build-image", "image"),
				tb.PipelineParamSpec("pipeline-param-1", v1alpha1.ParamTypeString, tb.ParamSpecDefault("somethingdifferent-1")),
				tb.PipelineParamSpec("rev-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("revision")),
				tb.PipelineTask("unit-test-1", "unit-test-task",
					tb.PipelineTaskInputResource("workspace", "git-repo"),
					tb.PipelineTaskOutputResource("image-to-use", "best-image"),
					tb.PipelineTaskOutputResource("workspace", "git-repo"),
				),
			), // spec
		), // pipeline
	}

	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		},
	}

	//Add namespaces to kube client
	seedData, _ := test.SeedTestData(t, pipelinetest.Data{Namespaces: ns})

	objs := []runtime.Object{ps[0]}
	pClient := newPipelineClient(objs...)

	cs := pipelinetest.Clients{
		Pipeline: pClient,
		Kube:     seedData.Kube,
	}

	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pipeline := Command(p)
	got, _ := test.ExecuteCommand(pipeline, "start", pipelineName,
		"--last",
		"-s=svc1",
		"--showlog=false",
		"-r=git-repo=scaffold-git",
		"-p=rev-param=revision2",
		"--task-serviceaccount=task3=task3svc3",
		"--task-serviceaccount=task5=task3svc5",
		"-n", "ns")

	expected := "Error: no pipelineruns related to pipeline test-pipeline found in namespace ns\n"
	test.AssertOutput(t, expected, got)
}

func Test_start_pipeline_last_list_err(t *testing.T) {

	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "ns",
			tb.PipelineSpec(
				tb.PipelineDeclaredResource("git-repo", "git"),
				tb.PipelineDeclaredResource("build-image", "image"),
				tb.PipelineParamSpec("pipeline-param-1", v1alpha1.ParamTypeString, tb.ParamSpecDefault("somethingdifferent-1")),
				tb.PipelineParamSpec("rev-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("revision")),
				tb.PipelineTask("unit-test-1", "unit-test-task",
					tb.PipelineTaskInputResource("workspace", "git-repo"),
					tb.PipelineTaskOutputResource("image-to-use", "best-image"),
					tb.PipelineTaskOutputResource("workspace", "git-repo"),
				),
			), // spec
		), // pipeline
	}

	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		},
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{Pipelines: ps, Namespaces: ns})
	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	cs.Pipeline.PrependReactor("list", "pipelineruns", func(action k8stest.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("test generated error")
	})

	pipeline := Command(p)
	got, _ := test.ExecuteCommand(pipeline, "start", pipelineName,
		"--last",
		"-s=svc1",
		"--showlog=false",
		"-r=git-repo=scaffold-git",
		"-p=rev-param=revision2",
		"--task-serviceaccount=task3=task3svc3",
		"--task-serviceaccount=task5=task3svc5",
		"-n", "ns")

	expected := "Error: test generated error\n"
	test.AssertOutput(t, expected, got)
}

func Test_start_pipeline_client_error(t *testing.T) {

	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "namespace",
			tb.PipelineSpec(
				tb.PipelineTask("unit-test-1", "unit-test-task",
					tb.PipelineTaskInputResource("workspace", "git-repo"),
					tb.PipelineTaskOutputResource("image-to-use", "best-image"),
					tb.PipelineTaskOutputResource("workspace", "git-repo"),
				),
			),
		),
	}

	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "namespace",
			},
		},
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{Pipelines: ps, Namespaces: ns})

	cs.Pipeline.PrependReactor("create", "*", func(_ k8stest.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("mock error")
	})
	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pipeline := Command(p)
	got, _ := test.ExecuteCommand(pipeline, "start", pipelineName,
		"--showlog=false",
		"-s=svc1",
		"-n=namespace")

	expected := "Error: mock error\n"
	test.AssertOutput(t, expected, got)
}

func Test_start_pipeline_res_err(t *testing.T) {

	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "ns",
			tb.PipelineSpec(
				tb.PipelineDeclaredResource("git-repo", "git"),
				tb.PipelineDeclaredResource("build-image", "image"),
				tb.PipelineParamSpec("pipeline-param-1", v1alpha1.ParamTypeString, tb.ParamSpecDefault("somethingdifferent-1")),
				tb.PipelineParamSpec("rev-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("revision")),
				tb.PipelineTask("unit-test-1", "unit-test-task",
					tb.PipelineTaskInputResource("workspace", "git-repo"),
					tb.PipelineTaskOutputResource("image-to-use", "best-image"),
					tb.PipelineTaskOutputResource("workspace", "git-repo"),
				),
			), // spec
		), // pipeline
	}

	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		},
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{Pipelines: ps, Namespaces: ns})
	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pipeline := Command(p)
	got, _ := test.ExecuteCommand(pipeline, "start", pipelineName,
		"-s=svc1",
		"--showlog=false",
		"-r=git-reposcaffold-git",
		"-p=rev-param=revision2",
		"--task-serviceaccount=task3=task3svc3",
		"--task-serviceaccount=task5=task3svc5",
		"-n", "ns")

	expected := "Error: invalid input format for resource parameter : git-reposcaffold-git\n"
	test.AssertOutput(t, expected, got)
}

func Test_start_pipeline_param_err(t *testing.T) {

	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "ns",
			tb.PipelineSpec(
				tb.PipelineDeclaredResource("git-repo", "git"),
				tb.PipelineDeclaredResource("build-image", "image"),
				tb.PipelineParamSpec("pipeline-param-1", v1alpha1.ParamTypeString, tb.ParamSpecDefault("somethingdifferent-1")),
				tb.PipelineParamSpec("rev-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("revision")),
				tb.PipelineTask("unit-test-1", "unit-test-task",
					tb.PipelineTaskInputResource("workspace", "git-repo"),
					tb.PipelineTaskOutputResource("image-to-use", "best-image"),
					tb.PipelineTaskOutputResource("workspace", "git-repo"),
				),
			), // spec
		), // pipeline
	}

	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		},
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{Pipelines: ps, Namespaces: ns})
	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pipeline := Command(p)
	got, _ := test.ExecuteCommand(pipeline, "start", pipelineName,
		"-s=svc1",
		"--showlog=false",
		"-r=git-repo=scaffold-git",
		"-p=rev-paramrevision2",
		"--task-serviceaccount=task3=task3svc3",
		"--task-serviceaccount=task5=task3svc5",
		"-n", "ns")

	expected := "Error: invalid input format for param parameter : rev-paramrevision2\n"
	test.AssertOutput(t, expected, got)
}

func Test_start_pipeline_label_err(t *testing.T) {

	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "ns",
			tb.PipelineSpec(
				tb.PipelineDeclaredResource("git-repo", "git"),
				tb.PipelineDeclaredResource("build-image", "image"),
				tb.PipelineParamSpec("pipeline-param-1", v1alpha1.ParamTypeString, tb.ParamSpecDefault("somethingdifferent-1")),
				tb.PipelineParamSpec("rev-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("revision")),
				tb.PipelineTask("unit-test-1", "unit-test-task",
					tb.PipelineTaskInputResource("workspace", "git-repo"),
					tb.PipelineTaskOutputResource("image-to-use", "best-image"),
					tb.PipelineTaskOutputResource("workspace", "git-repo"),
				),
			), // spec
		), // pipeline
	}

	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		},
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{Pipelines: ps, Namespaces: ns})
	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pipeline := Command(p)
	got, _ := test.ExecuteCommand(pipeline, "start", pipelineName,
		"-s=svc1",
		"-r=git-repo=scaffold-git",
		"-p=rev-param=revision2",
		"-l=keyvalue",
		"--task-serviceaccount=task3=task3svc3",
		"--task-serviceaccount=task5=task3svc5",
		"-n", "ns")

	expected := "Error: invalid input format for label parameter : keyvalue\n"
	test.AssertOutput(t, expected, got)
}

func Test_start_pipeline_task_svc_error(t *testing.T) {

	pipelineName := "test-pipeline"

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, "foo",
			tb.PipelineSpec(
				tb.PipelineTask("unit-test-1", "unit-test-task",
					tb.PipelineTaskInputResource("workspace", "git-repo"),
					tb.PipelineTaskOutputResource("image-to-use", "best-image"),
					tb.PipelineTaskOutputResource("workspace", "git-repo"),
				),
			),
		),
	}

	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
		},
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{Pipelines: ps, Namespaces: ns})

	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pipeline := Command(p)
	got, _ := test.ExecuteCommand(pipeline, "start", pipelineName,
		"--task-serviceaccount=task3svc3",
		"-n=foo")

	expected := "Error: invalid service account parameter: task3svc3\n" +
		"Please pass task service accounts as --task-serviceaccount" +
		" TaskName=ServiceAccount\n"

	test.AssertOutput(t, expected, got)
}

func Test_mergeResource(t *testing.T) {
	pr := &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    "ns",
			GenerateName: "test-run",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{Name: "test"},
			Resources: []v1alpha1.PipelineResourceBinding{
				{
					Name: "source",
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "git",
					},
				},
			},
		},
	}

	err := mergeRes(pr, []string{"test"})
	if err == nil {
		t.Errorf("Expected error")
	}

	err = mergeRes(pr, []string{})
	if err != nil {
		t.Errorf("Did not expect error")
	}
	test.AssertOutput(t, 1, len(pr.Spec.Resources))

	err = mergeRes(pr, []string{"image=test-1"})
	if err != nil {
		t.Errorf("Did not expect error")
	}
	test.AssertOutput(t, 2, len(pr.Spec.Resources))

	err = mergeRes(pr, []string{"image=test-new", "image-2=test-2"})
	if err != nil {
		t.Errorf("Did not expect error")
	}
	test.AssertOutput(t, 3, len(pr.Spec.Resources))
}

func Test_getPipelineResourceByFormat(t *testing.T) {
	pipelineResources := []*v1alpha1.PipelineResource{
		tb.PipelineResource("scaffold-git", "ns",
			tb.PipelineResourceSpec("git",
				tb.PipelineResourceSpecParam("url", "git@github.com:tektoncd/cli.git"),
			),
		),
		tb.PipelineResource("scaffold-git-fork", "ns",
			tb.PipelineResourceSpec("git",
				tb.PipelineResourceSpecParam("url", "git@github.com:tektoncd-fork/cli.git"),
				tb.PipelineResourceSpecParam("revision", "release"),
			),
		),
		tb.PipelineResource("scaffold-image", "ns",
			tb.PipelineResourceSpec("image",
				tb.PipelineResourceSpecParam("url", "docker.io/tektoncd/cli"),
			),
		),
		tb.PipelineResource("scaffold-pull", "ns",
			tb.PipelineResourceSpec("pullRequest",
				tb.PipelineResourceSpecParam("url", "https://github.com/tektoncd/cli/pulls/9"),
			),
		),
		tb.PipelineResource("scaffold-cluster", "ns",
			tb.PipelineResourceSpec("cluster",
				tb.PipelineResourceSpecParam("url", "https://opemshift.com"),
				tb.PipelineResourceSpecParam("user", "tektoncd-developer"),
			),
		),
		tb.PipelineResource("scaffold-storage", "ns",
			tb.PipelineResourceSpec("storage",
				tb.PipelineResourceSpecParam("location", "/home/tektoncd"),
			),
		),
	}

	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		},
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{PipelineResources: pipelineResources, Namespaces: ns})
	res, _ := getPipelineResources(cs.Pipeline, "ns")
	resFormat := getPipelineResourcesByFormat(res.Items)

	output := getOptionsByType(resFormat, "git")
	expected := []string{"scaffold-git (git@github.com:tektoncd/cli.git)", "scaffold-git-fork (git@github.com:tektoncd-fork/cli.git#release)"}
	if !reflect.DeepEqual(output, expected) {
		t.Errorf("output git = %v, want %v", output, expected)
	}

	output = getOptionsByType(resFormat, "image")
	expected = []string{"scaffold-image (docker.io/tektoncd/cli)"}
	if !reflect.DeepEqual(output, expected) {
		t.Errorf("output image = %v, want %v", output, expected)
	}

	output = getOptionsByType(resFormat, "pullRequest")
	expected = []string{"scaffold-pull (https://github.com/tektoncd/cli/pulls/9)"}
	if !reflect.DeepEqual(output, expected) {
		t.Errorf("output pullRequest = %v, want %v", output, expected)
	}

	output = getOptionsByType(resFormat, "cluster")
	expected = []string{"scaffold-cluster (https://opemshift.com#tektoncd-developer)"}
	if !reflect.DeepEqual(output, expected) {
		t.Errorf("output cluster = %v, want %v", output, expected)
	}

	output = getOptionsByType(resFormat, "storage")
	expected = []string{"scaffold-storage (/home/tektoncd)"}
	if !reflect.DeepEqual(output, expected) {
		t.Errorf("output storage = %v, want %v", output, expected)
	}

	output = getOptionsByType(resFormat, "file")
	expected = []string{}
	if !reflect.DeepEqual(output, expected) {
		t.Errorf("output error = %v, want %v", output, expected)
	}
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

func Test_parseTaskSvc(t *testing.T) {
	type args struct {
		p []string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]v1alpha1.PipelineRunSpecServiceAccountName
		wantErr bool
	}{{
		name: "Test_parseParam No Err",
		args: args{
			p: []string{"key1=value1", "key2=value2"},
		},
		want: map[string]v1alpha1.PipelineRunSpecServiceAccountName{
			"key1": {TaskName: "key1", ServiceAccountName: "value1"},
			"key2": {TaskName: "key2", ServiceAccountName: "value2"},
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

func Test_parseDeprecatedTaskSvc(t *testing.T) {
	type args struct {
		p []string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]v1alpha1.DeprecatedPipelineRunSpecServiceAccount
		wantErr bool
	}{{
		name: "Test_parseParam No Err",
		args: args{
			p: []string{"key1=value1", "key2=value2"},
		},
		want: map[string]v1alpha1.DeprecatedPipelineRunSpecServiceAccount{
			"key1": {TaskName: "key1", DeprecatedServiceAccount: "value1"},
			"key2": {TaskName: "key2", DeprecatedServiceAccount: "value2"},
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
			got, err := parseDeprecatedTaskSvc(tt.args.p)
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

	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "namespace",
			},
		},
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
				p: func() *test.Params {
					clock.Advance(time.Duration(60) * time.Minute)

					cs, _ := test.SeedTestData(t, pipelinetest.Data{PipelineRuns: prs, Namespaces: ns})
					p := &test.Params{Tekton: cs.Pipeline, Clock: clock}
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
				p: func() *test.Params {
					cs, _ := test.SeedTestData(t, pipelinetest.Data{Namespaces: ns})
					p := &test.Params{Tekton: cs.Pipeline}
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
			got, err := pipeline.LastRun(cs.Tekton, tt.args.pipeline, tt.args.p.Namespace())
			if (err != nil) != tt.wantErr {
				t.Errorf("lastPipelineRun() error = %v, wantErr %v", err, tt.wantErr)
				return
			} else if err == nil {
				test.AssertOutput(t, tt.want, got.Name)
			}
		})
	}
}

func startOpts(ns string, cs pipelinetest.Clients, last bool, svc string, svcs []string) *startOptions {
	p := test.Params{
		Kube:   cs.Kube,
		Tekton: cs.Pipeline,
	}
	p.SetNamespace(ns)
	startOp := startOptions{
		cliparams:          &p,
		Last:               last,
		ServiceAccountName: svc,
		ServiceAccounts:    svcs,
	}

	return &startOp
}
