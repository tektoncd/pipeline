package e2e

import (
	"bytes"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"testing"
	"text/tabwriter"
	"text/template"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"

	"github.com/tektoncd/cli/pkg/formatted"
	prhsort "github.com/tektoncd/cli/pkg/helper/pipelinerun/sort"
	trhsort "github.com/tektoncd/cli/pkg/helper/taskrun/sort"
	"github.com/tektoncd/cli/pkg/helper/validate"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DescribeTemplateForTaskList(cs *Clients) string {

	const (
		emptyMsg = "No tasks found"
		header   = "NAME\tAGE"
		body     = "%s\t%s\n"
	)

	log.Print("validating Task List command\n")
	clock := clockwork.NewFakeClockAt(time.Now())
	task := GetTaskList(cs)
	var tmplBytes bytes.Buffer
	w := tabwriter.NewWriter(&tmplBytes, 0, 5, 3, ' ', tabwriter.TabIndent)

	if len(task.Items) == 0 {
		fmt.Fprintln(w, emptyMsg)
		w.Flush()
		return tmplBytes.String()
	}
	fmt.Fprintln(w, header)
	for _, task := range task.Items {
		fmt.Fprintf(w, body,
			task.Name,
			formatted.Age(&task.CreationTimestamp, clock),
		)
	}
	w.Flush()
	return tmplBytes.String()
}

func DescribeTemplateForTaskRunList(cs *Clients) string {

	const (
		emptyMsg = "No taskruns found"
		header   = "NAME\tSTARTED\tDURATION\tSTATUS\t"
		body     = "%s\t%s\t%s\t%s\t\n"
	)

	clock := clockwork.NewFakeClockAt(time.Now())
	taskrun := GetTaskRunList(cs)
	trslen := len(taskrun.Items)

	if trslen != 0 {
		taskrun.Items = trhsort.SortTaskRunsByStartTime(taskrun.Items)
	}

	var tmplBytes bytes.Buffer
	w := tabwriter.NewWriter(&tmplBytes, 0, 5, 3, ' ', tabwriter.TabIndent)

	if len(taskrun.Items) == 0 {
		fmt.Fprintln(w, emptyMsg)
		w.Flush()
		return tmplBytes.String()
	}
	fmt.Fprintln(w, header)
	for _, tr := range taskrun.Items {
		fmt.Fprintf(w, body,
			tr.Name,
			formatted.Age(tr.Status.StartTime, clock),
			formatted.Duration(tr.Status.StartTime, tr.Status.CompletionTime),
			formatted.Condition(tr.Status.Conditions),
		)
	}
	w.Flush()
	return tmplBytes.String()
}

type pipelineruns map[string]v1alpha1.PipelineRun

func DescribeTemplateForPipelineResourceList(cs *Clients) string {

	const (
		emptyMsg = "No pipelineresources found."
		header   = "NAME\tTYPE\tDETAILS"
		body     = "%s\t%s\t%s\n"
	)

	log.Print("validating Pipeline Resources List command\n")

	pipelineResourcelist := GetPipelineResourceList(cs)

	var tmplBytes bytes.Buffer
	w := tabwriter.NewWriter(&tmplBytes, 0, 5, 3, ' ', tabwriter.TabIndent)

	if len(pipelineResourcelist.Items) == 0 {
		fmt.Fprintln(w, emptyMsg)
		w.Flush()
		return tmplBytes.String()
	}
	fmt.Fprintln(w, header)
	for _, pre := range pipelineResourcelist.Items {
		fmt.Fprintf(w, body,
			pre.Name,
			pre.Spec.Type,
			Details(pre),
		)
	}
	w.Flush()

	return tmplBytes.String()
}

func ListResourceNamesForJsonPath(obj interface{}) string {
	const (
		emptyMsg = ""
		body     = "%s\n"
	)
	var tmplBytes bytes.Buffer
	w := tabwriter.NewWriter(&tmplBytes, 0, 5, 3, ' ', tabwriter.TabIndent)

	switch obj := obj.(type) {
	case *v1alpha1.TaskList:

		if len(obj.Items) == 0 {

			return emptyMsg
		}

		for _, r := range obj.Items {
			fmt.Fprintf(w, body,
				r.Name,
			)
		}
		w.Flush()
		return tmplBytes.String()
	case *v1alpha1.TaskRunList:

		if len(obj.Items) == 0 {

			return emptyMsg
		}
		//sort by start Time
		trslen := len(obj.Items)
		if trslen != 0 {
			obj.Items = trhsort.SortTaskRunsByStartTime(obj.Items)
		}

		for _, r := range obj.Items {
			fmt.Fprintf(w, body,
				r.Name,
			)
		}
		w.Flush()
		return tmplBytes.String()
	case *v1alpha1.PipelineList:
		if len(obj.Items) == 0 {
			return emptyMsg
		}

		for _, r := range obj.Items {
			fmt.Fprintf(w, body,
				r.Name,
			)
		}
		w.Flush()
		return tmplBytes.String()

	case *v1alpha1.PipelineRunList:
		if len(obj.Items) == 0 {
			return emptyMsg
		}

		for _, r := range obj.Items {
			fmt.Fprintf(w, body,
				r.Name,
			)
		}
		w.Flush()
		return tmplBytes.String()
	case *v1alpha1.PipelineResourceList:
		if len(obj.Items) == 0 {
			return emptyMsg
		}

		for _, r := range obj.Items {
			fmt.Fprintf(w, body,
				r.Name,
			)
		}
		w.Flush()
		return tmplBytes.String()
	case *v1alpha1.ClusterTaskList:
		if len(obj.Items) == 0 {
			return emptyMsg
		}

		for _, r := range obj.Items {
			fmt.Fprintf(w, body,
				r.Name,
			)
		}
		w.Flush()
		return tmplBytes.String()
	}

	return ""

}

func GetTask(c *Clients, name string) *v1alpha1.Task {

	task, err := c.TaskClient.Get(name, metav1.GetOptions{})
	if err != nil {
		log.Fatalf("Couldn't get expected task  %s", err)
	}

	return task
}

func GetTaskList(c *Clients) *v1alpha1.TaskList {

	tasklist, err := c.TaskClient.List(metav1.ListOptions{})
	// 	require.Nil(t, err)
	if err != nil {
		log.Fatalf("Couldn't get expected tasklist  %s", err)
	}

	return tasklist
}

func GetTaskRun(c *Clients, name string) *v1alpha1.TaskRun {

	taskRun, err := c.TaskRunClient.Get(name, metav1.GetOptions{})
	// 	require.Nil(t, err)
	if err != nil {
		log.Fatalf("Couldn't get expected taskRun  %s", err)
	}

	return taskRun
}

func GetTaskRunList(c *Clients) *v1alpha1.TaskRunList {

	taskRunlist, err := c.TaskRunClient.List(metav1.ListOptions{})
	// 	require.Nil(t, err)
	if err != nil {
		log.Fatalf("Couldn't get expected taskRunlist  %s", err)
	}

	return taskRunlist
}

func GetPipelineResource(c *Clients, name string) *v1alpha1.PipelineResource {

	pipelineResource, err := c.PipelineResourceClient.Get(name, metav1.GetOptions{})
	// 	require.Nil(t, err)
	if err != nil {
		log.Fatalf("Couldn't get expected pipelineResource  %s", err)
	}

	return pipelineResource
}

func GetPipelineResourceList(c *Clients) *v1alpha1.PipelineResourceList {

	pipelineResourceList, err := c.PipelineResourceClient.List(metav1.ListOptions{})
	// 	require.Nil(t, err)
	if err != nil {
		log.Fatalf("Couldn't get expected pipelineResourceList  %s", err)
	}

	return pipelineResourceList
}

func GetPipeline(c *Clients, name string) *v1alpha1.Pipeline {

	pipeline, err := c.PipelineClient.Get(name, metav1.GetOptions{})
	// 	require.Nil(t, err)
	if err != nil {
		log.Fatalf("Couldn't get expected pipeline  %s", err)
	}

	return pipeline
}

func GetPipelineList(c *Clients) *v1alpha1.PipelineList {

	pipelineList, err := c.PipelineClient.List(metav1.ListOptions{})
	// 	require.Nil(t, err)
	if err != nil {
		log.Fatalf("Couldn't get expected pipelineList  %s", err)
	}

	return pipelineList
}

func GetPipelineRun(c *Clients, name string) *v1alpha1.PipelineRun {

	pipelineRun, err := c.PipelineRunClient.Get(name, metav1.GetOptions{})
	// 	require.Nil(t, err)
	if err != nil {
		log.Fatalf("Couldn't get expected pipelineRun  %s", err)
	}

	return pipelineRun
}

func GetPipelineRunListWithName(c *Clients, pname string) *v1alpha1.PipelineRunList {
	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("tekton.dev/pipeline=%s", pname),
	}
	pipelineRunList, err := c.PipelineRunClient.List(opts)
	// 	require.Nil(t, err)
	if err != nil {
		log.Fatalf("Couldn't get expected pipelineRunList  %s", err)
	}

	return pipelineRunList
}

func GetPipelineRunList(c *Clients) *v1alpha1.PipelineRunList {

	pipelineRunList, err := c.PipelineRunClient.List(metav1.ListOptions{})
	// 	require.Nil(t, err)
	if err != nil {
		log.Fatalf("Couldn't get expected pipelineRunList  %s", err)
	}

	return pipelineRunList
}

////////////////////TestData//////////////////////
type TaskData struct {
	Name string
}

func ListAllTasksOutput(t *testing.T, cs *Clients, td map[int]interface{}) string {
	t.Helper()
	const (
		emptyMsg = "No tasks found"
		header   = "NAME\tAGE"
		body     = "%s\t%s\n"
	)

	clock := clockwork.NewFakeClockAt(time.Now())

	task := GetTaskListWithTestData(t, cs, td)

	var tmplBytes bytes.Buffer
	w := tabwriter.NewWriter(&tmplBytes, 0, 5, 3, ' ', tabwriter.TabIndent)

	if len(task.Items) == 0 {
		fmt.Fprintln(w, emptyMsg)
		w.Flush()
		return tmplBytes.String()
	}
	fmt.Fprintln(w, header)

	for _, task := range task.Items {
		fmt.Fprintf(w, body,
			task.Name,
			formatted.Age(&task.CreationTimestamp, clock),
		)
	}
	w.Flush()
	return tmplBytes.String()
}

func GetTaskListWithTestData(t *testing.T, c *Clients, td map[int]interface{}) *v1alpha1.TaskList {
	t.Helper()

	tasklist := GetTaskList(c)

	if len(tasklist.Items) != len(td) {
		t.Errorf("Length of task list and Testdata provided not matching")
	}
	if len(tasklist.Items) == 0 {
		return tasklist
	}
	for i, task := range td {
		switch task := task.(type) {
		case *TaskData:
			tasklist.Items[i].Name = task.Name
		default:
			t.Error("Test Data Format Didn't Match please do check Test Data which you passing")
		}
	}

	if changelog := cmp.Diff(tasklist, GetTaskList(c)); changelog != "" {
		t.Logf("Changes occured while performing diff operation %+v", changelog)
	}
	return tasklist
}

type TaskRunData struct {
	Name   string
	Status string
}

func ListAllTaskRunsOutput(t *testing.T, cs *Clients, td map[int]interface{}) string {

	const (
		emptyMsg = "No taskruns found"
		header   = "NAME\tSTARTED\tDURATION\tSTATUS\t"
		body     = "%s\t%s\t%s\t%s\t\n"
	)

	clock := clockwork.NewFakeClockAt(time.Now())
	taskrun := GetTaskRunListWithTestData(t, cs, td)

	trslen := len(taskrun.Items)
	if trslen != 0 {
		taskrun.Items = trhsort.SortTaskRunsByStartTime(taskrun.Items)
	}

	var tmplBytes bytes.Buffer
	w := tabwriter.NewWriter(&tmplBytes, 0, 5, 3, ' ', tabwriter.TabIndent)

	if len(taskrun.Items) == 0 {
		fmt.Fprintln(w, emptyMsg)
		w.Flush()
		return tmplBytes.String()
	}

	fmt.Fprintln(w, header)
	for _, tr := range taskrun.Items {
		fmt.Fprintf(w, body,
			tr.Name,
			formatted.Age(tr.Status.StartTime, clock),
			formatted.Duration(tr.Status.StartTime, tr.Status.CompletionTime),
			formatted.Condition(tr.Status.Conditions),
		)
	}
	w.Flush()
	return tmplBytes.String()
}

func GetTaskRunListWithTestData(t *testing.T, c *Clients, td map[int]interface{}) *v1alpha1.TaskRunList {
	taskRunlist := GetTaskRunList(c)
	if len(taskRunlist.Items) != len(td) {
		t.Errorf("Length of taskrun list and Testdata provided not matching")
	}
	if len(taskRunlist.Items) == 0 {
		return taskRunlist
	}
	for i, tr := range td {
		switch tr := tr.(type) {
		case *TaskRunData:
			match, _ := regexp.Compile(tr.Name + ".*")
			if match.MatchString(taskRunlist.Items[i].Name) {
				taskRunlist.Items[i].Status.Conditions[0].Reason = tr.Status
			} else {
				t.Errorf("TaskRun Name didnt match , Expected %s Got %s", tr.Name, taskRunlist.Items[i].Name)
			}
		default:
			t.Errorf("Test Data Format Didn't Match please do check Test Data which you passing")
		}

	}

	if changelog := cmp.Diff(taskRunlist, GetTaskRunList(c)); changelog != "" {
		t.Logf("Changes occured while performing diff operation %+v", changelog)
	}
	return taskRunlist
}

const TaskRunDescribeTemplate = `Name:	{{ .TaskRun.Name }}
Namespace:	{{ .TaskRun.Namespace }}
{{- $tRefName := taskRefExists .TaskRun.Spec }}{{- if ne $tRefName "" }}
Task Ref:    {{ $tRefName }}
{{- end }}
{{- if ne .TaskRun.Spec.DeprecatedServiceAccount "" }}
Service Account (deprecated):	{{ .TaskRun.Spec.DeprecatedServiceAccount }}
{{- end }}
{{- if ne .TaskRun.Spec.ServiceAccountName "" }}
Service Account:	{{ .TaskRun.Spec.ServiceAccountName }}
{{- end }}

Status
STARTED 	DURATION 	STATUS
{{ formatAge .TaskRun.Status.StartTime  .Params }}	{{ formatDuration .TaskRun.Status.StartTime .TaskRun.Status.CompletionTime }}	{{ formatCondition .TaskRun.Status.Conditions }}
{{- $msg := hasFailed .TaskRun -}}
{{-  if ne $msg "" }}

Message
{{ $msg }}
{{- end }}

Input Resources
{{- $l := len .TaskRun.Spec.Inputs.Resources }}{{ if eq $l 0 }}
No resources
{{- else }}
NAME	RESOURCE REF
{{- range $i, $r := .TaskRun.Spec.Inputs.Resources }}
{{$r.Name }}	{{ $r.ResourceRef.Name }}
{{- end }}
{{- end }}

Output Resources
{{- $l := len .TaskRun.Spec.Outputs.Resources }}{{ if eq $l 0 }}
No resources
{{- else }}
NAME	RESOURCE REF
{{- range $i, $r := .TaskRun.Spec.Outputs.Resources }}
{{$r.Name }}	{{ $r.ResourceRef.Name }}
{{- end }}
{{- end }}

Params
{{- $l := len .TaskRun.Spec.Inputs.Params }}{{ if eq $l 0 }}
No params
{{- else }}
NAME	VALUE
{{- range $i, $p := .TaskRun.Spec.Inputs.Params }}
{{- if eq $p.Value.Type "string" }}
{{ $p.Name }}	{{ $p.Value.StringVal }}
{{- else }}
{{ $p.Name }}	{{ $p.Value.ArrayVal }}
{{- end }}
{{- end }}
{{- end }}

Steps
{{- $l := len .TaskRun.Status.Steps }}{{ if eq $l 0 }}
No steps
{{- else }}
NAME	STATUS
{{- range $step := .TaskRun.Status.Steps }}
{{- $reason := stepReasonExists $step }}
{{ $step.Name }}	{{ $reason }}
{{- end }}
{{- end }}
`

func GetTaskRunDescribeOutput(t *testing.T, c *Clients, trname string, td map[int]interface{}) string {
	t.Helper()
	clock := clockwork.NewFakeClockAt(time.Now())
	taskRun := GetTaskRunWithTestData(t, c, trname, td)
	var data = struct {
		TaskRun *v1alpha1.TaskRun
		Params  clockwork.Clock
	}{
		TaskRun: taskRun,
		Params:  clock,
	}

	funcMap := template.FuncMap{
		"formatAge":        formatted.Age,
		"formatDuration":   formatted.Duration,
		"formatCondition":  formatted.Condition,
		"hasFailed":        TaskRunHasFailed,
		"taskRefExists":    validate.TaskRefExists,
		"stepReasonExists": validate.StepReasonExists,
	}

	tmp := template.Must(template.New("Describe TaskRun").Funcs(funcMap).Parse(TaskRunDescribeTemplate))

	var tmplBytes bytes.Buffer

	w := tabwriter.NewWriter(&tmplBytes, 0, 5, 3, ' ', tabwriter.TabIndent)

	err1 := tmp.Execute(w, data)
	if err1 != nil {
		panic(err1)
	}

	w.Flush()
	return tmplBytes.String()

}

type TaskRunDescribeData struct {
	Name           string
	Namespace      string
	TaskRef        string
	ServiceAccount string
	Status         string
	FailureMessage string
	Input          map[string]string
	Output         map[string]string
	Params         map[string]interface{}
	Steps          []string
}

func GetTaskRunWithTestData(t *testing.T, c *Clients, trname string, td map[int]interface{}) *v1alpha1.TaskRun {
	t.Helper()
	taskRun := GetTaskRun(c, trname)
	for _, tr := range td {
		switch tr := tr.(type) {
		case *TaskRunDescribeData:
			taskRun.Name = tr.Name
			taskRun.Namespace = tr.Namespace
			taskRun.Spec.TaskRef.Name = tr.TaskRef
			taskRun.Spec.ServiceAccountName = tr.ServiceAccount
			taskRun.Status.Conditions[0].Reason = tr.Status
			if tr.FailureMessage != "" {
				taskRun.Status.Conditions[0].Message = tr.FailureMessage
			}
			if len(tr.Input) == len(taskRun.Spec.Inputs.Resources) {
				counter := 0
				for rname, rref := range tr.Input {
					taskRun.Spec.Inputs.Resources[counter].Name = rname
					taskRun.Spec.Inputs.Resources[counter].ResourceRef.Name = rref
					counter++
				}

			} else {
				t.Error("Input Resource length didnt match with test data")
			}
			if len(tr.Output) == len(taskRun.Spec.Outputs.Resources) {
				counter := 0
				for rname, rref := range tr.Output {
					taskRun.Spec.Outputs.Resources[counter].Name = rname
					taskRun.Spec.Outputs.Resources[counter].ResourceRef.Name = rref
					counter++
				}

			} else {
				t.Error("Input Resource length didnt match with test data")
			}
			counter := 0
			for ipname, ipvalue := range tr.Params {
				taskRun.Spec.Inputs.Params[counter].Name = ipname
				switch ipvalue.(type) {
				case *string:
					taskRun.Spec.Inputs.Params[counter].Value.StringVal = ipvalue.(string)
					counter++
				case *[]string:
					taskRun.Spec.Inputs.Params[counter].Value.ArrayVal = ipvalue.([]string)
					counter++
				default:
					t.Error("Input parameter test data type mismatch ")
				}
			}

			for i, stepname := range tr.Steps {
				taskRun.Status.Steps[i].Name = stepname
			}

		default:
			t.Error("Test Data Format Didn't Match please do check Test Data which you passing")
		}
	}

	return taskRun

}

//----------------Pipeline Resources -----------------------------

type PipelineResourcesData struct {
	Name    string
	Type    string
	Details string
}

func ListAllPipelineResourcesOutput(t *testing.T, cs *Clients, td map[int]interface{}) string {
	t.Helper()
	const (
		emptyMsg = "No pipelineresources found."
		header   = "NAME\tTYPE\tDETAILS"
		body     = "%s\t%s\t%s\n"
	)

	pipelineResourcelist := GetPipelineResourceListWithTestData(t, cs, td)

	var tmplBytes bytes.Buffer
	w := tabwriter.NewWriter(&tmplBytes, 0, 5, 3, ' ', tabwriter.TabIndent)

	if len(pipelineResourcelist.Items) == 0 {
		fmt.Fprintln(w, emptyMsg)
		w.Flush()
		return tmplBytes.String()
	}
	fmt.Fprintln(w, header)
	for _, pre := range pipelineResourcelist.Items {
		fmt.Fprintf(w, body,
			pre.Name,
			pre.Spec.Type,
			Details(pre),
		)
	}
	w.Flush()

	return tmplBytes.String()
}

func GetPipelineResourceListWithTestData(t *testing.T, c *Clients, td map[int]interface{}) *v1alpha1.PipelineResourceList {
	t.Helper()
	pipelineResourceList := GetPipelineResourceList(c)

	if len(pipelineResourceList.Items) != len(td) {
		t.Error("Length of PipelineResources list and Testdata provided not matching")
	}
	if len(pipelineResourceList.Items) == 0 {
		return pipelineResourceList
	}

	var key = "url"
	for i, pr := range td {
		switch pr := pr.(type) {
		case *PipelineResourcesData:

			//Mock Name
			pipelineResourceList.Items[i].Name = pr.Name

			//Mock Resource Type
			if pr.Type == "git" {
				pipelineResourceList.Items[i].Spec.Type = v1alpha1.PipelineResourceTypeGit
			} else if pr.Type == "storage" {
				pipelineResourceList.Items[i].Spec.Type = v1alpha1.PipelineResourceTypeStorage
				key = "location"
			} else if pr.Type == "image" {
				pipelineResourceList.Items[i].Spec.Type = v1alpha1.PipelineResourceTypeImage
			} else if pr.Type == "cluster" {
				pipelineResourceList.Items[i].Spec.Type = v1alpha1.PipelineResourceTypeCluster
			} else if pr.Type == "pullRequest" {
				pipelineResourceList.Items[i].Spec.Type = v1alpha1.PipelineResourceTypePullRequest
			} else if pr.Type == "build-gcs" {
				pipelineResourceList.Items[i].Spec.Type = v1alpha1.PipelineResourceTypeBuildGCS
			} else if pr.Type == "gcs" {
				pipelineResourceList.Items[i].Spec.Type = v1alpha1.PipelineResourceTypeGCS
			} else {
				t.Errorf("Provided PipelineResourcesData is not Valid Type : Need to Provide (%s, %s, %s, %s, %s)", v1alpha1.PipelineResourceTypeGit, v1alpha1.PipelineResourceTypeImage, v1alpha1.PipelineResourceTypePullRequest, v1alpha1.PipelineResourceTypeBuildGCS, v1alpha1.PipelineResourceTypeCluster)
			}

			for k, p := range pipelineResourceList.Items[i].Spec.Params {
				if strings.ToLower(p.Name) == key {
					pipelineResourceList.Items[i].Spec.Params[k].Value = pr.Details
					break
				} else {
					pipelineResourceList.Items[i].Spec.Params[k].Value = pr.Details
				}
			}

		default:
			t.Error("Test Data Format Didn't Match please do check Test Data which you passing")
		}

	}

	if changelog := cmp.Diff(pipelineResourceList, GetPipelineResourceList(c)); changelog != "" {
		t.Logf("Changes occured while performing diff operation %+v", changelog)
	}
	return pipelineResourceList
}

type PipelineResourcesDescribeData struct {
	Name                 string
	Namespace            string
	PipelineResourceType string
	Params               map[string]string
	SecretParams         map[string]string
}

const describeTemplateForPipelinesResources = `Name:	{{ .PipelineResource.Name }}
Namespace:	{{ .PipelineResource.Namespace }}
PipelineResource Type:	{{ .PipelineResource.Spec.Type }}

Params
{{- $l := len .PipelineResource.Spec.Params }}{{ if eq $l 0 }}
No params
{{- else }}
NAME	VALUE
{{- range $i, $p := .PipelineResource.Spec.Params }}
{{ $p.Name }}	{{ $p.Value }}
{{- end }}
{{- end }}

Secret Params
{{- $l := len .PipelineResource.Spec.SecretParams }}{{ if eq $l 0 }}
No secret params
{{- else }}
FIELDNAME	SECRETNAME
{{- range $i, $p := .PipelineResource.Spec.SecretParams }}
{{ $p.FieldName }}	{{ $p.SecretName }}
{{- end }}
{{- end }}
`

func GetPipelineResourceDescribeOutput(t *testing.T, cs *Clients, prname string, td map[int]interface{}) string {
	t.Helper()
	clock := clockwork.NewFakeClockAt(time.Now())
	pipelineResource := GetPipelineResourceWithTestData(t, cs, prname, td)
	var data = struct {
		PipelineResource *v1alpha1.PipelineResource
		Params           clockwork.Clock
	}{
		PipelineResource: pipelineResource,
		Params:           clock,
	}

	funcMap := template.FuncMap{}

	tmp := template.Must(template.New("Describe PipelineResource").Funcs(funcMap).Parse(describeTemplateForPipelinesResources))

	var tmplBytes bytes.Buffer

	w := tabwriter.NewWriter(&tmplBytes, 0, 5, 3, ' ', tabwriter.TabIndent)

	err1 := tmp.Execute(w, data)
	if err1 != nil {
		panic(err1)
	}

	w.Flush()
	return tmplBytes.String()
}

func GetPipelineResourceWithTestData(t *testing.T, c *Clients, name string, td map[int]interface{}) *v1alpha1.PipelineResource {
	t.Helper()
	pipelineResource := GetPipelineResource(c, name)

	for _, pr := range td {

		switch pr := pr.(type) {
		case *PipelineResourcesDescribeData:
			if pr.Name == name {
				pipelineResource.Name = pr.Name

				pipelineResource.Namespace = pr.Namespace

				if pr.PipelineResourceType == "git" {
					pipelineResource.Spec.Type = v1alpha1.PipelineResourceTypeGit
				} else if pr.PipelineResourceType == "storage" {
					pipelineResource.Spec.Type = v1alpha1.PipelineResourceTypeStorage
				} else if pr.PipelineResourceType == "image" {
					pipelineResource.Spec.Type = v1alpha1.PipelineResourceTypeImage
				} else if pr.PipelineResourceType == "cluster" {
					pipelineResource.Spec.Type = v1alpha1.PipelineResourceTypeCluster
				} else if pr.PipelineResourceType == "pullRequest" {
					pipelineResource.Spec.Type = v1alpha1.PipelineResourceTypePullRequest
				} else if pr.PipelineResourceType == "build-gcs" {
					pipelineResource.Spec.Type = v1alpha1.PipelineResourceTypeBuildGCS
				} else if pr.PipelineResourceType == "gcs" {
					pipelineResource.Spec.Type = v1alpha1.PipelineResourceTypeGCS
				} else {
					t.Errorf("Provided PipelineResourcesData is not Valid Type : Need to Provide (%s, %s, %s, %s, %s)", v1alpha1.PipelineResourceTypeGit, v1alpha1.PipelineResourceTypeImage, v1alpha1.PipelineResourceTypePullRequest, v1alpha1.PipelineResourceTypeBuildGCS, v1alpha1.PipelineResourceTypeCluster)
				}

				if len(pr.Params) == len(pipelineResource.Spec.Params) {
					for i := range pipelineResource.Spec.Params {
						pipelineResource.Spec.Params[i].Value = pr.Params[pipelineResource.Spec.Params[i].Name]
					}
				} else {
					t.Error("Pipeline Resources Params lenght didnt match...")
				}

				if len(pr.SecretParams) == len(pipelineResource.Spec.SecretParams) {

					for i := range pipelineResource.Spec.SecretParams {
						pipelineResource.Spec.SecretParams[i].SecretName = pr.SecretParams[pipelineResource.Spec.SecretParams[i].FieldName]
					}
				} else {
					t.Error("Pipeline Resources secret Params lenght didnt match...")
				}
			} else {
				continue
			}

		default:
			t.Errorf("Test Data Format Didn't Match please do check Test Data which you passing")
		}

	}

	if changelog := cmp.Diff(pipelineResource, GetPipelineResource(c, name)); changelog != "" {
		t.Logf("Changes occured while performing diff operation %+v", changelog)
	}

	return pipelineResource
}

type PipelinesListData struct {
	Name   string
	Status string
}

const pipelineslistTemplate = `{{- $pl := len .Pipelines.Items }}{{ if eq $pl 0 -}}
No pipelines
{{- else -}}
NAME	AGE	LAST RUN	STARTED	DURATION	STATUS
{{- range $_, $p := .Pipelines.Items }}
{{- $pr := accessMap $.PipelineRuns $p.Name }}
{{- if $pr }}
{{ $p.Name }}	{{ formatAge $p.CreationTimestamp $.Params }}	{{ $pr.Name }}	{{ formatAge $pr.Status.StartTime $.Params }}	{{ formatDuration $pr.Status.StartTime $pr.Status.CompletionTime }}	{{ index $pr.Status.Conditions | formatCondition }}
{{- else }}
{{ $p.Name }}	{{ formatAge $p.CreationTimestamp $.Params }}	---	---	---	---
{{- end }}
{{- end }}
{{- end }}
`

func ListAllPipelinesOutput(t *testing.T, cs *Clients, td map[int]interface{}) string {
	t.Helper()
	t.Log("validating Pipelines List command\n")
	clock := clockwork.NewFakeClockAt(time.Now())
	ps, prs, err := listPipelineDetailsWithTestData(t, cs, td)
	if err != nil {
		t.Error("Failed to list pipelines")
	}
	var data = struct {
		Pipelines    *v1alpha1.PipelineList
		PipelineRuns pipelineruns
		Params       clockwork.Clock
	}{
		Pipelines:    ps,
		PipelineRuns: prs,
		Params:       clock,
	}

	funcMap := template.FuncMap{
		"accessMap": func(prs pipelineruns, name string) *v1alpha1.PipelineRun {
			if pr, ok := prs[name]; ok {
				return &pr
			}

			return nil
		},
		"formatAge":       formatted.Age,
		"formatDuration":  formatted.Duration,
		"formatCondition": formatted.Condition,
	}

	tmp := template.Must(template.New("Pipelines List").Funcs(funcMap).Parse(pipelineslistTemplate))

	var tmplBytes bytes.Buffer

	w := tabwriter.NewWriter(&tmplBytes, 0, 5, 3, ' ', tabwriter.TabIndent)

	err1 := tmp.Execute(w, data)
	if err1 != nil {
		panic(err1)
	}

	w.Flush()
	return tmplBytes.String()
}

func listPipelineDetailsWithTestData(t *testing.T, cs *Clients, td map[int]interface{}) (*v1alpha1.PipelineList, pipelineruns, error) {
	t.Helper()
	ps := GetPipelineListWithTestData(t, cs, td)
	runs := GetPipelineRunList(cs)
	latestRuns := pipelineruns{}
	for _, p := range td {
		switch p := p.(type) {
		case *PipelinesListData:
			for _, run := range runs.Items {
				pipelineName := p.Name
				latest, ok := latestRuns[pipelineName]
				if !ok {
					run.Status.Conditions[0].Reason = p.Status
					latestRuns[pipelineName] = run
					continue
				}
				if run.CreationTimestamp.After(latest.CreationTimestamp.Time) {
					run.Status.Conditions[0].Reason = p.Status
					latestRuns[pipelineName] = run
				}
			}
		}
	}

	return ps, latestRuns, nil
}

func GetPipelineListWithTestData(t *testing.T, c *Clients, td map[int]interface{}) *v1alpha1.PipelineList {
	t.Helper()
	ps := GetPipelineList(c)

	if len(ps.Items) == 0 {
		return ps
	}

	if len(ps.Items) != len(td) {
		t.Error("Lenght of pipeline list and Testdata provided not matching")
	}

	for i, p := range td {
		switch p := p.(type) {
		case *PipelinesListData:
			ps.Items[i].Name = p.Name
		default:
			t.Error("Test Data Format Didn't Match please do check Test Data which you passing")
		}
	}

	if changelog := cmp.Diff(ps, GetPipelineList(c)); changelog != "" {
		t.Logf("Changes occured while performing diff operation %+v", changelog)
	}

	return ps
}

type PipelineDescribeData struct {
	Name      string
	Resources map[string]string
	Task      map[int]interface{}
	Runs      map[string]string
}

type TaskRefData struct {
	TaskName string
	TaskRef  string
	RunAfter []string
}

const describeTemplate = `Name:	{{ .PipelineName }}

Resources
{{- $rl := len .Pipeline.Spec.Resources }}{{ if eq $rl 0 }}
No resources
{{- else }}
NAME	TYPE
{{- range $i, $r := .Pipeline.Spec.Resources }}
{{$r.Name }}	{{ $r.Type }}
{{- end }}
{{- end }}

Params
{{- $l := len .Pipeline.Spec.Params }}{{ if eq $l 0 }}
No params
{{- else }}
NAME	TYPE	DEFAULT VALUE
{{- range $i, $p := .Pipeline.Spec.Params }}
{{- if not $p.Default }}
{{ $p.Name }}	{{ $p.Type }}	{{ "" }}
{{- else }}
{{- if eq $p.Type "string" }}
{{ $p.Name }}	{{ $p.Type }}	{{ $p.Default.StringVal }}
{{- else }}
{{ $p.Name }}	{{ $p.Type }}	{{ $p.Default.ArrayVal }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}

Tasks
{{- $tl := len .Pipeline.Spec.Tasks }}{{ if eq $tl 0 }}
No tasks
{{- else }}
NAME	TASKREF	RUNAFTER
{{- range $i, $t := .Pipeline.Spec.Tasks }}
{{ $t.Name }}	{{ $t.TaskRef.Name }}	{{ $t.RunAfter }}
{{- end }}
{{- end }}

Pipelineruns
{{- $rl := len .PipelineRuns.Items }}{{ if eq $rl 0 }}
No pipelineruns
{{- else }}
NAME	STARTED	DURATION	STATUS
{{- range $i, $pr := .PipelineRuns.Items }}
{{ $pr.Name }}	{{ formatAge $pr.Status.StartTime $.Params }}	{{ formatDuration $pr.Status.StartTime $pr.Status.CompletionTime }}	{{ formatCondition $pr.Status.Conditions }}
{{- end }}
{{- end }}
`

func GetPipelineDescribeOutput(t *testing.T, cs *Clients, pname string, td map[int]interface{}) string {

	t.Helper()
	clock := clockwork.NewFakeClockAt(time.Now())

	pipeline := GetPipelineWithTestData(t, cs, pname, td)
	if len(pipeline.Spec.Resources) > 0 {
		pipeline.Spec.Resources = SortResourcesByTypeAndName(pipeline.Spec.Resources)
	}
	pipelineRuns := GetPipelineRunListWithNameAndTestData(t, cs, pname, td)

	var data = struct {
		Pipeline     *v1alpha1.Pipeline
		PipelineRuns *v1alpha1.PipelineRunList
		PipelineName string
		Params       clockwork.Clock
	}{
		Pipeline:     pipeline,
		PipelineRuns: pipelineRuns,
		PipelineName: pname,
		Params:       clock,
	}

	funcMap := template.FuncMap{
		"formatAge":       formatted.Age,
		"formatDuration":  formatted.Duration,
		"formatCondition": formatted.Condition,
	}

	var tmplBytes bytes.Buffer

	w := tabwriter.NewWriter(&tmplBytes, 0, 5, 3, ' ', tabwriter.TabIndent)

	tmp := template.Must(template.New("Describe Pipeline").Funcs(funcMap).Parse(describeTemplate))

	err1 := tmp.Execute(w, data)
	if err1 != nil {
		panic(err1)
	}

	w.Flush()
	return tmplBytes.String()

}

func GetPipelineWithTestData(t *testing.T, c *Clients, name string, td map[int]interface{}) *v1alpha1.Pipeline {
	t.Helper()
	pipeline := GetPipeline(c, name)

	for _, p := range td {
		switch p := p.(type) {
		case *PipelineDescribeData:
			pipeline.Name = p.Name
			if len(pipeline.Spec.Resources) == len(p.Resources) {
				count := 0
				for k, v := range p.Resources {
					pipeline.Spec.Resources[count].Name = k
					if v == "git" {
						pipeline.Spec.Resources[count].Type = v1alpha1.PipelineResourceTypeGit
					} else if v == "storage" {
						pipeline.Spec.Resources[count].Type = v1alpha1.PipelineResourceTypeStorage

					} else if v == "image" {
						pipeline.Spec.Resources[count].Type = v1alpha1.PipelineResourceTypeImage
					} else if v == "cluster" {
						pipeline.Spec.Resources[count].Type = v1alpha1.PipelineResourceTypeCluster
					} else if v == "pullRequest" {
						pipeline.Spec.Resources[count].Type = v1alpha1.PipelineResourceTypePullRequest
					} else if v == "build-gcs" {
						pipeline.Spec.Resources[count].Type = v1alpha1.PipelineResourceTypeBuildGCS
					} else if v == "gcs" {
						pipeline.Spec.Resources[count].Type = v1alpha1.PipelineResourceTypeGCS
					} else {
						t.Errorf("Provided PipelineResourcesData is not Valid Type : Need to Provide (%s, %s, %s, %s, %s)", v1alpha1.PipelineResourceTypeGit, v1alpha1.PipelineResourceTypeImage, v1alpha1.PipelineResourceTypePullRequest, v1alpha1.PipelineResourceTypeBuildGCS, v1alpha1.PipelineResourceTypeCluster)
					}
					count++
				}
			} else {
				t.Errorf("length of Resources didn't match with testdata for pipeline %s", p.Name)
			}

			if len(pipeline.Spec.Tasks) == len(p.Task) {

				for i, tref := range p.Task {
					switch tref := tref.(type) {
					case *TaskRefData:
						pipeline.Spec.Tasks[i].Name = tref.TaskName
						pipeline.Spec.Tasks[i].TaskRef.Name = tref.TaskRef
						pipeline.Spec.Tasks[i].RunAfter = tref.RunAfter
					default:
						t.Errorf(" TaskRef Data type doesnt match with Expected Type Recheck your Test Data for pipeline %s", p.Name)
					}
				}
			} else {
				t.Errorf("length of Task didn't match with testdata for pipeline %s", p.Name)
			}
		default:
			t.Errorf(" Pipeline Describe Data type doesnt match with Expected Type Recheck your Test Data for pipeline %s", p.(*PipelineDescribeData).Name)
		}
	}

	if changelog := cmp.Diff(pipeline, GetPipeline(c, name)); changelog != "" {
		t.Logf("Changes occured while performing diff operation %+v", changelog)
	}

	return pipeline
}

func GetPipelineRunListWithNameAndTestData(t *testing.T, c *Clients, pname string, td map[int]interface{}) *v1alpha1.PipelineRunList {
	t.Helper()
	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("tekton.dev/pipeline=%s", pname),
	}
	pipelineRunList, err := c.PipelineRunClient.List(opts)
	if err != nil {
		t.Errorf("Couldn't get expected pipelineRunList  %s", err)
	}
	if len(pipelineRunList.Items) == 0 {
		return pipelineRunList
	}

	for _, p := range td {
		switch p := p.(type) {
		case *PipelineDescribeData:

			if len(pipelineRunList.Items) == len(p.Runs) {
				count := 0
				for k, v := range p.Runs {
					pipelineRunList.Items[count].Name = k
					pipelineRunList.Items[count].Status.Conditions[0].Reason = v
					count++
				}
			} else {
				t.Errorf("length of PipelineRuns didn't match with testdata for pipeline %s", p.Name)
			}

		default:
			t.Errorf(" Pipeline Describe Data type doesnt match with Expected Type Recheck your Test Data for pipeline %s", p.(*PipelineDescribeData).Name)
		}
	}

	if changelog := cmp.Diff(pipelineRunList, GetPipelineRunListWithName(c, pname)); changelog != "" {
		t.Logf("Changes occured while performing diff operation %+v", changelog)
	}

	return pipelineRunList
}

// ------------------------ PR -------------------------------

type PipelineRunListData struct {
	Name   string
	Status string
}

func CreateTemplateForPipelineRunListWithTestData(t *testing.T, cs *Clients, pipeline string, td map[int]interface{}) string {
	t.Helper()
	const (
		emptyMsg = "No pipelineruns found"
		header   = "NAME\tSTARTED\tDURATION\tSTATUS\t"
		body     = "%s\t%s\t%s\t%s\t\n"
	)

	log.Print("validating PipelineRun List command\n")
	clock := clockwork.NewFakeClockAt(time.Now())
	pipelinerunlst := GetSortedPipelineRunListWithTestData(t, cs, pipeline, td)

	var tmplBytes bytes.Buffer
	w := tabwriter.NewWriter(&tmplBytes, 0, 5, 3, ' ', tabwriter.TabIndent)

	if len(pipelinerunlst.Items) == 0 {
		fmt.Fprintln(w, emptyMsg)
		w.Flush()
		return tmplBytes.String()
	}
	fmt.Fprintln(w, header)
	for _, pr := range pipelinerunlst.Items {
		fmt.Fprintf(w, body,
			pr.Name,
			formatted.Age(pr.Status.StartTime, clock),
			formatted.Duration(pr.Status.StartTime, pr.Status.CompletionTime),
			formatted.Condition(pr.Status.Conditions),
		)
	}
	w.Flush()
	return tmplBytes.String()
}

func GetSortedPipelineRunListWithTestData(t *testing.T, c *Clients, pipeline string, td map[int]interface{}) *v1alpha1.PipelineRunList {
	t.Helper()
	options := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("tekton.dev/pipeline=%s", pipeline),
	}

	pipelineRunList, err := c.PipelineRunClient.List(options)
	// 	require.Nil(t, err)
	if err != nil {
		log.Fatalf("Couldn't get expected pipelineRunList  %s", err)
	}

	if len(pipelineRunList.Items) == 0 {
		return pipelineRunList
	}

	if len(pipelineRunList.Items) != len(td) {
		t.Error("Lenght of pipeline run list and Testdata provided not matching")
	}

	for i, p := range td {
		switch p := p.(type) {
		case *PipelineRunListData:
			pipelineRunList.Items[i].Name = p.Name
			pipelineRunList.Items[i].Status.Conditions[0].Reason = p.Status
		default:
			t.Error("Test Data Format Didn't Match please do check Test Data which you passing")
		}
	}

	prslen := len(pipelineRunList.Items)

	if prslen != 0 {
		pipelineRunList.Items = prhsort.SortPipelineRunsByStartTime(pipelineRunList.Items)
	}

	return pipelineRunList
}

const describeTemplateForPipelinesRun = `Name:	{{ .PipelineRun.Name }}
Namespace:	{{ .PipelineRun.Namespace }}
{{- if ne .PipelineRun.Spec.PipelineRef.Name "" }}
Pipeline Ref:	{{ .PipelineRun.Spec.PipelineRef.Name }}
{{- end }}
{{- if ne .PipelineRun.Spec.DeprecatedServiceAccount "" }}
Service Account (deprecated):	{{ .PipelineRun.Spec.DeprecatedServiceAccount }}
{{- end }}
{{- if ne .PipelineRun.Spec.ServiceAccountName "" }}
Service Account:	{{ .PipelineRun.Spec.ServiceAccountName }}
{{- end }}

Status
STARTED	DURATION	STATUS
{{ formatAge .PipelineRun.Status.StartTime  .Params }}	{{ formatDuration .PipelineRun.Status.StartTime .PipelineRun.Status.CompletionTime }}	{{ formatCondition .PipelineRun.Status.Conditions }}
{{- $msg := hasFailed .PipelineRun -}}
{{-  if ne $msg "" }}

Message
{{ $msg }}
{{- end }}

Resources
{{- $l := len .PipelineRun.Spec.Resources }}{{ if eq $l 0 }}
No resources
{{- else }}
NAME	RESOURCE REF
{{- range $i, $r := .PipelineRun.Spec.Resources }}
{{$r.Name }}	{{ $r.ResourceRef.Name }}
{{- end }}
{{- end }}

Params
{{- $l := len .PipelineRun.Spec.Params }}{{ if eq $l 0 }}
No params
{{- else }}
NAME	VALUE
{{- range $i, $p := .PipelineRun.Spec.Params }}
{{- if eq $p.Value.Type "string" }}
{{ $p.Name }}	{{ $p.Value.StringVal }}
{{- else }}
{{ $p.Name }}	{{ $p.Value.ArrayVal }}
{{- end }}
{{- end }}
{{- end }}

Taskruns
{{- $l := len .TaskrunList }}{{ if eq $l 0 }}
No taskruns
{{- else }}
NAME	TASK NAME	STARTED	DURATION	STATUS
{{- range $taskrun := .TaskrunList }}
{{ $taskrun.TaskrunName }}	{{ $taskrun.PipelineTaskName }}	{{ formatAge $taskrun.Status.StartTime $.Params }}	{{ formatDuration $taskrun.Status.StartTime $taskrun.Status.CompletionTime }}	{{ formatCondition $taskrun.Status.Conditions }}
{{- end }}
{{- end }}
`

func CreateDescribeTemplateForPipelineRunWithTestData(t *testing.T, cs *Clients, prname string, td map[int]interface{}) string {
	t.Helper()

	clock := clockwork.NewFakeClockAt(time.Now())
	pipelinerun := GetPipelineRunWithTestData(t, cs, prname, td)

	var trl taskrunList

	if len(pipelinerun.Status.TaskRuns) != 0 {
		trl = NewTaskrunListFromMapWithTestData(t, pipelinerun.Status.TaskRuns, td)
		sort.Sort(trl)
	}

	var data = struct {
		PipelineRun *v1alpha1.PipelineRun
		Params      clockwork.Clock
		TaskrunList taskrunList
	}{
		PipelineRun: pipelinerun,
		Params:      clock,
		TaskrunList: trl,
	}

	funcMap := template.FuncMap{
		"formatAge":       formatted.Age,
		"formatDuration":  formatted.Duration,
		"formatCondition": formatted.Condition,
		"hasFailed":       PipelineRunHasFailed,
	}

	tmp := template.Must(template.New("Describe PipelineRun").Funcs(funcMap).Parse(describeTemplateForPipelinesRun))

	var tmplBytes bytes.Buffer

	w := tabwriter.NewWriter(&tmplBytes, 0, 5, 3, ' ', tabwriter.TabIndent)

	err1 := tmp.Execute(w, data)
	if err1 != nil {
		panic(err1)
	}

	w.Flush()

	return tmplBytes.String()
}

type PipelineRunDescribeData struct {
	Name           string
	Namespace      string
	PipelineRef    string
	ServiceAccount string
	Status         string
	FailureMessage string
	Resources      map[int]interface{}
	Params         map[string]interface{}
	TaskRuns       map[int]interface{}
}

type ResourceRefData struct {
	ResourceName string
	ResourceRef  string
}

type TaskRunRefData struct {
	TaskRunName string
	TaskRef     string
	Status      string
}

func GetPipelineRunWithTestData(t *testing.T, c *Clients, name string, td map[int]interface{}) *v1alpha1.PipelineRun {
	t.Helper()
	pipelineRun := GetPipelineRun(c, name)

	for _, tr := range td {
		switch tr := tr.(type) {
		case *PipelineRunDescribeData:
			pipelineRun.Name = tr.Name
			pipelineRun.Namespace = tr.Namespace
			pipelineRun.Spec.PipelineRef.Name = tr.PipelineRef
			pipelineRun.Spec.ServiceAccountName = tr.ServiceAccount
			pipelineRun.Status.Conditions[0].Reason = tr.Status
			if tr.FailureMessage != "" {
				pipelineRun.Status.Conditions[0].Message = tr.FailureMessage
			}
			if len(tr.Resources) == len(pipelineRun.Spec.Resources) {
				for i, rref := range tr.Resources {

					switch rref := rref.(type) {
					case *ResourceRefData:
						pipelineRun.Spec.Resources[i].Name = rref.ResourceName
						pipelineRun.Spec.Resources[i].ResourceRef.Name = rref.ResourceRef
					default:
						t.Error("ResourceRef Test Data Format Didn't Match please do check Test Data which you passing")
					}
				}

			} else {
				t.Error("PipelineRun Resources length didnt match with test data")
			}

			counter := 0

			if len(tr.Params) == len(pipelineRun.Spec.Params) {
				for ipname, ipvalue := range tr.Params {
					pipelineRun.Spec.Params[counter].Name = ipname
					switch ipvalue.(type) {
					case *string:
						pipelineRun.Spec.Params[counter].Value.StringVal = ipvalue.(string)
						counter++
					case *[]string:
						pipelineRun.Spec.Params[counter].Value.ArrayVal = ipvalue.([]string)
						counter++
					default:
						t.Error("PipelineRun  parameter and test data type mismatch ")
					}
				}
			} else {
				t.Error("PipelineRun Params length didnt match with test data")
			}

		default:
			t.Error("Test Data Format Didn't Match please do check Test Data which you passing")
		}
	}

	return pipelineRun
}
