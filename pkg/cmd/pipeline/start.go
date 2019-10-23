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
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/cmd/pipelinerun"
	"github.com/tektoncd/cli/pkg/flags"
	"github.com/tektoncd/cli/pkg/helper/labels"
	"github.com/tektoncd/cli/pkg/helper/params"
	"github.com/tektoncd/cli/pkg/helper/pipeline"
	validate "github.com/tektoncd/cli/pkg/helper/validate"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	errNoPipeline      = errors.New("missing pipeline name")
	errInvalidPipeline = "pipeline name %s does not exist in namespace %s"
)

const (
	invalidResource = "invalid input format for resource parameter : "
	invalidSvc      = "invalid service account parameter: "
)

type startOptions struct {
	cliparams          cli.Params
	stream             *cli.Stream
	askOpts            survey.AskOpt
	Params             []string
	Resources          []string
	ServiceAccountName string
	ServiceAccounts    []string
	Last               bool
	Labels             []string
	ShowLog            bool
}

type resourceOptionsFilter struct {
	git         []string
	image       []string
	cluster     []string
	storage     []string
	pullRequest []string
}

// NameArg validates that the first argument is a valid pipeline name
func NameArg(args []string, p cli.Params) error {
	if len(args) == 0 {
		return errNoPipeline
	}

	if err := validate.NamespaceExists(p); err != nil {
		return err
	}

	c, err := p.Clients()
	if err != nil {
		return err
	}

	name, ns := args[0], p.Namespace()
	_, err = c.Tekton.TektonV1alpha1().Pipelines(ns).Get(name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf(errInvalidPipeline, name, ns)
	}

	return nil
}

func startCommand(p cli.Params) *cobra.Command {
	opt := startOptions{
		cliparams: p,
		askOpts: func(opt *survey.AskOptions) error {
			opt.Stdio = terminal.Stdio{
				In:  os.Stdin,
				Out: os.Stdout,
				Err: os.Stderr,
			}
			return nil
		},
	}

	c := &cobra.Command{
		Use:     "start pipeline [RESOURCES...] [PARAMS...] [SERVICEACCOUNT]",
		Aliases: []string{"trigger"},
		Short:   "Start pipelines",
		Annotations: map[string]string{
			"commandType": "main",
		},
		Example: `
# start pipeline foo by creating a pipelinerun named "foo-run-xyz123" from the namespace "bar"
tkn pipeline start foo -s ServiceAccountName -n bar

For params value, if you want to provide multiple values, provide them comma separated
like cat,foo,bar
`,
		SilenceUsage: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if err := flags.InitParams(p, cmd); err != nil {
				return err
			}

			return NameArg(args, p)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opt.stream = &cli.Stream{
				Out: cmd.OutOrStdout(),
				Err: cmd.OutOrStderr(),
			}

			return opt.run(args[0])
		},
	}

	c.Flags().BoolVarP(&opt.ShowLog, "showlog", "", true, "show logs right after starting the pipeline")
	c.Flags().StringSliceVarP(&opt.Resources, "resource", "r", []string{}, "pass the resource name and ref as name=ref")
	c.Flags().StringArrayVarP(&opt.Params, "param", "p", []string{}, "pass the param as key=value or key=value1,value2")
	c.Flags().StringVarP(&opt.ServiceAccountName, "serviceaccount", "s", "", "pass the serviceaccount name")
	flags.AddShellCompletion(c.Flags().Lookup("serviceaccount"), "__kubectl_get_serviceaccount")
	c.Flags().StringSliceVar(&opt.ServiceAccounts, "task-serviceaccount", []string{}, "pass the service account corresponding to the task")
	flags.AddShellCompletion(c.Flags().Lookup("task-serviceaccount"), "__kubectl_get_serviceaccount")
	c.Flags().BoolVarP(&opt.Last, "last", "L", false, "re-run the pipeline using last pipelinerun values")
	c.Flags().StringSliceVarP(&opt.Labels, "labels", "l", []string{}, "pass labels as label=value.")

	_ = c.MarkZshCompPositionalArgumentCustom(1, "__tkn_get_pipeline")

	return c
}

func (opt *startOptions) run(pName string) error {
	if err := opt.getInput(pName); err != nil {
		return err
	}

	return opt.startPipeline(pName)
}

func (opt *startOptions) getInput(pname string) error {
	cs, err := opt.cliparams.Clients()
	if err != nil {
		return err
	}

	pipeline, err := getPipeline(cs.Tekton, opt.cliparams.Namespace(), pname)
	if err != nil {
		fmt.Fprintf(opt.stream.Err, "failed to get pipeline %s from %s namespace \n", pname, opt.cliparams.Namespace())
		return err
	}

	if len(opt.Resources) == 0 && !opt.Last {
		pres, err := getPipelineResources(cs.Tekton, opt.cliparams.Namespace())
		if err != nil {
			fmt.Fprintf(opt.stream.Err, "failed to list pipelineresources from %s namespace \n", opt.cliparams.Namespace())
			return err
		}

		resources := getPipelineResourcesByFormat(pres.Items)

		if err = opt.getInputResources(resources, pipeline); err != nil {
			return err
		}
	}

	if len(opt.Params) == 0 && !opt.Last {
		if err = opt.getInputParams(pipeline); err != nil {
			return err
		}
	}

	return nil
}

func (opt *startOptions) getInputResources(resources resourceOptionsFilter, pipeline *v1alpha1.Pipeline) error {
	var ans string
	for _, res := range pipeline.Spec.Resources {
		options := getOptionsByType(resources, string(res.Type))
		if len(options) == 0 {
			return fmt.Errorf("no pipeline resource of type %s found in namespace: %s",
				string(res.Type), opt.cliparams.Namespace())
		}

		var qs = []*survey.Question{
			{
				Name: "pipelineresource",
				Prompt: &survey.Select{
					Message: fmt.Sprintf("Choose the %s resource to use for %s:", res.Type, res.Name),
					Options: options,
				},
			},
		}

		if err := survey.Ask(qs, &ans, opt.askOpts); err != nil {
			fmt.Println(err.Error())
			return err
		}

		name := strings.TrimSpace(strings.Split(ans, " ")[0])
		opt.Resources = append(opt.Resources, res.Name+"="+name)
	}
	return nil
}

func (opt *startOptions) getInputParams(pipeline *v1alpha1.Pipeline) error {
	for _, param := range pipeline.Spec.Params {
		var ans string
		var qs = []*survey.Question{
			{
				Name: "pipeline param",
				Prompt: &survey.Input{
					Message: fmt.Sprintf("Value of param `%s` ? (Default is %s)", param.Name, param.Default.StringVal),
					Default: param.Default.StringVal,
				},
			},
		}

		if err := survey.Ask(qs, &ans, opt.askOpts); err != nil {
			fmt.Println(err.Error())
			return err
		}

		opt.Params = append(opt.Params, param.Name+"="+ans)
	}
	return nil
}

func getPipelineResources(client versioned.Interface, namespace string) (*v1alpha1.PipelineResourceList, error) {
	pres, err := client.TektonV1alpha1().PipelineResources(namespace).List(v1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return pres, nil
}

func getPipeline(client versioned.Interface, namespace string, pname string) (*v1alpha1.Pipeline, error) {
	pipeline, err := client.TektonV1alpha1().Pipelines(namespace).Get(pname, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return pipeline, nil
}

func getPipelineResourcesByFormat(resources []v1alpha1.PipelineResource) (ret resourceOptionsFilter) {
	for _, res := range resources {
		output := ""
		switch string(res.Spec.Type) {
		case "git":
			for _, param := range res.Spec.Params {
				if param.Name == "url" {
					output = param.Value + output
				}
				if param.Name == "revision" && param.Value != "master" {
					output = output + "#" + param.Value
				}
			}
			ret.git = append(ret.git, fmt.Sprintf("%s (%s)", res.Name, output))
		case "image":
			for _, param := range res.Spec.Params {
				if param.Name == "url" {
					output = param.Value + output
				}
			}
			ret.image = append(ret.image, fmt.Sprintf("%s (%s)", res.Name, output))
		case "pullRequest":
			for _, param := range res.Spec.Params {
				if param.Name == "url" {
					output = param.Value + output
				}
			}
			ret.pullRequest = append(ret.pullRequest, fmt.Sprintf("%s (%s)", res.Name, output))
		case "storage":
			for _, param := range res.Spec.Params {
				if param.Name == "location" {
					output = param.Value + output
				}
			}
			ret.storage = append(ret.storage, fmt.Sprintf("%s (%s)", res.Name, output))
		case "cluster":
			for _, param := range res.Spec.Params {
				if param.Name == "url" {
					output = param.Value + output
				}
				if param.Name == "user" {
					output = output + "#" + param.Value
				}
			}
			ret.cluster = append(ret.cluster, fmt.Sprintf("%s (%s)", res.Name, output))
		}
	}
	return
}

func getOptionsByType(resources resourceOptionsFilter, restype string) []string {
	if restype == "git" {
		return resources.git
	}
	if restype == "image" {
		return resources.image
	}
	if restype == "pullRequest" {
		return resources.pullRequest
	}
	if restype == "cluster" {
		return resources.cluster
	}
	if restype == "storage" {
		return resources.storage
	}
	return []string{}
}

func (opt *startOptions) startPipeline(pName string) error {
	pr := &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    opt.cliparams.Namespace(),
			GenerateName: pName + "-run-",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{Name: pName},
		},
	}

	cs, err := opt.cliparams.Clients()
	if err != nil {
		return err
	}

	if opt.Last {
		prLast, err := pipeline.LastRun(cs.Tekton, pName, opt.cliparams.Namespace())
		if err != nil {
			return err
		}
		pr.Spec.Resources = prLast.Spec.Resources
		pr.Spec.Params = prLast.Spec.Params
		// TODO(vdemeester) remove those 2 lines with bump to 0.9.0
		pr.Spec.DeprecatedServiceAccount = prLast.Spec.DeprecatedServiceAccount
		pr.Spec.DeprecatedServiceAccounts = prLast.Spec.DeprecatedServiceAccounts
		// If the prLast is a "new" PR, let's populate those fields too
		pr.Spec.ServiceAccountName = prLast.Spec.ServiceAccountName
		pr.Spec.ServiceAccountNames = prLast.Spec.ServiceAccountNames
	}

	if err := mergeRes(pr, opt.Resources); err != nil {
		return err
	}

	labels, err := labels.MergeLabels(pr.ObjectMeta.Labels, opt.Labels)
	if err != nil {
		return err
	}
	pr.ObjectMeta.Labels = labels

	param, err := params.MergeParam(pr.Spec.Params, opt.Params)
	if err != nil {
		return err
	}
	pr.Spec.Params = param

	if err := mergeSvc(pr, opt.ServiceAccounts); err != nil {
		return err
	}

	if len(opt.ServiceAccountName) > 0 {
		// FIXME(vdemeester) Populate the ServiceAccountName instead with bump to 0.9.0
		pr.Spec.DeprecatedServiceAccount = opt.ServiceAccountName
	}

	prCreated, err := cs.Tekton.TektonV1alpha1().PipelineRuns(opt.cliparams.Namespace()).Create(pr)
	if err != nil {
		return err
	}

	fmt.Fprintf(opt.stream.Out, "Pipelinerun started: %s\n", prCreated.Name)
	if !opt.ShowLog {
		fmt.Fprintf(opt.stream.Out, "\nIn order to track the pipelinerun progress run:\ntkn pipelinerun logs %s -f -n %s\n", prCreated.Name, prCreated.Namespace)
		return nil
	}

	fmt.Fprintf(opt.stream.Out, "Showing logs...\n")
	runLogOpts := &pipelinerun.LogOptions{
		PipelineName:    pName,
		PipelineRunName: prCreated.Name,
		Stream:          opt.stream,
		Follow:          true,
		Params:          opt.cliparams,
		AllSteps:        false,
	}
	return runLogOpts.Run()
}

func mergeRes(pr *v1alpha1.PipelineRun, optRes []string) error {
	res, err := parseRes(optRes)
	if err != nil {
		return err
	}

	if len(res) == 0 {
		return nil
	}

	for i := range pr.Spec.Resources {
		if v, ok := res[pr.Spec.Resources[i].Name]; ok {
			pr.Spec.Resources[i] = v
			delete(res, v.Name)
		}
	}
	for _, v := range res {
		pr.Spec.Resources = append(pr.Spec.Resources, v)
	}
	return nil
}

func mergeSvc(pr *v1alpha1.PipelineRun, optSvc []string) error {
	svcs, err := parseTaskSvc(optSvc)
	if err != nil {
		return err
	}
	dsvcs, err := parseDeprecatedTaskSvc(optSvc)
	if err != nil {
		return err
	}

	if len(svcs) == 0 && len(dsvcs) == 0 {
		return nil
	}

	// FIXME(vdemeester) Remove the next 2 loops with bump to 0.9.0
	for i := range pr.Spec.DeprecatedServiceAccounts {
		if v, ok := dsvcs[pr.Spec.DeprecatedServiceAccounts[i].TaskName]; ok {
			pr.Spec.DeprecatedServiceAccounts[i] = v
			delete(dsvcs, v.TaskName)
		}
	}

	for _, v := range dsvcs {
		pr.Spec.DeprecatedServiceAccounts = append(pr.Spec.DeprecatedServiceAccounts, v)
	}

	for i := range pr.Spec.ServiceAccountNames {
		if v, ok := svcs[pr.Spec.ServiceAccountNames[i].TaskName]; ok {
			pr.Spec.ServiceAccountNames[i] = v
			delete(svcs, v.TaskName)
		}
	}

	for _, v := range svcs {
		pr.Spec.ServiceAccountNames = append(pr.Spec.ServiceAccountNames, v)
	}

	return nil
}

func parseRes(res []string) (map[string]v1alpha1.PipelineResourceBinding, error) {
	resources := map[string]v1alpha1.PipelineResourceBinding{}
	for _, v := range res {
		r := strings.SplitN(v, "=", 2)
		if len(r) != 2 {
			return nil, errors.New(invalidResource + v)
		}
		resources[r[0]] = v1alpha1.PipelineResourceBinding{
			Name: r[0],
			ResourceRef: v1alpha1.PipelineResourceRef{
				Name: r[1],
			},
		}
	}
	return resources, nil
}

func parseTaskSvc(s []string) (map[string]v1alpha1.PipelineRunSpecServiceAccountName, error) {
	svcs := map[string]v1alpha1.PipelineRunSpecServiceAccountName{}
	for _, v := range s {
		r := strings.Split(v, "=")
		if len(r) != 2 || len(r[0]) == 0 {
			errMsg := invalidSvc + v +
				"\nPlease pass task service accounts as " +
				"--task-serviceaccount TaskName=ServiceAccount"
			return nil, errors.New(errMsg)
		}
		svcs[r[0]] = v1alpha1.PipelineRunSpecServiceAccountName{
			TaskName:           r[0],
			ServiceAccountName: r[1],
		}
	}
	return svcs, nil
}

func parseDeprecatedTaskSvc(s []string) (map[string]v1alpha1.DeprecatedPipelineRunSpecServiceAccount, error) {
	svcs := map[string]v1alpha1.DeprecatedPipelineRunSpecServiceAccount{}
	for _, v := range s {
		r := strings.Split(v, "=")
		if len(r) != 2 || len(r[0]) == 0 {
			errMsg := invalidSvc + v +
				"\nPlease pass task service accounts as " +
				"--task-serviceaccount TaskName=ServiceAccount"
			return nil, errors.New(errMsg)
		}
		svcs[r[0]] = v1alpha1.DeprecatedPipelineRunSpecServiceAccount{
			TaskName:                 r[0],
			DeprecatedServiceAccount: r[1],
		}
	}
	return svcs, nil
}
