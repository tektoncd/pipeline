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
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/flags"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	errNoPipeline      = errors.New("missing pipeline name")
	errInvalidPipeline = errors.New("invalid pipeline name")
)

const (
	invalidResource = "invalid resource parameter: "
	invalidParam    = "invalid param parameter: "
)

type startOptions struct {
	Params             []string
	Resources          []string
	ServiceAccountName string
	Last               bool
}

// NameArg validates that the first argument is a valid pipeline name
func NameArg(cmd *cobra.Command, args []string, p cli.Params) error {
	if len(args) == 0 {
		return errNoPipeline
	}

	c, err := p.Clients()
	if err != nil {
		return err
	}

	name, ns := args[0], p.Namespace()
	_, err = c.Tekton.TektonV1alpha1().Pipelines(ns).Get(name, metav1.GetOptions{})
	if err != nil {
		return errInvalidPipeline
	}

	return nil
}

func startCommand(p cli.Params) *cobra.Command {
	var (
		res    []string
		params []string
		svc    string
		last   bool
	)

	c := &cobra.Command{
		Use:     "start pipeline [RESOURCES...] [PARAMS...] [SERVICEACCOUNT]",
		Aliases: []string{"trigger"},
		Short:   "Start pipelines",
		Example: `
# start pipeline foo by creating a pipelinerun named "foo-run-xyz123" from the namespace "bar"
tkn pipeline start foo --param NAME=VALUE --resource source=scaffold-git  -s ServiceAccountName  -n bar
`,
		SilenceUsage: true,
		Args: func(cmd *cobra.Command, args []string) error {
			initResult := flags.InitParams(p, cmd)
			if initResult != nil {
				return initResult
			}
			return NameArg(cmd, args, p)
		},
		RunE: func(cmd *cobra.Command, args []string) error {

			opt := startOptions{
				Resources:          res,
				Params:             params,
				ServiceAccountName: svc,
				Last:               last,
			}
			return startPipeline(cmd.OutOrStdout(), opt, p, args[0])
		},
	}

	c.Flags().StringSliceVarP(&res, "resource", "r", []string{}, "pass the resource name and ref")
	c.Flags().StringSliceVarP(&params, "param", "p", []string{}, "pass the param")
	c.Flags().StringVarP(&svc, "serviceaccount", "s", svc, "pass the serviceaccount name")
	c.Flags().BoolVarP(&last, "last", "l", false, "re-run the pipeline using last pipelinerun values")
	return c
}

func startPipeline(out io.Writer, opt startOptions, p cli.Params, pName string) error {
	pr := &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    p.Namespace(),
			GenerateName: pName + "-run-",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{Name: pName},
		},
	}

	cs, err := p.Clients()
	if err != nil {
		return err
	}

	if opt.Last {
		prLast, err := lastPipelineRun(cs, pName, p.Namespace())
		if err != nil {
			return err
		}
		pr.Spec.Resources = prLast.Spec.Resources
		pr.Spec.Params = prLast.Spec.Params
		pr.Spec.ServiceAccount = prLast.Spec.ServiceAccount
	}

	if err := mergeRes(pr, opt.Resources); err != nil {
		return err
	}

	if err := mergeParam(pr, opt.Params); err != nil {
		return err
	}

	if len(opt.ServiceAccountName) > 0 {
		pr.Spec.ServiceAccount = opt.ServiceAccountName
	}

	pr, err = cs.Tekton.TektonV1alpha1().PipelineRuns(p.Namespace()).Create(pr)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Pipelinerun started: %s\n", pr.Name)
	return nil
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

func mergeParam(pr *v1alpha1.PipelineRun, optPar []string) error {
	params, err := parseParam(optPar)
	if err != nil {
		return err
	}

	if len(params) == 0 {
		return nil
	}

	for i := range pr.Spec.Params {
		if v, ok := params[pr.Spec.Params[i].Name]; ok {
			pr.Spec.Params[i] = v
			delete(params, v.Name)
		}
	}

	for _, v := range params {
		pr.Spec.Params = append(pr.Spec.Params, v)
	}

	return nil
}

func lastPipelineRun(cs *cli.Clients, pipeline, ns string) (*v1alpha1.PipelineRun, error) {
	options := metav1.ListOptions{}
	if pipeline != "" {
		options = metav1.ListOptions{
			LabelSelector: fmt.Sprintf("tekton.dev/pipeline=%s", pipeline),
		}
	}

	runs, err := cs.Tekton.TektonV1alpha1().PipelineRuns(ns).List(options)
	if err != nil {
		return nil, err
	}

	if len(runs.Items) == 0 {
		return nil, fmt.Errorf("No pipelineruns found in namespace: %s", ns)
	}

	latest := runs.Items[0]
	for _, run := range runs.Items {
		if run.CreationTimestamp.Time.After(latest.CreationTimestamp.Time) {
			latest = run
		}
	}

	return &latest, nil
}

func parseRes(res []string) (map[string]v1alpha1.PipelineResourceBinding, error) {
	resources := map[string]v1alpha1.PipelineResourceBinding{}
	for _, v := range res {
		r := strings.Split(v, "=")
		if len(r) != 2 || len(r[0]) == 0 {
			errMsg := invalidResource + v +
				"\n Please pass resource as -p ResourceName=ResourceRef"
			return nil, errors.New(errMsg)
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

func parseParam(p []string) (map[string]v1alpha1.Param, error) {
	params := map[string]v1alpha1.Param{}
	for _, v := range p {
		r := strings.Split(v, "=")
		if len(r) != 2 || len(r[0]) == 0 {
			errMsg := invalidParam + v +
				"\n Please pass resource as -r ParamName=ParamValue"
			return nil, errors.New(errMsg)
		}
		params[r[0]] = v1alpha1.Param{
			Name:  r[0],
			Value: r[1],
		}
	}
	return params, nil
}
