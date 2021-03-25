// +build e2e

/*
Copyright 2021 The Tekton Authors
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

package test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/selection"
	knativetest "knative.dev/pkg/test"
)

func TestPipelineBetaAlphaRoundtrip(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	pipelineName := "pipeline1"
	pipeline := &v1beta1.Pipeline{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1beta1",
			Kind:       "pipeline",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: pipelineName,
		},
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name: "test1",
				TaskRef: &v1beta1.TaskRef{
					Name: "test1-task",
				},
				WhenExpressions: v1beta1.WhenExpressions{{
					Input:    "foo",
					Operator: selection.NotIn,
					Values:   []string{"bar", "baz"},
				}},
			}},
			Finally: []v1beta1.PipelineTask{{
				Name: "test2",
				TaskRef: &v1beta1.TaskRef{
					Name: "test2-task",
				},
			}},
			Results: []v1beta1.PipelineResult{{
				Name:        "result1",
				Description: "A test result from this pipeline.",
				Value:       "$(tasks.test1.results.r)",
			}},
		},
	}

	if _, err := c.PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("failed to create Pipeline: %v", err)
	}

	pV1beta1, err := c.PipelineClient.Get(ctx, pipelineName, v1.GetOptions{})
	if err != nil {
		t.Fatalf("error getting initial v1beta1 pipeline: %v", err)
	}

	pV1alpha1, err := c.PipelineClientV1Alpha1.Get(ctx, pipelineName, v1.GetOptions{})
	if err != nil {
		t.Fatalf("error getting initial v1alpha1 pipeline: %v", err)
	}

	if _, ok := pV1alpha1.Annotations[v1alpha1.V1Beta1PipelineSpecSerializedAnnotationKey]; !ok {
		t.Fatal("the serialized v1beta1 object is not present in the v1alpha1 object's annotations")
	}

	// this value will be overwritten when the v1alpha1 resource is converted up to v1beta1
	// because the v1beta1 annotation will overwrite whatever changes we make to the v1alpha1
	// spec here.
	updatedResultValue := "this message should not be present in the pipeline results"
	pV1alpha1.Spec.Results[0].Value = updatedResultValue

	if _, err := c.PipelineClientV1Alpha1.Update(ctx, pV1alpha1, v1.UpdateOptions{}); err != nil {
		t.Fatalf("error applying modified v1alpha1 pipeline: %v", err)
	}

	pV1beta1After, err := c.PipelineClient.Get(ctx, pipelineName, v1.GetOptions{})
	if err != nil {
		t.Fatalf("error getting updated v1beta1 pipeline: %v", err)
	}

	// ignore the "meta" section of the pipelines when diffing them
	// because it contains fields like "generation" which are expected to
	// change every update.
	if d := cmp.Diff(pV1beta1, pV1beta1After, cmpopts.IgnoreFields(v1beta1.Pipeline{}, "ObjectMeta")); d != "" {
		t.Fatalf("unexpected difference detected after round trip: %s", diff.PrintWantGot(d))
	}
}
