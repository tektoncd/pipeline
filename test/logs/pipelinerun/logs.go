/*
Copyright 2017 Google Inc. All Rights Reserved.
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

package pipelinerun

import (
	"context"
	"fmt"
	"io"

	TektonV1alpha1 "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1alpha1"
	trlogs "github.com/tektoncd/pipeline/test/logs/taskrun"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

/*
TailLogs tails the logs for pipeline run object with name `name` in namespace `namespace`
in `out` Writer.

ctx is used to carry cancellation of command. cfg is the kubernetes REST configuration
used to create pipeline client.
*/
func TailLogs(ctx context.Context, cfg *rest.Config, out io.Writer, name, namespace string) error {
	pclient, err := TektonV1alpha1.NewForConfig(cfg)
	if err != nil {
		return err
	}

	pipelineRun, err := pclient.PipelineRuns(namespace).Get(name, metav1.GetOptions{IncludeUninitialized: true})
	if err != nil {
		return err
	}

	pipelineName := pipelineRun.Spec.PipelineRef.Name
	if pipelineName == "" {
		return fmt.Errorf("Expected pipeline ref to be set")
	}

	pp, err := pclient.Pipelines(namespace).Get(pipelineName, metav1.GetOptions{IncludeUninitialized: true})
	if err != nil {
		return err
	}

	var expectedTaskRuns []string
	for _, pt := range pp.Spec.Tasks {
		expectedTaskRuns = append(expectedTaskRuns, fmt.Sprintf("%s-%s", pipelineRun.Name, pt.Name))
	}

	for _, expectedTaskRun := range expectedTaskRuns {
		err = trlogs.TailLogs(ctx, cfg, expectedTaskRun, namespace, out)
		if err != nil {
			return err
		}
	}

	return nil
}
