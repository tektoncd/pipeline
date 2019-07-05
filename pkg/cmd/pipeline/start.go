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
}

// NameArg validates that the first argument is a valid pipeline name
func NameArg(p cli.Params) cobra.PositionalArgs {

	return func(cmd *cobra.Command, args []string) error {
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

}

func startCommand(p cli.Params) *cobra.Command {
	var (
		res    []string
		params []string
		svc    string
	)

	c := &cobra.Command{
		Use:     "start pipeline [RESOURCES...] [PARAMS...] [SERVICEACCOUNT]",
		Aliases: []string{"trigger"},
		Short:   "Start pipelines",
		Example: `
# start pipeline foo by creating a pipelienrun named "foo-run-xyz123" from the namespace "bar"
tkn pipeline start foo --param NAME=VALUE --resource source=scaffold-git  -s ServiceAccountName  -n bar
`,
		SilenceUsage: true,
		Args:         NameArg(p),
		RunE: func(cmd *cobra.Command, args []string) error {

			opt := startOptions{
				Resources:          res,
				Params:             params,
				ServiceAccountName: svc,
			}
			return startPipeline(cmd.OutOrStdout(), opt, p, args[0])
		},
	}

	c.Flags().StringSliceVarP(&res, "resource", "r", []string{}, "pass the resource name and ref")
	c.Flags().StringSliceVarP(&params, "param", "p", []string{}, "pass the param")
	c.Flags().StringVarP(&svc, "serviceaccount", "s", svc, "pass the serviceaccount name")

	return c
}

func startPipeline(out io.Writer, opt startOptions, p cli.Params, pName string) error {

	cs, err := p.Clients()
	if err != nil {
		return err
	}

	res, err := parseRes(opt.Resources)
	if err != nil {
		return err
	}

	params, err := parseParam(opt.Params)
	if err != nil {
		return err
	}

	pr := &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    p.Namespace(),
			GenerateName: pName + "-run-",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef:    v1alpha1.PipelineRef{Name: pName},
			ServiceAccount: opt.ServiceAccountName,
			Resources:      res,
			Params:         params,
		},
	}

	pr, err = cs.Tekton.TektonV1alpha1().PipelineRuns(p.Namespace()).Create(pr)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Pipelinerun started: %s\n", pr.Name)
	return nil
}

func parseRes(res []string) ([]v1alpha1.PipelineResourceBinding, error) {
	var resources []v1alpha1.PipelineResourceBinding
	for _, v := range res {
		r := strings.Split(v, "=")
		if len(r) != 2 {
			errMsg := invalidResource + v +
				"\n Please pass resource as -p ResourceName=ResourceRef"
			return nil, errors.New(errMsg)
		}
		resources = append(resources, v1alpha1.PipelineResourceBinding{
			Name: r[0],
			ResourceRef: v1alpha1.PipelineResourceRef{
				Name: r[1],
			},
		})
	}
	return resources, nil
}

func parseParam(p []string) ([]v1alpha1.Param, error) {
	var params []v1alpha1.Param
	for _, v := range p {
		r := strings.Split(v, "=")
		if len(r) != 2 {
			errMsg := invalidParam + v +
				"\n Please pass resource as -r ParamName=ParamValue"
			return nil, errors.New(errMsg)
		}
		params = append(params, v1alpha1.Param{
			Name:  r[0],
			Value: r[1],
		})
	}
	return params, nil
}
